package store

import (
	"context"
	"sort"
	"sync"

	"hotnew/internal/domain"
)

type MemoryArticleStore struct {
	mu       sync.RWMutex
	articles map[string]domain.Article
	order    []string
}

func NewMemoryArticleStore() *MemoryArticleStore {
	return &MemoryArticleStore{
		articles: make(map[string]domain.Article),
	}
}

func (s *MemoryArticleStore) Upsert(_ context.Context, article domain.Article) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.articles[article.ID]; exists {
		s.articles[article.ID] = article
		return false, nil
	}

	s.articles[article.ID] = article
	s.order = append(s.order, article.ID)
	return true, nil
}

func (s *MemoryArticleStore) List(_ context.Context, limit int) ([]domain.Article, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.order) {
		limit = len(s.order)
	}

	items := make([]domain.Article, 0, limit)
	for i := len(s.order) - 1; i >= 0 && len(items) < limit; i-- {
		items = append(items, s.articles[s.order[i]])
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})
	return items, nil
}

func (s *MemoryArticleStore) Get(_ context.Context, id string) (domain.Article, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	article, ok := s.articles[id]
	return article, ok, nil
}
