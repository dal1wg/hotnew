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

type runnerFakeSource struct {
	items []domain.RawItem
}

func (f runnerFakeSource) Name() string { return "fake" }
func (f runnerFakeSource) Kind() string { return "test" }
func (f runnerFakeSource) Fetch(context.Context, domain.FetchRequest) ([]domain.RawItem, error) {
	return f.items, nil
}

func TestRunnerTracksStatus(t *testing.T) {
	p := NewPipeline(
		normalize.NewService(),
		summarize.NewRuleSummarizer(50),
		store.NewMemoryArticleStore(),
		distribute.NewMultiDistributor(),
	)
	p.AddSource(runnerFakeSource{items: []domain.RawItem{{Source: "fake", Title: "hello", URL: "https://example.com"}}})

	runner := NewRunner(p)
	result, err := runner.RunNow(context.Background(), 10, "manual")
	if err != nil {
		t.Fatalf("run now error: %v", err)
	}
	if result.Created != 1 {
		t.Fatalf("created = %d, want 1", result.Created)
	}

	status := runner.Status()
	if status.Running {
		t.Fatalf("expected runner to be idle")
	}
	if status.Trigger != "manual" {
		t.Fatalf("trigger = %q, want manual", status.Trigger)
	}
	if status.LastStarted.IsZero() || status.LastFinished.IsZero() {
		t.Fatalf("expected timestamps to be set")
	}
}
