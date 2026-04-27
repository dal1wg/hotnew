package app

import (
	"context"
	"fmt"

	"hotnew/internal/domain"
	"hotnew/internal/ingest"
)

type Pipeline struct {
	normalizer  domain.Normalizer
	summarizer  domain.Summarizer
	store       domain.ArticleStore
	distributor domain.Distributor
	sources     []domain.Source
}

func NewPipeline(
	normalizer domain.Normalizer,
	summarizer domain.Summarizer,
	store domain.ArticleStore,
	distributor domain.Distributor,
) *Pipeline {
	return &Pipeline{
		normalizer:  normalizer,
		summarizer:  summarizer,
		store:       store,
		distributor: distributor,
	}
}

func (p *Pipeline) AddSource(source domain.Source) {
	p.sources = append(p.sources, source)
}

func (p *Pipeline) Run(ctx context.Context, limit int) (RunResult, error) {
	rawItems, err := ingest.NewService(p.sources).Run(ctx, domain.FetchRequest{Limit: limit})
	if err != nil {
		return RunResult{}, fmt.Errorf("ingest: %w", err)
	}

	result := RunResult{}
	for _, raw := range rawItems {
		article, err := p.normalizer.Normalize(raw)
		if err != nil {
			result.Failed++
			continue
		}

		summary, err := p.summarizer.Summarize(ctx, article)
		if err == nil {
			article.Summary = summary
		}

		created, err := p.store.Upsert(ctx, article)
		if err != nil {
			result.Failed++
			continue
		}
		if !created {
			result.Deduplicated++
			continue
		}

		if err := p.distributor.Distribute(ctx, article); err != nil {
			result.Failed++
			continue
		}

		result.Created++
	}

	return result, nil
}

type RunResult struct {
	Created      int `json:"created"`
	Deduplicated int `json:"deduplicated"`
	Failed       int `json:"failed"`
}
