package summarize

import (
	"context"
	"strings"

	"hotnew/internal/domain"
)

type RuleSummarizer struct {
	maxChars int
}

func NewRuleSummarizer(maxChars int) RuleSummarizer {
	if maxChars <= 0 {
		maxChars = 180
	}
	return RuleSummarizer{maxChars: maxChars}
}

func (r RuleSummarizer) Summarize(_ context.Context, article domain.Article) (string, error) {
	base := strings.TrimSpace(article.Content)
	if base == "" {
		base = strings.TrimSpace(article.Title)
	}
	if len([]rune(base)) <= r.maxChars {
		return base, nil
	}

	runes := []rune(base)
	return string(runes[:r.maxChars]) + "...", nil
}
