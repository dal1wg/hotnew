package store

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"hotnew/internal/domain"
)

type MemoryRetryQueue struct {
	mu       sync.RWMutex
	jobs     []domain.RetryJob
	archived []domain.RetryJob
}

func NewMemoryRetryQueue() *MemoryRetryQueue { return &MemoryRetryQueue{} }

func (q *MemoryRetryQueue) Enqueue(_ context.Context, job domain.RetryJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, existing := range q.jobs {
		if existing.ArticleID == job.ArticleID && existing.Channel == job.Channel {
			if existing.Status == "queued" || existing.Status == "retrying" || existing.Status == "processing" {
				return nil
			}
		}
	}
	q.jobs = append(q.jobs, job)
	return nil
}

func (q *MemoryRetryQueue) List(ctx context.Context, limit int) ([]domain.RetryJob, error) {
	return q.ListFiltered(ctx, domain.RetryFilter{Limit: limit})
}
func (q *MemoryRetryQueue) ListByStatus(ctx context.Context, status string, limit int) ([]domain.RetryJob, error) {
	return q.ListFiltered(ctx, domain.RetryFilter{Status: status, Limit: limit})
}

func (q *MemoryRetryQueue) ListFiltered(_ context.Context, filter domain.RetryFilter) ([]domain.RetryJob, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	items := q.filteredLocked(filter)
	if filter.Limit <= 0 || filter.Limit > len(items) {
		filter.Limit = len(items)
	}
	return append([]domain.RetryJob(nil), items[:filter.Limit]...), nil
}

func (q *MemoryRetryQueue) Get(_ context.Context, id string) (domain.RetryJob, bool, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	for _, job := range q.jobs {
		if job.ID == id {
			return job, true, nil
		}
	}
	return domain.RetryJob{}, false, nil
}

func (q *MemoryRetryQueue) Reset(_ context.Context, id string, nextAttemptAt time.Time) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.jobs {
		if q.jobs[i].ID == id {
			q.jobs[i].Status = "queued"
			q.jobs[i].Attempts = 0
			q.jobs[i].LastError = ""
			q.jobs[i].NextAttemptAt = nextAttemptAt.UTC()
			q.jobs[i].UpdatedAt = time.Now().UTC()
			return true, nil
		}
	}
	return false, nil
}

func (q *MemoryRetryQueue) ArchiveSucceededBefore(_ context.Context, before time.Time, limit int) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	remaining := make([]domain.RetryJob, 0, len(q.jobs))
	archived := 0
	for _, job := range q.jobs {
		if job.Status == "succeeded" && !job.UpdatedAt.After(before) && (limit <= 0 || archived < limit) {
			q.archived = append(q.archived, job)
			archived++
			continue
		}
		remaining = append(remaining, job)
	}
	q.jobs = remaining
	return archived, nil
}

func (q *MemoryRetryQueue) ClaimReady(_ context.Context, now time.Time, limit int) ([]domain.RetryJob, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var items []domain.RetryJob
	for i := range q.jobs {
		if limit > 0 && len(items) >= limit {
			break
		}
		if (q.jobs[i].Status == "queued" || q.jobs[i].Status == "retrying") && !q.jobs[i].NextAttemptAt.After(now) {
			q.jobs[i].Status = "processing"
			q.jobs[i].UpdatedAt = now
			items = append(items, q.jobs[i])
		}
	}
	return items, nil
}

func (q *MemoryRetryQueue) ClaimByID(_ context.Context, id string, now time.Time) (domain.RetryJob, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.jobs {
		if q.jobs[i].ID == id {
			if q.jobs[i].Status == "processing" {
				return domain.RetryJob{}, false, nil
			}
			q.jobs[i].Status = "processing"
			q.jobs[i].UpdatedAt = now
			return q.jobs[i], true, nil
		}
	}
	return domain.RetryJob{}, false, nil
}

func (q *MemoryRetryQueue) MarkSucceeded(_ context.Context, job domain.RetryJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.jobs {
		if q.jobs[i].ID == job.ID {
			q.jobs[i].Status = "succeeded"
			q.jobs[i].Attempts = job.Attempts + 1
			q.jobs[i].LastError = ""
			q.jobs[i].UpdatedAt = time.Now().UTC()
			break
		}
	}
	return nil
}

func (q *MemoryRetryQueue) MarkFailed(_ context.Context, job domain.RetryJob, lastError string, nextAttemptAt time.Time, terminal bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.jobs {
		if q.jobs[i].ID == job.ID {
			q.jobs[i].Attempts = job.Attempts + 1
			q.jobs[i].LastError = lastError
			q.jobs[i].NextAttemptAt = nextAttemptAt.UTC()
			q.jobs[i].UpdatedAt = time.Now().UTC()
			if terminal {
				q.jobs[i].Status = "failed"
			} else {
				q.jobs[i].Status = "retrying"
			}
			break
		}
	}
	return nil
}

func (q *MemoryRetryQueue) filteredLocked(filter domain.RetryFilter) []domain.RetryJob {
	items := make([]domain.RetryJob, 0, len(q.jobs))
	for _, job := range q.jobs {
		if filter.Status != "" && !strings.EqualFold(job.Status, filter.Status) {
			continue
		}
		if filter.Channel != "" && !strings.EqualFold(job.Channel, filter.Channel) {
			continue
		}
		if filter.ArticleID != "" && job.ArticleID != filter.ArticleID {
			continue
		}
		items = append(items, job)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items
}
