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

type BlogDistributor struct {
	endpoint  string
	authToken string
	mode      string
	siteName  string
	author    string
	client    *http.Client
}

func NewBlogDistributor(cfg config.BlogConfig) (BlogDistributor, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return BlogDistributor{}, fmt.Errorf("blog endpoint is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	mode := strings.TrimSpace(strings.ToLower(cfg.Mode))
	if mode == "" {
		mode = "markdown"
	}

	return BlogDistributor{
		endpoint:  endpoint,
		authToken: strings.TrimSpace(cfg.AuthToken),
		mode:      mode,
		siteName:  strings.TrimSpace(cfg.SiteName),
		author:    strings.TrimSpace(cfg.Author),
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (d BlogDistributor) Distribute(ctx context.Context, article domain.Article) error {
	post := d.BuildPost(article)
	payload, err := json.Marshal(post)
	if err != nil {
		return fmt.Errorf("marshal blog post: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+d.authToken)
	}
	req.Header.Set("X-Idempotency-Key", post.IdempotencyKey)

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("blog endpoint returned status %s", resp.Status)
	}
	return nil
}

func (d BlogDistributor) BuildPost(article domain.Article) domain.BlogPost {
	title := strings.TrimSpace(article.Title)
	excerpt := strings.TrimSpace(article.Summary)
	if excerpt == "" {
		excerpt = title
	}

	return domain.BlogPost{
		ID:             article.ID,
		Title:          title,
		Slug:           slugify(title, article.ID),
		Excerpt:        excerpt,
		Content:        renderBlogContent(d.mode, article),
		Author:         fallbackString(d.author, article.Author, "hotnew"),
		Tags:           append([]string(nil), article.Tags...),
		SourceName:     article.Source,
		SourceURL:      article.URL,
		SourceAt:       article.PublishedAt,
		Language:       article.Language,
		SiteName:       fallbackString(d.siteName, "hotnew"),
		IdempotencyKey: article.Hash,
	}
}

func renderBlogContent(mode string, article domain.Article) string {
	switch mode {
	case "summary":
		return renderSummaryContent(article)
	default:
		return renderMarkdownContent(article)
	}
}

func renderSummaryContent(article domain.Article) string {
	body := strings.TrimSpace(article.Summary)
	if body == "" {
		body = strings.TrimSpace(article.Title)
	}
	return body + "\n\n来源：" + article.Source + "\n原文：" + article.URL
}

func renderMarkdownContent(article domain.Article) string {
	body := strings.TrimSpace(article.Content)
	if body == "" {
		body = strings.TrimSpace(article.Summary)
	}
	if body == "" {
		body = strings.TrimSpace(article.Title)
	}

	var builder strings.Builder
	builder.WriteString("## 摘要\n\n")
	if strings.TrimSpace(article.Summary) != "" {
		builder.WriteString(article.Summary)
	} else {
		builder.WriteString(strings.TrimSpace(article.Title))
	}
	builder.WriteString("\n\n## 正文\n\n")
	builder.WriteString(body)
	builder.WriteString("\n\n## 来源\n\n")
	builder.WriteString("- 来源站点：")
	builder.WriteString(article.Source)
	builder.WriteString("\n- 原文链接：")
	builder.WriteString(article.URL)
	if !article.PublishedAt.IsZero() {
		builder.WriteString("\n- 发布时间：")
		builder.WriteString(article.PublishedAt.Format(time.RFC3339))
	}
	return builder.String()
}

func slugify(title, suffix string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	var builder strings.Builder
	lastDash := false
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		slug = "post"
	}
	suffix = strings.TrimSpace(suffix)
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	if suffix != "" {
		slug += "-" + suffix
	}
	return slug
}

func fallbackString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
