package distribute

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"hotnew/internal/domain"
)

type AsyncDistributor struct {
	downstream domain.Distributor
	queue      chan domain.Article
	wg         sync.WaitGroup

	mu     sync.RWMutex
	closed bool
}

func NewAsyncDistributor(buffer int, downstream domain.Distributor) *AsyncDistributor {
	if buffer <= 0 {
		buffer = 128
	}
	d := &AsyncDistributor{
		downstream: downstream,
		queue:      make(chan domain.Article, buffer),
	}
	d.wg.Add(1)
	go d.loop()
	return d
}

func (d *AsyncDistributor) Distribute(_ context.Context, article domain.Article) error {
	d.mu.RLock()
	closed := d.closed
	d.mu.RUnlock()
	if closed {
		return errors.New("async distributor closed")
	}

	select {
	case d.queue <- article:
		return nil
	default:
		return fmt.Errorf("async distributor queue full")
	}
}

func (d *AsyncDistributor) Close(ctx context.Context) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	close(d.queue)
	d.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		d.wg.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (d *AsyncDistributor) loop() {
	defer d.wg.Done()
	for article := range d.queue {
		_ = d.downstream.Distribute(context.Background(), article)
	}
}
