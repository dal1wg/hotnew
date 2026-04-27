package distribute

import (
	"context"

	"hotnew/internal/domain"
)

type MultiDistributor struct {
	items []domain.Distributor
}

func NewMultiDistributor(items ...domain.Distributor) MultiDistributor {
	return MultiDistributor{items: items}
}

func (m MultiDistributor) Distribute(ctx context.Context, article domain.Article) error {
	for _, item := range m.items {
		if err := item.Distribute(ctx, article); err != nil {
			return err
		}
	}
	return nil
}
