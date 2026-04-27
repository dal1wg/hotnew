package distribute

import (
	"context"
	"log"

	"hotnew/internal/domain"
)

type StdoutDistributor struct{}

func NewStdoutDistributor() StdoutDistributor {
	return StdoutDistributor{}
}

func (StdoutDistributor) Distribute(_ context.Context, article domain.Article) error {
	log.Printf("distributed article source=%s title=%q", article.Source, article.Title)
	return nil
}
