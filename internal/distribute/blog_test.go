package distribute

import (
	"strings"
	"testing"
	"time"

	"hotnew/internal/config"
	"hotnew/internal/domain"
)

func TestBuildPost(t *testing.T) {
	distributor, err := NewBlogDistributor(config.BlogConfig{
		Endpoint: "https://example.com/webhook",
		SiteName: "my-blog",
		Author:   "editor",
		Mode:     "markdown",
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new blog distributor: %v", err)
	}

	post := distributor.BuildPost(domain.Article{
		ID:          "abc12345xyz",
		Hash:        "hash-1",
		Source:      "feed",
		Title:       "Go 1.23 Released",
		URL:         "https://example.com/source",
		Summary:     "summary text",
		Content:     "full content",
		Tags:        []string{"go", "release"},
		Language:    "en",
		PublishedAt: time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC),
	})

	if post.Slug == "" || !strings.Contains(post.Slug, "go-1-23-released") {
		t.Fatalf("unexpected slug: %q", post.Slug)
	}
	if post.SiteName != "my-blog" {
		t.Fatalf("site name = %q", post.SiteName)
	}
	if !strings.Contains(post.Content, "## 来源") {
		t.Fatalf("expected markdown content, got %q", post.Content)
	}
	if post.IdempotencyKey != "hash-1" {
		t.Fatalf("idempotency key = %q", post.IdempotencyKey)
	}
}
