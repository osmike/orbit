// Package pool implements the core orchestration engine responsible for executing and managing scheduled jobs.
//
// It is designed as an isolated execution unit that handles:
//   - Concurrent job execution with worker limit control.
//   - Job state transitions (Waiting, Running, Completed, Error).
//   - Periodic polling and scheduling based on intervals or cron rules.
//   - Graceful cancellation and shutdown of the execution engine.
//   - Integration with pluggable monitoring and external job factories.
//
// The pool is initialized with a context, lifecycle configuration, monitoring implementation,
// and a JobFactory — a function responsible for creating job instances from raw configuration (JobDTO).
package pool

import (
	"context"
	"github.com/osmike/orbit/internal/domain"
	errs "github.com/osmike/orbit/internal/error"
	"sync"
	"time"
)

// JobFactory defines a function signature responsible for converting a JobDTO
// into a fully initialized and executable Job instance.
//
// This abstraction allows the Pool to remain decoupled from any specific job implementation,
// following Dependency Inversion and facilitating testing or customization.
type JobFactory func(jobDTO domain.JobDTO, ctx context.Context, mon domain.Monitoring) (domain.Job, error)

// Pool manages the lifecycle and scheduling of jobs,
// including their concurrent execution, state transitions, and shutdown behavior.
//
// Jobs are stored in a concurrent map and executed periodically based on their scheduling configuration.
// The pool uses a worker semaphore to enforce concurrency limits and relies on the JobFactory to create jobs.
type Pool struct {
	domain.Pool                    // Embeds basic configuration: MaxWorkers, CheckInterval, IdleTimeout.
	mon         domain.Monitoring  // Monitoring sink for job execution metrics.
	jobs        sync.Map           // Concurrent-safe storage of all active jobs by ID.
	ctx         context.Context    // Root context shared by all jobs and pool lifecycle.
	cancel      context.CancelFunc // Function to cancel the pool context.
	killed      bool               // Flag indicating whether the pool has been terminated and cannot be restarted.
	jf          JobFactory         // External job factory to create Job instances from configuration.
}

// New constructs and initializes a new Pool instance based on the provided configuration and job factory.
//
// It sets default values for MaxWorkers, CheckInterval, and IdleTimeout if they are unset (zero values).
//
// Parameters:
//   - ctx: Parent context used for cancellation and lifetime control.
//   - cfg: PoolConfig with concurrency and timing settings.
//   - mon: Monitoring interface used for collecting job metrics.
//   - jf: JobFactory used to create executable Job instances from configuration.
//
// Returns:
//   - A fully initialized Pool instance ready to run jobs.
//   - An error if the JobFactory is nil.
func New(ctx context.Context, cfg domain.Pool, mon domain.Monitoring, jf JobFactory) (*Pool, error) {
	if cfg.MaxWorkers == 0 {
		cfg.MaxWorkers = domain.DEFAULT_NUM_WORKERS
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = domain.DEFAULT_CHECK_INTERVAL
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = domain.DEFAULT_IDLE_TIMEOUT
	}

	if jf == nil {
		return nil, errs.New(errs.ErrJobFactoryIsNil, "pool error")
	}

	ctx, cancel := context.WithCancel(ctx)

	return &Pool{
		Pool:   cfg,
		ctx:    ctx,
		cancel: cancel,
		mon:    mon,
		killed: false,
		jf:     jf,
	}, nil
}

// GetJobByID looks up a job in the pool by its unique ID.
//
// Returns the job instance if found, or ErrJobNotFound otherwise.
func (p *Pool) GetJobByID(id string) (domain.Job, error) {
	jobInterface, ok := p.jobs.Load(id)
	if !ok {
		return nil, errs.New(errs.ErrJobNotFound, id)
	}
	return jobInterface.(domain.Job), nil
}

// Run launches the main scheduler loop that monitors job states and dispatches jobs for execution.
//
// Behavior:
//   - Executes jobs based on their status and readiness (Waiting, Running, Completed, Error).
//   - Enforces MaxWorkers using a semaphore.
//   - Periodically polls jobs at the configured CheckInterval.
//   - Gracefully handles shutdown via context cancellation:
//   - Waits for in-progress jobs to complete.
//   - Updates all jobs to Stopped state with a shutdown error.
//   - Clears internal job storage and marks the pool as killed.
//
// Returns:
//   - ErrPoolShutdown if the pool was already stopped or after termination.
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
			case <-p.ctx.Done():
				wg.Wait()

				p.jobs.Range(func(key, value interface{}) bool {
					job := value.(domain.Job)
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
					job := value.(domain.Job)
					status := job.GetStatus()
					switch status {
					case domain.Waiting:
						p.ProcessWaiting(job, sem, &wg)
					case domain.Running:
						p.ProcessRunning(job)
					case domain.Completed:
						p.ProcessCompleted(job)
					case domain.Error:
						p.ProcessError(job)
					}
					return true
				})
			}
		}
	}()

	return err
}

// GetMetrics returns a snapshot of all job metrics collected by the monitoring implementation.
//
// This includes runtime data such as execution time, success/failure counts, current state, etc.
func (p *Pool) GetMetrics() map[string]interface{} {
	return p.mon.GetMetrics()
}
