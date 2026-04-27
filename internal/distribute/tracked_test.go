package distribute

import (
	"context"
	"errors"
	"testing"
	"time"

	"hotnew/internal/domain"
	"hotnew/internal/store"
)

type trackedDistributorStub struct {
	err error
}

func (s trackedDistributorStub) Distribute(context.Context, domain.Article) error {
	return s.err
}

func TestTrackedDistributorRecordsFailures(t *testing.T) {
	deliveryStore := store.NewMemoryDeliveryStore()
	retryQueue := store.NewMemoryRetryQueue()
	distributor := NewTrackedDistributor(
		"blog",
		"https://example.com",
		trackedDistributorStub{err: errors.New("boom")},
		deliveryStore,
		retryQueue,
		3,
		time.Minute,
	)

	err := distributor.Distribute(context.Background(), domain.Article{ID: "a1", Hash: "h1"})
	if err == nil {
		t.Fatalf("expected error")
	}

	records, err := deliveryStore.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 || records[0].Status != "failed" {
		t.Fatalf("unexpected records: %+v", records)
	}

	jobs, err := retryQueue.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Channel != "blog" {
		t.Fatalf("unexpected retry jobs: %+v", jobs)
	}
}
