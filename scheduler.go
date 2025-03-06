package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Scheduler manages the lifecycle and execution of scheduled jobs.
type Scheduler struct {
	cfg    Config             // Scheduler configuration settings.
	log    *slog.Logger       // Logger instance for logging scheduler activities.
	jobs   sync.Map           // Concurrent-safe storage for jobs.
	ctx    context.Context    // Context to control the scheduler lifecycle.
	cancel context.CancelFunc // Function to cancel the scheduler context.
}

// Config defines the scheduler's configuration parameters.
type Config struct {
	MaxWorkers    int           // Maximum concurrent job executions
	IdleTimeout   time.Duration // Default job timeout if not specified
	CheckInterval time.Duration // Frequency of checking job statuses
}

// New creates and starts a new Scheduler.
func New(cfg Config, log *slog.Logger, ctx context.Context) *Scheduler {
	ctx, cancel := context.WithCancel(ctx)
	if cfg.MaxWorkers < 1 {
		cfg.MaxWorkers = 100_000
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 100 * time.Millisecond
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 100 * time.Hour
	}
	s := &Scheduler{
		cfg:    cfg,
		log:    log,
		jobs:   sync.Map{},
		ctx:    ctx,
		cancel: cancel,
	}
	go s.runScheduler()
	return s
}

// Add registers one or multiple jobs to the scheduler.
func (s *Scheduler) Add(jobs ...*Job) error {
	for _, job := range jobs {
		if err := s.sanitizeJob(job); err != nil {
			return fmt.Errorf("error adding job - %v, %w, err: %v", job, ErrAddingJob, err)
		}
		s.jobs.Store(job.ID, job)
	}
	return nil
}
