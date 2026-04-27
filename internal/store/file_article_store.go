package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"hotnew/internal/domain"
)

type FileArticleStore struct {
	mu       sync.RWMutex
	path     string
	file     *os.File
	articles map[string]domain.Article
	order    []string
}

func NewFileArticleStore(dataDir string) (*FileArticleStore, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	return NewFileArticleStoreAt(filepath.Join(dataDir, "articles.jsonl"))
}

func NewFileArticleStoreAt(path string) (*FileArticleStore, error) {
	if path == "" {
		path = filepath.Join("data", "articles.jsonl")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open store file: %w", err)
	}
	store := &FileArticleStore{path: path, file: file, articles: make(map[string]domain.Article)}
	if err := store.load(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return store, nil
}

func (s *FileArticleStore) load() error {
	if _, err := s.file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek store file: %w", err)
	}
	scanner := bufio.NewScanner(s.file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var article domain.Article
		if err := json.Unmarshal(line, &article); err != nil {
			return fmt.Errorf("decode article log: %w", err)
		}
		if _, exists := s.articles[article.ID]; !exists {
			s.order = append(s.order, article.ID)
		}
		s.articles[article.ID] = article
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan article log: %w", err)
	}
	_, err := s.file.Seek(0, 2)
	return err
}

func (s *FileArticleStore) Upsert(_ context.Context, article domain.Article) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.articles[article.ID]
	if !exists {
		s.order = append(s.order, article.ID)
	}
	s.articles[article.ID] = article
	payload, err := json.Marshal(article)
	if err != nil {
		return false, fmt.Errorf("marshal article: %w", err)
	}
	if _, err := s.file.Write(append(payload, '\n')); err != nil {
		return false, fmt.Errorf("append article log: %w", err)
	}
	if err := s.file.Sync(); err != nil {
		return false, fmt.Errorf("sync article log: %w", err)
	}
	return !exists, nil
}

func (s *FileArticleStore) List(_ context.Context, limit int) ([]domain.Article, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.order) {
		limit = len(s.order)
	}
	items := make([]domain.Article, 0, limit)
	for i := len(s.order) - 1; i >= 0 && len(items) < limit; i-- {
		items = append(items, s.articles[s.order[i]])
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].PublishedAt.After(items[j].PublishedAt) })
	return items, nil
}

func (s *FileArticleStore) Get(_ context.Context, id string) (domain.Article, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	article, ok := s.articles[id]
	return article, ok, nil
}

func (s *FileArticleStore) Close(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}
