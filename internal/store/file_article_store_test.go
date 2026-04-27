package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hotnew/internal/domain"
)

func TestFileArticleStorePersistsRecords(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArticleStore(dir)
	if err != nil {
		t.Fatalf("new file article store: %v", err)
	}

	article := domain.Article{
		ID:          "1",
		Hash:        "1",
		Title:       "hello",
		Source:      "test",
		URL:         "https://example.com",
		PublishedAt: time.Now().UTC(),
	}
	created, err := store.Upsert(context.Background(), article)
	if err != nil || !created {
		t.Fatalf("upsert = (%v, %v), want (true, nil)", created, err)
	}
	if err := store.Close(context.Background()); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := NewFileArticleStore(filepath.Clean(dir))
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	got, ok, err := reopened.Get(context.Background(), article.ID)
	if err != nil || !ok {
		t.Fatalf("get = (%v, %v), want article", ok, err)
	}
	if got.Title != article.Title {
		t.Fatalf("title = %q, want %q", got.Title, article.Title)
	}
}
