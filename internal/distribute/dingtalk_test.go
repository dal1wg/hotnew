package distribute

import (
	"testing"
	"time"

	"hotnew/internal/config"
	"hotnew/internal/domain"
)

func TestNewDingTalkDistributor(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.DingTalkConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid webhook",
			cfg: config.DingTalkConfig{
				Webhook: "https://oapi.dingtalk.com/robot/send?access_token=test_token",
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "empty webhook",
			cfg: config.DingTalkConfig{
				Webhook: "",
				Timeout: 5 * time.Second,
			},
			wantErr: true,
			errMsg:  "dingtalk webhook is required",
		},
		{
			name: "invalid webhook format",
			cfg: config.DingTalkConfig{
				Webhook: "https://invalid.com/robot/send?access_token=test",
				Timeout: 5 * time.Second,
			},
			wantErr: true,
			errMsg:  "invalid dingtalk webhook format",
		},
		{
			name: "webhook with backticks",
			cfg: config.DingTalkConfig{
				Webhook: "`https://oapi.dingtalk.com/robot/send?access_token=test_token`",
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "webhook with spaces",
			cfg: config.DingTalkConfig{
				Webhook: "  https://oapi.dingtalk.com/robot/send?access_token=test_token  ",
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dt, err := NewDingTalkDistributor(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q but got %q", tt.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if dt.webhook != "https://oapi.dingtalk.com/robot/send?access_token=test_token" {
				t.Errorf("webhook not cleaned correctly")
			}
		})
	}
}

func TestDingTalkDistributor_BuildMessage(t *testing.T) {
	dt := DingTalkDistributor{
		webhook: "https://oapi.dingtalk.com/robot/send?access_token=test",
	}

	article := domain.Article{
		Title:       "Test Article",
		Source:      "test-source",
		URL:         "https://example.com/article",
		Content:     "This is the article content",
		Summary:     "This is a summary",
		PublishedAt: time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC),
	}

	msg := dt.BuildMessage(article)

	if msg.MsgType != "markdown" {
		t.Errorf("expected msgtype markdown but got %s", msg.MsgType)
	}
	if msg.Markdown.Title != "Test Article" {
		t.Errorf("expected title Test Article but got %s", msg.Markdown.Title)
	}
	if msg.Markdown.Text == "" {
		t.Error("expected markdown text but got empty")
	}
}

func TestRenderDingTalkTitle(t *testing.T) {
	tests := []struct {
		name     string
		article  domain.Article
		expected string
	}{
		{
			name: "normal title",
			article: domain.Article{
				Title: "Test Title",
			},
			expected: "Test Title",
		},
		{
			name: "empty title",
			article: domain.Article{
				Title: "",
			},
			expected: "Untitled",
		},
		{
			name: "whitespace title",
			article: domain.Article{
				Title: "   ",
			},
			expected: "Untitled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderDingTalkTitle(tt.article)
			if result != tt.expected {
				t.Errorf("expected %q but got %q", tt.expected, result)
			}
		})
	}
}

func TestCompactText(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		limit    int
		expected string
	}{
		{
			name:     "normal text",
			value:    "Hello World",
			limit:    20,
			expected: "Hello World",
		},
		{
			name:     "text exceeding limit",
			value:    "Hello World This Is A Long Text",
			limit:    10,
			expected: "Hello...",
		},
		{
			name:     "text exactly at limit",
			value:    "Hello",
			limit:    5,
			expected: "Hello",
		},
		{
			name:     "text with extra spaces",
			value:    "Hello    World",
			limit:    20,
			expected: "Hello World",
		},
		{
			name:     "zero limit",
			value:    "Hello World",
			limit:    0,
			expected: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compactText(tt.value, tt.limit)
			if result != tt.expected {
				t.Errorf("expected %q but got %q", tt.expected, result)
			}
		})
	}
}