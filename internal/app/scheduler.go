package app

import (
	"context"
	"sync"
	"time"

	"hotnew/internal/platform/logger"
)

type Scheduler struct {
	runner           *Runner
	interval         time.Duration
	runLimit         int
	runTimeout       time.Duration
	startImmediately bool

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewScheduler(runner *Runner, interval time.Duration, runLimit int, runTimeout time.Duration, startImmediately bool) *Scheduler {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if runTimeout <= 0 {
		runTimeout = 30 * time.Second
	}
	return &Scheduler{
		runner:           runner,
		interval:         interval,
		runLimit:         runLimit,
		runTimeout:       runTimeout,
		startImmediately: startImmediately,
	}
}

func (s *Scheduler) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.wg.Add(1)
	go s.loop(ctx)
}

func (s *Scheduler) Stop(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.wg.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()

	if s.startImmediately {
		s.runOnce(ctx)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, s.runTimeout)
	defer cancel()

	result, err := s.runner.RunNow(runCtx, s.runLimit, "scheduler")
	if err != nil {
		logger.Error("scheduler run failed: %v", err)
		return
	}
	logger.Info("scheduler run completed: created=%d deduplicated=%d failed=%d", result.Created, result.Deduplicated, result.Failed)
}
