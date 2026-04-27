package store

import (
	"context"
	"fmt"
	"sync"

	"hotnew/internal/config"
	"hotnew/internal/domain"
)

type MemorySourceRegistry struct {
	mu    sync.RWMutex
	items map[string]domain.SourceMeta
}

func NewMemorySourceRegistry() *MemorySourceRegistry {
	return &MemorySourceRegistry{
		items: make(map[string]domain.SourceMeta),
	}
}

func (r *MemorySourceRegistry) RegisterDefaults(configs []config.SourceConfig) error {
	for _, cfg := range configs {
		if err := r.Register(context.Background(), domain.SourceMeta{
			Name:           cfg.Name,
			Kind:           cfg.Kind,
			BaseURL:        cfg.BaseURL,
			AccessMode:     cfg.AccessMode,
			LicenseNote:    cfg.LicenseNote,
			RateLimit:      cfg.RateLimit,
			TermsURL:       cfg.TermsURL,
			Enabled:        cfg.Enabled,
			DefaultTag:     cfg.DefaultTag,
			FetchBatchSize: cfg.FetchBatchSize,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *MemorySourceRegistry) Register(_ context.Context, meta domain.SourceMeta) error {
	if meta.Name == "" {
		return fmt.Errorf("source name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[meta.Name] = meta
	return nil
}

func (r *MemorySourceRegistry) List(_ context.Context) ([]domain.SourceMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.SourceMeta, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, nil
}
