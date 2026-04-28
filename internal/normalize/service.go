package normalize

import (
	"fmt"
	"strings"
	"time"

	"hotnew/internal/domain"
	"hotnew/internal/platform/hash"
)

type Service struct{}

func NewService() Service {
	return Service{}
}

func (Service) Normalize(raw domain.RawItem) (domain.Article, error) {
	title := strings.TrimSpace(raw.Title)
	link := strings.TrimSpace(raw.URL)
	if title == "" || link == "" || raw.Source == "" {
		return domain.Article{}, fmt.Errorf("invalid raw item: source, title and url are required")
	}

	publishedAt := parseTime(raw.PublishedAt)
	content := strings.TrimSpace(raw.Content)
	fingerprint := hash.Fingerprint(raw.Source, title, link)

	return domain.Article{
		ID:          fingerprint,
		Source:      raw.Source,
		Title:       title,
		URL:         link,
		Author:      strings.TrimSpace(raw.Author),
		PublishedAt: publishedAt,
		Content:     content,
		Tags:        compactStrings(raw.Tags),
		Language:    strings.TrimSpace(raw.Language),
	}, nil
}

func parseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC850,
	}

	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts.UTC()
		}
	}

	return time.Time{}
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}
