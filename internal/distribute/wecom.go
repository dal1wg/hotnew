package distribute

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hotnew/internal/config"
	"hotnew/internal/domain"
)

type WeComDistributor struct {
	webhook string
	client  *http.Client
}

type weComMessage struct {
	MsgType  string        `json:"msgtype"`
	Markdown weComMarkdown `json:"markdown,omitempty"`
}

type weComMarkdown struct {
	Content string `json:"content"`
}

func NewWeComDistributor(cfg config.WeComConfig) (WeComDistributor, error) {
	webhook := strings.TrimSpace(cfg.Webhook)
	if webhook == "" {
		return WeComDistributor{}, fmt.Errorf("wecom webhook is required")
	}
	// 清理可能的多余字符，如反引号
	webhook = strings.Trim(webhook, "` ")
	// 验证URL格式
	if !strings.HasPrefix(webhook, "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=") {
		return WeComDistributor{}, fmt.Errorf("invalid wecom webhook format, expected: https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return WeComDistributor{
		webhook: webhook,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (d WeComDistributor) Distribute(ctx context.Context, article domain.Article) error {
	payload, err := json.Marshal(d.BuildMessage(article))
	if err != nil {
		return fmt.Errorf("marshal wecom message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.webhook, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wecom webhook returned status %s", resp.Status)
	}
	return nil
}

func (d WeComDistributor) BuildMessage(article domain.Article) weComMessage {
	return weComMessage{
		MsgType: "markdown",
		Markdown: weComMarkdown{
			Content: renderWeComMarkdown(article),
		},
	}
}

func renderWeComMarkdown(article domain.Article) string {
	title := strings.TrimSpace(article.Title)
	if title == "" {
		title = "Untitled"
	}
	summary := strings.TrimSpace(article.Summary)
	if summary == "" {
		summary = strings.TrimSpace(article.Content)
	}
	if summary == "" {
		summary = title
	}
	summary = compactText(summary, 320)

	lines := []string{
		fmt.Sprintf("# %s", title),
		fmt.Sprintf("> 来源：<font color=\"comment\">%s</font>", fallbackString(article.Source, "unknown")),
	}
	if summary != "" {
		lines = append(lines, summary)
	}
	if strings.TrimSpace(article.URL) != "" {
		lines = append(lines, fmt.Sprintf("[查看原文](%s)", strings.TrimSpace(article.URL)))
	}
	if !article.PublishedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("> 发布时间：<font color=\"comment\">%s</font>", article.PublishedAt.Format(time.RFC3339)))
	}
	return strings.Join(lines, "\n\n")
}

func compactText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit > 0 && len(value) > limit {
		value = strings.TrimSpace(value[:limit]) + "..."
	}
	return value
}
