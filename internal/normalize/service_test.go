package normalize

import (
	"testing"

	"hotnew/internal/domain"
)

func TestNormalize(t *testing.T) {
	svc := NewService()
	article, err := svc.Normalize(domain.RawItem{
		Source:      "test",
		Title:       "hello",
		URL:         "https://example.com/a",
		PublishedAt: "Mon, 02 Jan 2006 15:04:05 MST",
		Content:     " body ",
		Tags:        []string{"go", "Go", " rss "},
	})
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	if article.ID == "" {
		t.Fatalf("expected id")
	}
	if len(article.Tags) != 2 {
		t.Fatalf("expected deduplicated tags, got %v", article.Tags)
	}
}
