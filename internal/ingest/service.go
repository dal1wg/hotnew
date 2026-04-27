package ingest

import (
	"context"

	"hotnew/internal/domain"
)

type Service struct {
	sources []domain.Source
}

func NewService(sources []domain.Source) Service {
	return Service{sources: sources}
}

func (s Service) Run(ctx context.Context, req domain.FetchRequest) ([]domain.RawItem, error) {
	items := make([]domain.RawItem, 0)
	for _, source := range s.sources {
		fetched, err := source.Fetch(ctx, req)
		if err != nil {
			return nil, err
		}
		items = append(items, fetched...)
	}
	return items, nil
}
