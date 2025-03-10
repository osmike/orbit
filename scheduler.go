package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
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
	mon    Monitor            // Monitoring interface for tracking job execution.
}

// Config defines the configuration parameters of the Scheduler, determining job
// execution behavior, concurrency control, and monitoring intervals.
type Config struct {
	MaxWorkers    int           // Maximum number of jobs that can execute concurrently.
	IdleTimeout   time.Duration // Default timeout for jobs if not explicitly defined.
	CheckInterval time.Duration // Frequency at which the scheduler checks job statuses.
}

type Monitor interface {
	AddJob(job *JobMetadata)
	GetJobByID(id string) *JobMetadata
	GetJobs() []*JobMetadata
	UpdateState(id string, state *State)
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
func New(cfg Config, log *slog.Logger, mon Monitor, ctx context.Context) *Scheduler {
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

	if mon == nil {
		mon = NewDefaultMonitor()
	}

	// Create and initialize the scheduler.
	s := &Scheduler{
		cfg:    cfg,
		log:    log,
		mon:    mon,
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
		s.mon.AddJob(&JobMetadata{
			ID:      job.ID,
			Name:    job.Name,
			StartAt: job.StartAt,
			EndAt:   job.EndAt,
			State: State{
				Status: func() atomic.Value {
					var v atomic.Value
					v.Store(Waiting)
					return v
				}(),
				StartAt:       job.State.StartAt,
				EndAt:         job.State.EndAt,
				Error:         job.State.Error,
				ExecutionTime: job.State.ExecutionTime,
				Data:          sync.Map{},
			},
		})
	}
	return nil
}

func (s *Scheduler) Stop(id string) error {
	job, err := s.getJobByID(id)
	if err != nil {
		return err
	}
	job.cancel()
	return nil
}

func (s *Scheduler) Pause(id string) error {
	job, err := s.getJobByID(id)
	if err != nil {
		return err
	}
	select {
	case job.pauseCh <- struct{}{}:
		if !job.tryChangeStatus([]JobStatus{Running}, Paused) {
			return fmt.Errorf("job %s is not running", id)
		}
	default:
		return fmt.Errorf("job %s is already paused or PauseChan is not being read", id)
	}
	return nil
}

func (s *Scheduler) Resume(id string) error {
	job, err := s.getJobByID(id)
	if err != nil {
		return err
	}
	select {
	case job.resumeCh <- struct{}{}:
		if !job.tryChangeStatus([]JobStatus{Paused}, Running) {
			return fmt.Errorf("job %s is not paused", id)
		}
	default:
		return fmt.Errorf("job %s is not paused or ResumeChan is not being read", id)
	}
	return nil
}

func (s *Scheduler) Delete(id string) error {
	job, err := s.getJobByID(id)
	if err != nil {
		return err
	}
	job.cancel()
	job.setStatus(Stopped)
	s.jobs.Delete(id)
	return nil
}
