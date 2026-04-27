package store

import (
	"context"
	"sort"
	"sync"

	"hotnew/internal/domain"
)

type MemoryDeliveryStore struct {
	mu      sync.RWMutex
	records []domain.DeliveryRecord
}

func NewMemoryDeliveryStore() *MemoryDeliveryStore {
	return &MemoryDeliveryStore{}
}

func (s *MemoryDeliveryStore) Append(_ context.Context, record domain.DeliveryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

func (s *MemoryDeliveryStore) List(_ context.Context, limit int) ([]domain.DeliveryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := append([]domain.DeliveryRecord(nil), s.records...)
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].AttemptedAt.After(records[j].AttemptedAt)
	})
	if limit <= 0 || limit > len(records) {
		limit = len(records)
	}
	return append([]domain.DeliveryRecord(nil), records[:limit]...), nil
}
