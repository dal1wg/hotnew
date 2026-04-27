package domain

import (
	"context"
	"time"
)

type Normalizer interface {
	Normalize(raw RawItem) (Article, error)
}

type Summarizer interface {
	Summarize(ctx context.Context, article Article) (string, error)
}

type ArticleStore interface {
	Upsert(ctx context.Context, article Article) (bool, error)
	List(ctx context.Context, limit int) ([]Article, error)
	Get(ctx context.Context, id string) (Article, bool, error)
}

type DeliveryStore interface {
	Append(ctx context.Context, record DeliveryRecord) error
	List(ctx context.Context, limit int) ([]DeliveryRecord, error)
}

type RetryQueue interface {
	Enqueue(ctx context.Context, job RetryJob) error
	List(ctx context.Context, limit int) ([]RetryJob, error)
	ListByStatus(ctx context.Context, status string, limit int) ([]RetryJob, error)
	ListFiltered(ctx context.Context, filter RetryFilter) ([]RetryJob, error)
	Get(ctx context.Context, id string) (RetryJob, bool, error)
	Reset(ctx context.Context, id string, nextAttemptAt time.Time) (bool, error)
	ArchiveSucceededBefore(ctx context.Context, before time.Time, limit int) (int, error)
	ClaimReady(ctx context.Context, now time.Time, limit int) ([]RetryJob, error)
	ClaimByID(ctx context.Context, id string, now time.Time) (RetryJob, bool, error)
	MarkSucceeded(ctx context.Context, job RetryJob) error
	MarkFailed(ctx context.Context, job RetryJob, lastError string, nextAttemptAt time.Time, terminal bool) error
}

type Distributor interface {
	Distribute(ctx context.Context, article Article) error
}

type Lifecycle interface {
	Close(ctx context.Context) error
}

type SourceRegistry interface {
	List(ctx context.Context) ([]SourceMeta, error)
	Register(ctx context.Context, meta SourceMeta) error
}

type SourceMeta struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	BaseURL        string `json:"base_url"`
	AccessMode     string `json:"access_mode"`
	LicenseNote    string `json:"license_note"`
	RateLimit      int    `json:"rate_limit"`
	TermsURL       string `json:"terms_url"`
	Enabled        bool   `json:"enabled"`
	DefaultTag     string `json:"default_tag,omitempty"`
	FetchBatchSize int    `json:"fetch_batch_size,omitempty"`
}
