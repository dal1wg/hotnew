package store

import (
	"context"
	"testing"
	"time"

	"hotnew/internal/domain"
)

func TestMemoryArticleStoreUpsertDeduplicates(t *testing.T) {
	s := NewMemoryArticleStore()
	article := domain.Article{
		ID:          "1",
		Title:       "hello",
		Source:      "test",
		URL:         "https://example.com",
		PublishedAt: time.Now().UTC(),
	}

	created, err := s.Upsert(context.Background(), article)
	if err != nil || !created {
		t.Fatalf("first upsert = (%v, %v), want (true, nil)", created, err)
	}

	created, err = s.Upsert(context.Background(), article)
	if err != nil {
		t.Fatalf("second upsert error: %v", err)
	}
	if created {
		t.Fatalf("expected deduplication")
	}
}
