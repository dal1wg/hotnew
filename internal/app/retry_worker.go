package app

import (
	"context"
	"log"
	"sync"
	"time"
)

type RetryWorker struct {
	processor *RetryProcessor
	interval  time.Duration
	batchSize int
	timeout   time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewRetryWorker(processor *RetryProcessor, interval time.Duration, batchSize int, timeout time.Duration) *RetryWorker {
	if interval <= 0 {
		interval = time.Minute
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &RetryWorker{processor: processor, interval: interval, batchSize: batchSize, timeout: timeout}
}

func (w *RetryWorker) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	w.wg.Add(1)
	go w.loop(ctx)
}

func (w *RetryWorker) Stop(ctx context.Context) error {
	if w.cancel != nil {
		w.cancel()
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.wg.Wait()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (w *RetryWorker) loop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, w.timeout)
			result, err := w.processor.ProcessOnce(runCtx, w.batchSize)
			cancel()
			if err != nil {
				log.Printf("retry worker failed: %v", err)
				continue
			}
			if result.Claimed > 0 {
				log.Printf("retry worker processed: claimed=%d succeeded=%d failed=%d", result.Claimed, result.Succeeded, result.Failed)
			}
		}
	}
}
