package distribute

import (
	"strings"
	"testing"
	"time"

	"hotnew/internal/config"
	"hotnew/internal/domain"
)

func TestWeComBuildMessage(t *testing.T) {
	distributor, err := NewWeComDistributor(config.WeComConfig{
		Webhook: "https://example.com/wecom",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new wecom distributor: %v", err)
	}

	message := distributor.BuildMessage(domain.Article{
		Title:       "Gemini 2.5 update",
		Source:      "google-deepmind-blog",
		URL:         "https://deepmind.google/blog/rss.xml",
		Summary:     "Latest research and product updates.",
		PublishedAt: time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC),
	})

	if message.MsgType != "markdown" {
		t.Fatalf("msgtype = %q", message.MsgType)
	}
	if !strings.Contains(message.Markdown.Content, "Gemini 2.5 update") {
		t.Fatalf("unexpected markdown content: %q", message.Markdown.Content)
	}
	if !strings.Contains(message.Markdown.Content, "查看原文") {
		t.Fatalf("expected source link, got %q", message.Markdown.Content)
	}
}
