package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Scheduler manages the execution of scheduled jobs, controlling their lifecycle
// and ensuring concurrency constraints. It provides mechanisms for job execution,
// monitoring, pausing, resuming, and retrying upon failures.
type Scheduler struct {
	cfg    Config             // Scheduler configuration settings.
	log    *slog.Logger       // Logger instance for logging scheduler activities.
	jobs   sync.Map           // Concurrent-safe storage for scheduled jobs.
	ctx    context.Context    // Root context for managing scheduler lifecycle.
	cancel context.CancelFunc // Function to cancel the scheduler, stopping all jobs.
}

// Config defines the configuration parameters of the Scheduler, determining job
// execution behavior, concurrency control, and monitoring intervals.
type Config struct {
	MaxWorkers    int           // Maximum number of jobs that can execute concurrently.
	IdleTimeout   time.Duration // Default timeout for jobs if not explicitly defined.
	CheckInterval time.Duration // Frequency at which the scheduler checks job statuses.
}

// New initializes and starts a new Scheduler instance with the given configuration.
//
// It ensures that valid defaults are applied if the configuration values are missing or invalid.
// The scheduler starts its internal monitoring process automatically upon creation.
//
// Parameters:
// - cfg: The configuration settings for the scheduler.
// - log: Logger instance for structured logging of job executions.
// - ctx: Parent context for managing scheduler lifecycle.
//
// Returns:
// - A pointer to the initialized Scheduler instance.
func New(cfg Config, log *slog.Logger, ctx context.Context) *Scheduler {
	ctx, cancel := context.WithCancel(ctx)

	// Apply default values if necessary.
	if cfg.MaxWorkers < 1 {
		cfg.MaxWorkers = DEFAULT_NUM_WORKERS
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = DEFAULT_CHECK_INTERVAL
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = DEFAULT_IDLE_TIMEOUT
	}

	// Create and initialize the scheduler.
	s := &Scheduler{
		cfg:    cfg,
		log:    log,
		jobs:   sync.Map{},
		ctx:    ctx,
		cancel: cancel,
	}

	// Start the scheduler's monitoring loop.
	go s.runScheduler()

	return s
}

// Add registers one or multiple jobs into the scheduler for execution.
//
// Each job is validated before being stored. If validation fails for any job,
// the function returns an error and does not add the invalid job.
//
// Parameters:
// - jobs: A variadic list of job pointers to be added.
//
// Returns:
// - An error if any job fails validation; otherwise, nil.
func (s *Scheduler) Add(jobs ...*Job) error {
	for _, job := range jobs {
		if err := s.sanitizeJob(job); err != nil {
			return newErr(ErrAddingJob, fmt.Sprintf("error: %v, id: %s", err, job.ID))
		}
		s.jobs.Store(job.ID, job)
	}
	return nil
}
