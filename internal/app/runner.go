package app

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrRunInProgress = errors.New("pipeline run already in progress")

type Runner struct {
	pipeline *Pipeline

	mu     sync.RWMutex
	status RunStatus
}

type RunStatus struct {
	Running      bool      `json:"running"`
	Trigger      string    `json:"trigger,omitempty"`
	LastStarted  time.Time `json:"last_started,omitempty"`
	LastFinished time.Time `json:"last_finished,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
	LastResult   RunResult `json:"last_result"`
}

func NewRunner(pipeline *Pipeline) *Runner {
	return &Runner{pipeline: pipeline}
}

func (r *Runner) RunNow(ctx context.Context, limit int, trigger string) (RunResult, error) {
	r.mu.Lock()
	if r.status.Running {
		r.mu.Unlock()
		return RunResult{}, ErrRunInProgress
	}
	r.status.Running = true
	r.status.Trigger = trigger
	r.status.LastStarted = time.Now().UTC()
	r.status.LastError = ""
	r.mu.Unlock()

	result, err := r.pipeline.Run(ctx, limit)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.Running = false
	r.status.LastFinished = time.Now().UTC()
	r.status.LastResult = result
	if err != nil {
		r.status.LastError = err.Error()
	}
	return result, err
}

func (r *Runner) Status() RunStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}
