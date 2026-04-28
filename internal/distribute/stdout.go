package distribute

import (
	"context"

	"hotnew/internal/domain"
	"hotnew/internal/platform/logger"
)

type StdoutDistributor struct{}

func NewStdoutDistributor() StdoutDistributor {
	return StdoutDistributor{}
}

func (StdoutDistributor) Distribute(_ context.Context, article domain.Article) error {
	logger.Info("distributed article source=%s title=%q", article.Source, article.Title)
	return nil
}
