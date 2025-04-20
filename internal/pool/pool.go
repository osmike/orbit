// Package pool provides the core scheduling and orchestration engine for job execution.
//
// It is responsible for:
//   - Concurrent execution of scheduled jobs with respect to MaxWorkers.
//   - Lifecycle management of jobs (waiting, running, completed, error).
//   - Periodic polling of job states at configurable intervals.
//   - Graceful shutdown and cleanup of job state and resources.
//
// Each pool operates as an independent execution unit, isolated via context for cancellation
// and monitoring support.
package pool

import (
	"context"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// Job represents an executable task managed by the scheduler.
// It encapsulates lifecycle management methods, status updates, execution logic, and retry mechanisms.
//
// Each Job implementation must provide thread-safe access to internal state and logic,
// ensuring safe concurrent interactions within the Pool environment.
type Job interface {
	// GetMetadata returns the job's configuration metadata.
	GetMetadata() domain.JobDTO

	// SetTimeout sets the max allowed job execution time; 0 disables timeout tracking.
	SetTimeout(timeout time.Duration)

	// GetStatus returns the current execution status of the job.
	GetStatus() domain.JobStatus

	// UpdateState partially updates the job's state with non-zero fields from the provided DTO.
	UpdateState(state domain.StateDTO)

	// NextRun calculates and returns the next scheduled execution time for the job.
	NextRun() time.Time

	// ProcessStart marks the job's state as "Running", initializing execution metrics and metadata.
	ProcessStart()

	// ProcessRun monitors the job during execution, checking for timeout conditions.
	// Returns ErrJobTimeout if the job exceeds its configured timeout.
	ProcessRun() error

	// ProcessEnd finalizes the job state after execution completes, recording metrics and handling errors.
	ProcessEnd(status domain.JobStatus, err error)

	// ProcessError performs retry logic and updates the job state in case of failure.
	ProcessError() error

	// CanExecute checks if the job meets conditions required to start execution immediately.
	// Returns an error indicating why execution is not allowed, or nil if eligible.
	CanExecute() error

	// Execute performs the job's main execution logic, handling internal lifecycle hooks and error handling.
	Execute() error

	// Stop forcibly stops job execution and updates its state accordingly.
	Stop() error

	// Pause attempts to pause job execution with a specified timeout.
	Pause(timeout time.Duration) error

	// Resume resumes execution of a paused job, if applicable.
	Resume() error

	// LockJob acquires exclusive execution access to the job if it is available.
	LockJob() bool

	// UnlockJob releases exclusive execution access to the job.
	UnlockJob()
}

// Pool manages scheduling, execution, lifecycle control, and concurrency of multiple jobs.
//
// Pool utilizes a background worker loop, controlled by context cancellation,
// to continuously check and execute jobs based on their defined schedules and states.
type Pool struct {
	domain.Pool                    // Configuration settings (max workers, check intervals, idle timeouts).
	Mon         domain.Monitoring  // Monitoring implementation for capturing execution metrics.
	jobs        sync.Map           // Concurrent-safe storage of active jobs.
	Ctx         context.Context    // Execution context for the scheduler pool.
	cancel      context.CancelFunc // Context cancellation function to gracefully stop the scheduler.
	killed      bool               // Indicates whether the pool has been terminated.
}

// New initializes and configures a new Pool instance with provided settings.
//
// It sets default values for configuration parameters if they are not explicitly defined.
//
// Parameters:
//   - ctx: Parent context for cancellation and graceful shutdown control.
//   - cfg: Pool configuration specifying worker limits, intervals, and timeouts.
//   - mon: Monitoring implementation for capturing job execution metrics.
//
// Returns:
//   - A fully initialized Pool instance ready for execution.
//   - An error if the configuration is invalid.
func New(ctx context.Context, cfg domain.Pool, mon domain.Monitoring) (*Pool, error) {
	if cfg.MaxWorkers == 0 {
		cfg.MaxWorkers = domain.DEFAULT_NUM_WORKERS
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = domain.DEFAULT_CHECK_INTERVAL
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = domain.DEFAULT_IDLE_TIMEOUT
	}

	ctx, cancel := context.WithCancel(ctx)

	return &Pool{
		Pool:   cfg,
		Ctx:    ctx,
		cancel: cancel,
		Mon:    mon,
		killed: false,
	}, nil
}

// getJobByID retrieves a job instance by its unique identifier.
//
// Parameters:
//   - id: Unique identifier of the job.
//
// Returns:
//   - The Job instance matching the provided ID.
//   - An error (ErrJobNotFound) if the job is not present in the pool.
func (p *Pool) getJobByID(id string) (Job, error) {
	jobInterface, ok := p.jobs.Load(id)
	if !ok {
		return nil, errs.New(errs.ErrJobNotFound, id)
	}
	return jobInterface.(Job), nil
}

// Run initiates the scheduler's main execution loop, periodically checking and managing jobs based on their states.
//
// Execution flow:
//   - Runs continuously until the Pool's context is canceled.
//   - Checks job states at regular intervals defined by CheckInterval.
//   - Manages job lifecycle transitions (waiting, running, completed, error).
//   - Controls concurrency using a semaphore to enforce MaxWorkers limits.
//   - Ensures graceful shutdown by waiting for all active jobs to complete upon cancellation.
//
// Returns:
//   - An error (ErrPoolShutdown) if the pool was previously terminated.
func (p *Pool) Run() (err error) {
	if p.killed {
		return errs.ErrPoolShutdown
	}

	go func() {
		ticker := time.NewTicker(p.CheckInterval)
		defer ticker.Stop()

		sem := make(chan struct{}, p.MaxWorkers)
		var wg sync.WaitGroup

		for {
			select {
			case <-p.Ctx.Done():
				wg.Wait()

				p.jobs.Range(func(key, value interface{}) bool {
					job := value.(Job)
					job.UpdateState(domain.StateDTO{
						Status: domain.Stopped,
						Error: domain.StateError{
							JobError: errs.ErrPoolShutdown,
						},
					})
					p.jobs.Delete(key)
					return true
				})

				p.killed = true
				err = errs.ErrPoolShutdown
				return

			case <-ticker.C:
				p.jobs.Range(func(key, value any) bool {
					job := value.(Job)
					switch job.GetStatus() {
					case domain.Waiting:
						p.processWaiting(job, sem, &wg)
					case domain.Running:
						p.processRunning(job)
					case domain.Completed:
						p.processCompleted(job)
					case domain.Error:
						p.processError(job)
					}
					return true
				})
			}
		}
	}()

	return err
}
