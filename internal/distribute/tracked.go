package distribute

import (
	"context"
	"strings"
	"time"

	"hotnew/internal/domain"
	"hotnew/internal/platform/hash"
)

type TrackedDistributor struct {
	channel     string
	target      string
	next        domain.Distributor
	store       domain.DeliveryStore
	retryQueue  domain.RetryQueue
	maxAttempts int
	backoff     time.Duration
}

func NewTrackedDistributor(channel, target string, next domain.Distributor, store domain.DeliveryStore, retryQueue domain.RetryQueue, maxAttempts int, backoff time.Duration) TrackedDistributor {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if backoff <= 0 {
		backoff = 5 * time.Minute
	}
	return TrackedDistributor{
		channel:     strings.TrimSpace(channel),
		target:      strings.TrimSpace(target),
		next:        next,
		store:       store,
		retryQueue:  retryQueue,
		maxAttempts: maxAttempts,
		backoff:     backoff,
	}
}

func (d TrackedDistributor) Distribute(ctx context.Context, article domain.Article) error {
	attemptedAt := time.Now().UTC()
	err := d.next.Distribute(ctx, article)

	status := "success"
	message := ""
	if err != nil {
		status = "failed"
		message = err.Error()
	}

	if d.store != nil {
		_ = d.store.Append(ctx, domain.DeliveryRecord{
			ID:          hash.Fingerprint(article.ID, d.channel, d.target, attemptedAt.Format(time.RFC3339Nano)),
			ArticleID:   article.ID,
			Channel:     fallback(d.channel, "unknown"),
			Target:      d.target,
			Status:      status,
			Error:       message,
			AttemptedAt: attemptedAt,
		})
	}

	if err != nil && d.retryQueue != nil {
		_ = d.retryQueue.Enqueue(ctx, domain.RetryJob{
			ID:            hash.Fingerprint(article.ID, d.channel, d.target, attemptedAt.Format(time.RFC3339Nano), "retry"),
			ArticleID:     article.ID,
			Channel:       fallback(d.channel, "unknown"),
			Target:        d.target,
			Status:        "queued",
			Attempts:      0,
			MaxAttempts:   d.maxAttempts,
			LastError:     message,
			NextAttemptAt: attemptedAt.Add(d.backoff),
			CreatedAt:     attemptedAt,
			UpdatedAt:     attemptedAt,
		})
	}

	return err
}

func fallback(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
