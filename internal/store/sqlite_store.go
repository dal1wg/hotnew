package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"hotnew/internal/domain"
)

type SQLiteDB struct{ db *sql.DB }
type SQLiteArticleStore struct{ db *SQLiteDB }
type SQLiteDeliveryStore struct{ db *SQLiteDB }
type SQLiteRetryQueue struct{ db *SQLiteDB }

func NewSQLiteDB(dsn string) (*SQLiteDB, error) {
	if dsn == "" {
		dsn = "data/hotnew.db"
	}
	if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	sqliteDB := &SQLiteDB{db: db}
	if err := sqliteDB.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return sqliteDB, nil
}

func (s *SQLiteDB) ArticleStore() *SQLiteArticleStore   { return &SQLiteArticleStore{db: s} }
func (s *SQLiteDB) DeliveryStore() *SQLiteDeliveryStore { return &SQLiteDeliveryStore{db: s} }
func (s *SQLiteDB) RetryQueue() *SQLiteRetryQueue       { return &SQLiteRetryQueue{db: s} }

func (s *SQLiteDB) init(ctx context.Context) error {
	statements := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
		`CREATE TABLE IF NOT EXISTS articles (id TEXT PRIMARY KEY, source TEXT NOT NULL, title TEXT NOT NULL, url TEXT NOT NULL, author TEXT, published_at TEXT, content TEXT, summary TEXT, tags_json TEXT, language TEXT, hash TEXT NOT NULL);`,
		`CREATE INDEX IF NOT EXISTS idx_articles_published_at ON articles(published_at DESC);`,
		`CREATE TABLE IF NOT EXISTS deliveries (id TEXT PRIMARY KEY, article_id TEXT NOT NULL, channel TEXT NOT NULL, target TEXT, status TEXT NOT NULL, error TEXT, attempted_at TEXT NOT NULL, FOREIGN KEY(article_id) REFERENCES articles(id));`,
		`CREATE INDEX IF NOT EXISTS idx_deliveries_attempted_at ON deliveries(attempted_at DESC);`,
		`CREATE TABLE IF NOT EXISTS retry_jobs (id TEXT PRIMARY KEY, article_id TEXT NOT NULL, channel TEXT NOT NULL, target TEXT, status TEXT NOT NULL, attempts INTEGER NOT NULL, max_attempts INTEGER NOT NULL, last_error TEXT, next_attempt_at TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, FOREIGN KEY(article_id) REFERENCES articles(id));`,
		`CREATE TABLE IF NOT EXISTS retry_jobs_archive (id TEXT PRIMARY KEY, article_id TEXT NOT NULL, channel TEXT NOT NULL, target TEXT, status TEXT NOT NULL, attempts INTEGER NOT NULL, max_attempts INTEGER NOT NULL, last_error TEXT, next_attempt_at TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, archived_at TEXT NOT NULL);`,
		`CREATE INDEX IF NOT EXISTS idx_retry_jobs_ready ON retry_jobs(status, next_attempt_at);`,
		`CREATE INDEX IF NOT EXISTS idx_retry_jobs_status ON retry_jobs(status, updated_at DESC);`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init sqlite schema: %w", err)
		}
	}
	return nil
}

func (s *SQLiteDB) Close(_ context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteArticleStore) Upsert(ctx context.Context, article domain.Article) (bool, error) {
	var exists int
	err := s.db.db.QueryRowContext(ctx, `SELECT 1 FROM articles WHERE id = ? LIMIT 1`, article.ID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("check article exists: %w", err)
	}
	created := err == sql.ErrNoRows
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return false, fmt.Errorf("marshal article tags: %w", err)
	}
	_, err = s.db.db.ExecContext(ctx, `INSERT INTO articles (id, source, title, url, author, published_at, content, summary, tags_json, language, hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET source=excluded.source, title=excluded.title, url=excluded.url, author=excluded.author, published_at=excluded.published_at, content=excluded.content, summary=excluded.summary, tags_json=excluded.tags_json, language=excluded.language, hash=excluded.hash`, article.ID, article.Source, article.Title, article.URL, article.Author, formatTime(article.PublishedAt), article.Content, article.Summary, string(tagsJSON), article.Language, article.Hash)
	if err != nil {
		return false, fmt.Errorf("upsert article: %w", err)
	}
	return created, nil
}

func (s *SQLiteArticleStore) List(ctx context.Context, limit int) ([]domain.Article, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.db.QueryContext(ctx, `SELECT id, source, title, url, author, published_at, content, summary, tags_json, language, hash FROM articles ORDER BY published_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()
	return scanArticles(rows)
}

func (s *SQLiteArticleStore) Get(ctx context.Context, id string) (domain.Article, bool, error) {
	row := s.db.db.QueryRowContext(ctx, `SELECT id, source, title, url, author, published_at, content, summary, tags_json, language, hash FROM articles WHERE id = ?`, id)
	article, err := scanArticle(row)
	if err == sql.ErrNoRows {
		return domain.Article{}, false, nil
	}
	if err != nil {
		return domain.Article{}, false, fmt.Errorf("get article: %w", err)
	}
	return article, true, nil
}

func (s *SQLiteDeliveryStore) Append(ctx context.Context, record domain.DeliveryRecord) error {
	_, err := s.db.db.ExecContext(ctx, `INSERT INTO deliveries (id, article_id, channel, target, status, error, attempted_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, record.ID, record.ArticleID, record.Channel, record.Target, record.Status, record.Error, formatTime(record.AttemptedAt))
	if err != nil {
		return fmt.Errorf("append delivery: %w", err)
	}
	return nil
}

func (s *SQLiteDeliveryStore) List(ctx context.Context, limit int) ([]domain.DeliveryRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.db.QueryContext(ctx, `SELECT id, article_id, channel, target, status, error, attempted_at FROM deliveries ORDER BY attempted_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()
	var items []domain.DeliveryRecord
	for rows.Next() {
		var record domain.DeliveryRecord
		var attemptedAt string
		if err := rows.Scan(&record.ID, &record.ArticleID, &record.Channel, &record.Target, &record.Status, &record.Error, &attemptedAt); err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		record.AttemptedAt = parseTime(attemptedAt)
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deliveries: %w", err)
	}
	return items, nil
}

func (s *SQLiteRetryQueue) Enqueue(ctx context.Context, job domain.RetryJob) error {
	now := time.Now().UTC()
	if job.ID == "" {
		return fmt.Errorf("retry job id is required")
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = 3
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = now
	}
	if job.NextAttemptAt.IsZero() {
		job.NextAttemptAt = now
	}
	var activeID string
	err := s.db.db.QueryRowContext(ctx, `SELECT id FROM retry_jobs WHERE article_id = ? AND channel = ? AND status IN ('queued','retrying','processing') ORDER BY created_at DESC LIMIT 1`, job.ArticleID, job.Channel).Scan(&activeID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("check active retry job: %w", err)
	}
	if err == nil && activeID != "" {
		return nil
	}
	_, err = s.db.db.ExecContext(ctx, `INSERT INTO retry_jobs (id, article_id, channel, target, status, attempts, max_attempts, last_error, next_attempt_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, job.ID, job.ArticleID, job.Channel, job.Target, job.Status, job.Attempts, job.MaxAttempts, job.LastError, formatTime(job.NextAttemptAt), formatTime(job.CreatedAt), formatTime(job.UpdatedAt))
	if err != nil {
		return fmt.Errorf("enqueue retry job: %w", err)
	}
	return nil
}

func (s *SQLiteRetryQueue) List(ctx context.Context, limit int) ([]domain.RetryJob, error) {
	return s.ListFiltered(ctx, domain.RetryFilter{Limit: limit})
}
func (s *SQLiteRetryQueue) ListByStatus(ctx context.Context, status string, limit int) ([]domain.RetryJob, error) {
	return s.ListFiltered(ctx, domain.RetryFilter{Status: status, Limit: limit})
}

func (s *SQLiteRetryQueue) ListFiltered(ctx context.Context, filter domain.RetryFilter) ([]domain.RetryJob, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	query := `SELECT id, article_id, channel, target, status, attempts, max_attempts, last_error, next_attempt_at, created_at, updated_at FROM retry_jobs WHERE 1=1`
	args := []any{}
	if filter.Status != "" {
		query += ` AND lower(status) = lower(?)`
		args = append(args, filter.Status)
	}
	if filter.Channel != "" {
		query += ` AND lower(channel) = lower(?)`
		args = append(args, filter.Channel)
	}
	if filter.ArticleID != "" {
		query += ` AND article_id = ?`
		args = append(args, filter.ArticleID)
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	args = append(args, filter.Limit)
	rows, err := s.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list retry jobs: %w", err)
	}
	defer rows.Close()
	return scanRetryJobs(rows)
}

func (s *SQLiteRetryQueue) Get(ctx context.Context, id string) (domain.RetryJob, bool, error) {
	row := s.db.db.QueryRowContext(ctx, `SELECT id, article_id, channel, target, status, attempts, max_attempts, last_error, next_attempt_at, created_at, updated_at FROM retry_jobs WHERE id = ?`, id)
	var job domain.RetryJob
	var nextAttemptAt, createdAt, updatedAt string
	err := row.Scan(&job.ID, &job.ArticleID, &job.Channel, &job.Target, &job.Status, &job.Attempts, &job.MaxAttempts, &job.LastError, &nextAttemptAt, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return domain.RetryJob{}, false, nil
	}
	if err != nil {
		return domain.RetryJob{}, false, fmt.Errorf("get retry job: %w", err)
	}
	job.NextAttemptAt = parseTime(nextAttemptAt)
	job.CreatedAt = parseTime(createdAt)
	job.UpdatedAt = parseTime(updatedAt)
	return job, true, nil
}

func (s *SQLiteRetryQueue) Reset(ctx context.Context, id string, nextAttemptAt time.Time) (bool, error) {
	result, err := s.db.db.ExecContext(ctx, `UPDATE retry_jobs SET status = 'queued', attempts = 0, last_error = '', next_attempt_at = ?, updated_at = ? WHERE id = ?`, formatTime(nextAttemptAt.UTC()), formatTime(time.Now().UTC()), id)
	if err != nil {
		return false, fmt.Errorf("reset retry job: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("retry job reset rows: %w", err)
	}
	return n > 0, nil
}

func (s *SQLiteRetryQueue) ArchiveSucceededBefore(ctx context.Context, before time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	tx, err := s.db.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin archive retry jobs tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id, article_id, channel, target, status, attempts, max_attempts, last_error, next_attempt_at, created_at, updated_at FROM retry_jobs WHERE status = 'succeeded' AND updated_at <= ? ORDER BY updated_at ASC LIMIT ?`, formatTime(before), limit)
	if err != nil {
		return 0, fmt.Errorf("query succeeded retry jobs: %w", err)
	}
	jobs, err := scanRetryJobs(rows)
	rows.Close()
	if err != nil {
		return 0, err
	}
	archivedAt := formatTime(time.Now().UTC())
	archived := 0
	for _, job := range jobs {
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO retry_jobs_archive (id, article_id, channel, target, status, attempts, max_attempts, last_error, next_attempt_at, created_at, updated_at, archived_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, job.ID, job.ArticleID, job.Channel, job.Target, job.Status, job.Attempts, job.MaxAttempts, job.LastError, formatTime(job.NextAttemptAt), formatTime(job.CreatedAt), formatTime(job.UpdatedAt), archivedAt)
		if err != nil {
			return archived, fmt.Errorf("insert retry archive job: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM retry_jobs WHERE id = ?`, job.ID); err != nil {
			return archived, fmt.Errorf("delete archived retry job: %w", err)
		}
		archived++
	}
	if err := tx.Commit(); err != nil {
		return archived, fmt.Errorf("commit archive retry jobs: %w", err)
	}
	return archived, nil
}

func (s *SQLiteRetryQueue) ClaimReady(ctx context.Context, now time.Time, limit int) ([]domain.RetryJob, error) {
	if limit <= 0 {
		limit = 10
	}
	tx, err := s.db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim retry jobs tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id, article_id, channel, target, status, attempts, max_attempts, last_error, next_attempt_at, created_at, updated_at FROM retry_jobs WHERE status IN ('queued','retrying') AND next_attempt_at <= ? ORDER BY next_attempt_at ASC, id ASC LIMIT ?`, formatTime(now), limit)
	if err != nil {
		return nil, fmt.Errorf("query ready retry jobs: %w", err)
	}
	jobs, err := scanRetryJobs(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		if _, err := tx.ExecContext(ctx, `UPDATE retry_jobs SET status = 'processing', updated_at = ? WHERE id = ?`, formatTime(now), jobs[i].ID); err != nil {
			return nil, fmt.Errorf("mark retry job processing: %w", err)
		}
		jobs[i].Status = "processing"
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit retry job claim: %w", err)
	}
	return jobs, nil
}

func (s *SQLiteRetryQueue) ClaimByID(ctx context.Context, id string, now time.Time) (domain.RetryJob, bool, error) {
	tx, err := s.db.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.RetryJob{}, false, fmt.Errorf("begin claim retry job tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRowContext(ctx, `SELECT id, article_id, channel, target, status, attempts, max_attempts, last_error, next_attempt_at, created_at, updated_at FROM retry_jobs WHERE id = ?`, id)
	var job domain.RetryJob
	var nextAttemptAt, createdAt, updatedAt string
	err = row.Scan(&job.ID, &job.ArticleID, &job.Channel, &job.Target, &job.Status, &job.Attempts, &job.MaxAttempts, &job.LastError, &nextAttemptAt, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return domain.RetryJob{}, false, nil
	}
	if err != nil {
		return domain.RetryJob{}, false, fmt.Errorf("get retry job for claim: %w", err)
	}
	if strings.EqualFold(job.Status, "processing") {
		return domain.RetryJob{}, false, nil
	}
	job.NextAttemptAt = parseTime(nextAttemptAt)
	job.CreatedAt = parseTime(createdAt)
	job.UpdatedAt = parseTime(updatedAt)
	if _, err := tx.ExecContext(ctx, `UPDATE retry_jobs SET status = 'processing', updated_at = ? WHERE id = ?`, formatTime(now), id); err != nil {
		return domain.RetryJob{}, false, fmt.Errorf("mark retry job processing: %w", err)
	}
	job.Status = "processing"
	job.UpdatedAt = now
	if err := tx.Commit(); err != nil {
		return domain.RetryJob{}, false, fmt.Errorf("commit retry job claim: %w", err)
	}
	return job, true, nil
}

func (s *SQLiteRetryQueue) MarkSucceeded(ctx context.Context, job domain.RetryJob) error {
	_, err := s.db.db.ExecContext(ctx, `UPDATE retry_jobs SET status = 'succeeded', attempts = ?, updated_at = ?, last_error = '' WHERE id = ?`, job.Attempts+1, formatTime(time.Now().UTC()), job.ID)
	if err != nil {
		return fmt.Errorf("mark retry job succeeded: %w", err)
	}
	return nil
}

func (s *SQLiteRetryQueue) MarkFailed(ctx context.Context, job domain.RetryJob, lastError string, nextAttemptAt time.Time, terminal bool) error {
	status := "retrying"
	if terminal {
		status = "failed"
	}
	_, err := s.db.db.ExecContext(ctx, `UPDATE retry_jobs SET status = ?, attempts = ?, last_error = ?, next_attempt_at = ?, updated_at = ? WHERE id = ?`, status, job.Attempts+1, lastError, formatTime(nextAttemptAt), formatTime(time.Now().UTC()), job.ID)
	if err != nil {
		return fmt.Errorf("mark retry job failed: %w", err)
	}
	return nil
}

type articleScanner interface{ Scan(dest ...any) error }

func scanArticle(scanner articleScanner) (domain.Article, error) {
	var article domain.Article
	var publishedAt, tagsJSON string
	if err := scanner.Scan(&article.ID, &article.Source, &article.Title, &article.URL, &article.Author, &publishedAt, &article.Content, &article.Summary, &tagsJSON, &article.Language, &article.Hash); err != nil {
		return domain.Article{}, err
	}
	article.PublishedAt = parseTime(publishedAt)
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &article.Tags)
	}
	return article, nil
}
func scanArticles(rows *sql.Rows) ([]domain.Article, error) {
	var items []domain.Article
	for rows.Next() {
		article, err := scanArticle(rows)
		if err != nil {
			return nil, fmt.Errorf("scan article: %w", err)
		}
		items = append(items, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate articles: %w", err)
	}
	return items, nil
}
func scanRetryJobs(rows *sql.Rows) ([]domain.RetryJob, error) {
	var items []domain.RetryJob
	for rows.Next() {
		var job domain.RetryJob
		var nextAttemptAt, createdAt, updatedAt string
		if err := rows.Scan(&job.ID, &job.ArticleID, &job.Channel, &job.Target, &job.Status, &job.Attempts, &job.MaxAttempts, &job.LastError, &nextAttemptAt, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan retry job: %w", err)
		}
		job.NextAttemptAt = parseTime(nextAttemptAt)
		job.CreatedAt = parseTime(createdAt)
		job.UpdatedAt = parseTime(updatedAt)
		items = append(items, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retry jobs: %w", err)
	}
	return items, nil
}
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func parseTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
