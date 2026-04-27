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

type FileDeliveryStore struct {
	mu      sync.RWMutex
	file    *os.File
	records []domain.DeliveryRecord
}

func NewFileDeliveryStore(dataDir string) (*FileDeliveryStore, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	return NewFileDeliveryStoreAt(filepath.Join(dataDir, "deliveries.jsonl"))
}

func NewFileDeliveryStoreAt(path string) (*FileDeliveryStore, error) {
	if path == "" {
		path = filepath.Join("data", "deliveries.jsonl")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open delivery file: %w", err)
	}
	store := &FileDeliveryStore{file: file}
	if err := store.load(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return store, nil
}

func (s *FileDeliveryStore) load() error {
	if _, err := s.file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek delivery file: %w", err)
	}
	scanner := bufio.NewScanner(s.file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record domain.DeliveryRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return fmt.Errorf("decode delivery log: %w", err)
		}
		s.records = append(s.records, record)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan delivery log: %w", err)
	}
	_, err := s.file.Seek(0, 2)
	return err
}

func (s *FileDeliveryStore) Append(_ context.Context, record domain.DeliveryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal delivery record: %w", err)
	}
	if _, err := s.file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("append delivery log: %w", err)
	}
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("sync delivery log: %w", err)
	}
	s.records = append(s.records, record)
	return nil
}

func (s *FileDeliveryStore) List(_ context.Context, limit int) ([]domain.DeliveryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := append([]domain.DeliveryRecord(nil), s.records...)
	sort.SliceStable(records, func(i, j int) bool { return records[i].AttemptedAt.After(records[j].AttemptedAt) })
	if limit <= 0 || limit > len(records) {
		limit = len(records)
	}
	return append([]domain.DeliveryRecord(nil), records[:limit]...), nil
}

func (s *FileDeliveryStore) Close(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}
