package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"hotnew/internal/domain"
)

type FileRetryQueue struct {
	mu          sync.RWMutex
	file        *os.File
	archiveFile *os.File
	jobs        map[string]domain.RetryJob
}

func NewFileRetryQueue(dataDir string) (*FileRetryQueue, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	return NewFileRetryQueueAt(filepath.Join(dataDir, "retry_jobs.jsonl"), filepath.Join(dataDir, "retry_jobs.archive.jsonl"))
}

func NewFileRetryQueueAt(path, archivePath string) (*FileRetryQueue, error) {
	if path == "" {
		path = filepath.Join("data", "retry_jobs.jsonl")
	}
	if archivePath == "" {
		archivePath = filepath.Join(filepath.Dir(path), "retry_jobs.archive.jsonl")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create retry dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open retry file: %w", err)
	}
	archiveFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("open retry archive file: %w", err)
	}
	q := &FileRetryQueue{file: file, archiveFile: archiveFile, jobs: make(map[string]domain.RetryJob)}
	if err := q.load(); err != nil {
		_ = file.Close()
		_ = archiveFile.Close()
		return nil, err
	}
	return q, nil
}

func (q *FileRetryQueue) load() error {
	if _, err := q.file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek retry file: %w", err)
	}
	scanner := bufio.NewScanner(q.file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var job domain.RetryJob
		if err := json.Unmarshal(line, &job); err != nil {
			return fmt.Errorf("decode retry job: %w", err)
		}
		q.jobs[job.ID] = job
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan retry file: %w", err)
	}
	_, err := q.file.Seek(0, 2)
	return err
}

func (q *FileRetryQueue) Enqueue(_ context.Context, job domain.RetryJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, existing := range q.jobs {
		if existing.ArticleID == job.ArticleID && existing.Channel == job.Channel {
			if existing.Status == "queued" || existing.Status == "retrying" || existing.Status == "processing" {
				return nil
			}
		}
	}
	q.jobs[job.ID] = job
	return q.append(q.file, job)
}

func (q *FileRetryQueue) List(ctx context.Context, limit int) ([]domain.RetryJob, error) {
	return q.ListFiltered(ctx, domain.RetryFilter{Limit: limit})
}
func (q *FileRetryQueue) ListByStatus(ctx context.Context, status string, limit int) ([]domain.RetryJob, error) {
	return q.ListFiltered(ctx, domain.RetryFilter{Status: status, Limit: limit})
}

func (q *FileRetryQueue) ListFiltered(_ context.Context, filter domain.RetryFilter) ([]domain.RetryJob, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	items := q.filteredLocked(filter)
	if filter.Limit <= 0 || filter.Limit > len(items) {
		filter.Limit = len(items)
	}
	return append([]domain.RetryJob(nil), items[:filter.Limit]...), nil
}

func (q *FileRetryQueue) Get(_ context.Context, id string) (domain.RetryJob, bool, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	job, ok := q.jobs[id]
	return job, ok, nil
}

func (q *FileRetryQueue) Reset(_ context.Context, id string, nextAttemptAt time.Time) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	job, ok := q.jobs[id]
	if !ok {
		return false, nil
	}
	job.Status = "queued"
	job.Attempts = 0
	job.LastError = ""
	job.NextAttemptAt = nextAttemptAt.UTC()
	job.UpdatedAt = time.Now().UTC()
	q.jobs[id] = job
	return true, q.append(q.file, job)
}

func (q *FileRetryQueue) ArchiveSucceededBefore(_ context.Context, before time.Time, limit int) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.filteredLocked(domain.RetryFilter{Status: "succeeded", Limit: 0})
	archived := 0
	for _, job := range items {
		if limit > 0 && archived >= limit {
			break
		}
		if job.UpdatedAt.After(before) {
			continue
		}
		if err := q.append(q.archiveFile, job); err != nil {
			return archived, err
		}
		delete(q.jobs, job.ID)
		archived++
	}
	if archived > 0 {
		if err := q.rewriteActive(); err != nil {
			return archived, err
		}
	}
	return archived, nil
}

func (q *FileRetryQueue) ClaimReady(_ context.Context, now time.Time, limit int) ([]domain.RetryJob, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.filteredLocked(domain.RetryFilter{})
	out := make([]domain.RetryJob, 0, limit)
	for _, job := range items {
		if limit > 0 && len(out) >= limit {
			break
		}
		if (job.Status == "queued" || job.Status == "retrying") && !job.NextAttemptAt.After(now) {
			job.Status = "processing"
			job.UpdatedAt = now
			q.jobs[job.ID] = job
			if err := q.append(q.file, job); err != nil {
				return nil, err
			}
			out = append(out, job)
		}
	}
	return out, nil
}

func (q *FileRetryQueue) ClaimByID(_ context.Context, id string, now time.Time) (domain.RetryJob, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	job, ok := q.jobs[id]
	if !ok || job.Status == "processing" {
		return domain.RetryJob{}, false, nil
	}
	job.Status = "processing"
	job.UpdatedAt = now
	q.jobs[id] = job
	if err := q.append(q.file, job); err != nil {
		return domain.RetryJob{}, false, err
	}
	return job, true, nil
}

func (q *FileRetryQueue) MarkSucceeded(_ context.Context, job domain.RetryJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	current, ok := q.jobs[job.ID]
	if !ok {
		return nil
	}
	current.Status = "succeeded"
	current.Attempts = job.Attempts + 1
	current.LastError = ""
	current.UpdatedAt = time.Now().UTC()
	q.jobs[job.ID] = current
	return q.append(q.file, current)
}

func (q *FileRetryQueue) MarkFailed(_ context.Context, job domain.RetryJob, lastError string, nextAttemptAt time.Time, terminal bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	current, ok := q.jobs[job.ID]
	if !ok {
		return nil
	}
	if terminal {
		current.Status = "failed"
	} else {
		current.Status = "retrying"
	}
	current.Attempts = job.Attempts + 1
	current.LastError = lastError
	current.NextAttemptAt = nextAttemptAt.UTC()
	current.UpdatedAt = time.Now().UTC()
	q.jobs[job.ID] = current
	return q.append(q.file, current)
}

func (q *FileRetryQueue) Close(_ context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.file != nil {
		_ = q.file.Close()
		q.file = nil
	}
	if q.archiveFile != nil {
		_ = q.archiveFile.Close()
		q.archiveFile = nil
	}
	return nil
}

func (q *FileRetryQueue) append(file *os.File, job domain.RetryJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal retry job: %w", err)
	}
	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("append retry job: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync retry job: %w", err)
	}
	return nil
}

func (q *FileRetryQueue) filteredLocked(filter domain.RetryFilter) []domain.RetryJob {
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

func (q *FileRetryQueue) rewriteActive() error {
	if q.file == nil {
		return nil
	}
	if err := q.file.Truncate(0); err != nil {
		return fmt.Errorf("truncate retry file: %w", err)
	}
	if _, err := q.file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek retry file: %w", err)
	}
	items := q.filteredLocked(domain.RetryFilter{})
	for _, job := range items {
		payload, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("marshal retry job rewrite: %w", err)
		}
		if _, err := q.file.Write(append(payload, '\n')); err != nil {
			return fmt.Errorf("rewrite retry job: %w", err)
		}
	}
	if err := q.file.Sync(); err != nil {
		return fmt.Errorf("sync retry file rewrite: %w", err)
	}
	_, err := q.file.Seek(0, 2)
	return err
}
