package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hotnew/internal/domain"
	"hotnew/internal/platform/hash"
)

type RetryProcessor struct {
	articles   domain.ArticleStore
	deliveries domain.DeliveryStore
	queue      domain.RetryQueue
	channels   map[string]domain.Distributor
	backoff    time.Duration
	maxBackoff time.Duration

	mu sync.Mutex
}

type RetryRunResult struct {
	Claimed   int `json:"claimed"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

func NewRetryProcessor(articles domain.ArticleStore, deliveries domain.DeliveryStore, queue domain.RetryQueue, channels map[string]domain.Distributor, backoff, maxBackoff time.Duration) *RetryProcessor {
	if backoff <= 0 {
		backoff = 5 * time.Minute
	}
	if maxBackoff <= 0 {
		maxBackoff = 6 * time.Hour
	}
	if maxBackoff < backoff {
		maxBackoff = backoff
	}
	return &RetryProcessor{articles: articles, deliveries: deliveries, queue: queue, channels: channels, backoff: backoff, maxBackoff: maxBackoff}
}

func (p *RetryProcessor) ProcessOnce(ctx context.Context, limit int) (RetryRunResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	jobs, err := p.queue.ClaimReady(ctx, time.Now().UTC(), limit)
	if err != nil {
		return RetryRunResult{}, err
	}
	return p.processClaimed(ctx, jobs)
}

func (p *RetryProcessor) ProcessJob(ctx context.Context, id string) (RetryRunResult, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, ok, err := p.queue.ClaimByID(ctx, id, time.Now().UTC())
	if err != nil || !ok {
		return RetryRunResult{}, ok, err
	}
	result, err := p.processClaimed(ctx, []domain.RetryJob{job})
	return result, true, err
}

func (p *RetryProcessor) processClaimed(ctx context.Context, jobs []domain.RetryJob) (RetryRunResult, error) {
	result := RetryRunResult{Claimed: len(jobs)}
	for _, job := range jobs {
		article, ok, err := p.articles.Get(ctx, job.ArticleID)
		if err != nil {
			result.Failed++
			_ = p.failJob(ctx, job, fmt.Sprintf("load article: %v", err), true)
			continue
		}
		if !ok {
			result.Failed++
			_ = p.failJob(ctx, job, "article not found", true)
			continue
		}
		distributor, ok := p.channels[job.Channel]
		if !ok {
			result.Failed++
			_ = p.failJob(ctx, job, "retry distributor not found", true)
			continue
		}

		attemptedAt := time.Now().UTC()
		err = distributor.Distribute(ctx, article)
		if err != nil {
			result.Failed++
			_ = p.recordDelivery(ctx, article, job.Channel, job.Target, "failed", err.Error(), attemptedAt)
			terminal := job.Attempts+1 >= job.MaxAttempts
			_ = p.failJob(ctx, job, err.Error(), terminal)
			continue
		}

		result.Succeeded++
		_ = p.recordDelivery(ctx, article, job.Channel, job.Target, "success", "", attemptedAt)
		_ = p.queue.MarkSucceeded(ctx, job)
	}
	return result, nil
}

func (p *RetryProcessor) recordDelivery(ctx context.Context, article domain.Article, channel, target, status, message string, attemptedAt time.Time) error {
	if p.deliveries == nil {
		return nil
	}
	return p.deliveries.Append(ctx, domain.DeliveryRecord{ID: hash.Fingerprint(article.ID, channel, target, attemptedAt.Format(time.RFC3339Nano), status, "retry"), ArticleID: article.ID, Channel: channel, Target: target, Status: status, Error: message, AttemptedAt: attemptedAt})
}

func (p *RetryProcessor) failJob(ctx context.Context, job domain.RetryJob, message string, terminal bool) error {
	return p.queue.MarkFailed(ctx, job, message, time.Now().UTC().Add(p.nextBackoff(job.Attempts+1)), terminal)
}

func (p *RetryProcessor) nextBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return p.backoff
	}
	backoff := p.backoff
	for i := 1; i < attempt; i++ {
		if backoff >= p.maxBackoff/2 {
			return p.maxBackoff
		}
		backoff *= 2
	}
	if backoff > p.maxBackoff {
		return p.maxBackoff
	}
	return backoff
}
