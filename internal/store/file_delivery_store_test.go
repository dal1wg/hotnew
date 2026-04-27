package store

import (
	"context"
	"testing"
	"time"

	"hotnew/internal/domain"
)

func TestFileDeliveryStorePersistsRecords(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileDeliveryStore(dir)
	if err != nil {
		t.Fatalf("new file delivery store: %v", err)
	}

	record := domain.DeliveryRecord{
		ID:          "1",
		ArticleID:   "article-1",
		Channel:     "blog",
		Target:      "https://example.com",
		Status:      "success",
		AttemptedAt: time.Now().UTC(),
	}
	if err := store.Append(context.Background(), record); err != nil {
		t.Fatalf("append record: %v", err)
	}
	if err := store.Close(context.Background()); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := NewFileDeliveryStore(dir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	items, err := reopened.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(items) != 1 || items[0].ArticleID != record.ArticleID {
		t.Fatalf("unexpected records: %+v", items)
	}
}
