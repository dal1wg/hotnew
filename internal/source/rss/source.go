package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hotnew/internal/config"
	"hotnew/internal/domain"
)

type Source struct {
	cfg    config.SourceConfig
	client *http.Client
}

func NewSource(cfg config.SourceConfig) Source {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	return Source{
		cfg: cfg,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s Source) Name() string {
	return s.cfg.Name
}

func (s Source) Kind() string {
	return s.cfg.Kind
}

func (s Source) Fetch(ctx context.Context, req domain.FetchRequest) ([]domain.RawItem, error) {
	if s.cfg.FeedURL == "" {
		return nil, fmt.Errorf("feed url is required")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.FeedURL, nil)
	if err != nil {
		return nil, err
	}
	if s.cfg.UserAgent != "" {
		httpReq.Header.Set("User-Agent", s.cfg.UserAgent)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit <= 0 {
		limit = s.cfg.FetchBatchSize
	}
	if limit <= 0 || limit > len(feed.Channel.Items) {
		limit = len(feed.Channel.Items)
	}

	items := make([]domain.RawItem, 0, limit)
	for i := 0; i < limit; i++ {
		item := feed.Channel.Items[i]
		items = append(items, domain.RawItem{
			Source:      s.cfg.Name,
			Title:       strings.TrimSpace(item.Title),
			URL:         strings.TrimSpace(item.Link),
			Author:      strings.TrimSpace(item.Author),
			PublishedAt: strings.TrimSpace(item.PubDate),
			Content:     strings.TrimSpace(item.Description),
			Tags:        append([]string{s.cfg.DefaultTag}, item.Categories...),
			Language:    feed.Channel.Language,
		})
	}

	return items, nil
}

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Language string    `xml:"language"`
	Items    []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Author      string   `xml:"author"`
	PubDate     string   `xml:"pubDate"`
	Description string   `xml:"description"`
	Categories  []string `xml:"category"`
}
