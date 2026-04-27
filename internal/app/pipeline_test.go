package app

import (
	"context"
	"testing"

	"hotnew/internal/distribute"
	"hotnew/internal/domain"
	"hotnew/internal/normalize"
	"hotnew/internal/store"
	"hotnew/internal/summarize"
)

type fakeSource struct {
	items []domain.RawItem
}

func (f fakeSource) Name() string { return "fake" }
func (f fakeSource) Kind() string { return "test" }
func (f fakeSource) Fetch(context.Context, domain.FetchRequest) ([]domain.RawItem, error) {
	return f.items, nil
}

func TestPipelineRun(t *testing.T) {
	p := NewPipeline(
		normalize.NewService(),
		summarize.NewRuleSummarizer(50),
		store.NewMemoryArticleStore(),
		distribute.NewMultiDistributor(),
	)
	p.AddSource(fakeSource{
		items: []domain.RawItem{{
			Source:  "fake",
			Title:   "hello",
			URL:     "https://example.com",
			Content: "some content",
		}},
	})

	result, err := p.Run(context.Background(), 10)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if result.Created != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
