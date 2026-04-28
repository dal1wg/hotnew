package ingest

import (
	"context"
	"fmt"

	"hotnew/internal/domain"
	"hotnew/internal/platform/logger"
)

type Service struct {
	sources []domain.Source
}

func NewService(sources []domain.Source) Service {
	return Service{sources: sources}
}

func (s Service) Run(ctx context.Context, req domain.FetchRequest) ([]domain.RawItem, error) {
	items := make([]domain.RawItem, 0)
	var errs []error
	for _, source := range s.sources {
		fetched, err := source.Fetch(ctx, req)
		if err != nil {
			logger.Warn("failed to fetch from source %s: %v", source.Name(), err)
			errs = append(errs, fmt.Errorf("source %s: %w", source.Name(), err))
			continue
		}
		logger.Info("fetched %d items from source %s", len(fetched), source.Name())
		items = append(items, fetched...)
	}
	if len(errs) > 0 && len(items) == 0 {
		return nil, fmt.Errorf("all sources failed: %v", errs)
	}
	if len(errs) > 0 {
		logger.Warn("%d sources failed but continuing with %d items from successful sources", len(errs), len(items))
	}
	return items, nil
}
