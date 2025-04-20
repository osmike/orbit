// Package orbit provides a high-level abstraction for scheduling and executing concurrent jobs
// with support for retries, timeouts, interval and cron-based scheduling, and detailed lifecycle management.
//
// It exposes a user-friendly API for creating execution pools, registering jobs with customizable behavior,
// and tracking runtime state using pluggable monitoring implementations.
//
// Features:
//   - Configurable execution pools with worker limits and polling intervals.
//   - Job scheduling using fixed intervals or cron expressions.
//   - Per-job lifecycle hooks (onStart, onSuccess, onError, onPause, onResume, finally).
//   - Retry policies with retry limits and cooldown intervals.
//   - Timeout control and pause/resume support.
//   - Isolated runtime state and metadata tracking for each job.
//   - Integration with in-memory or user-provided monitoring backends.
//
// This package is intended to be used by applications that require consistent and controlled background task
// execution, periodic health checks, message dispatchers, or batch processing routines.
//
// Example usage:
//
//	s := go_scheduler.New(context.Background())
//
//	poolCfg := go_scheduler.PoolConfig{
//		ID:           "analytics-pool",
//		MaxWorkers:   10,
//		CheckInterval: 200 * time.Millisecond,
//	}
//
//	pool, _ := s.CreatePool(poolCfg, nil)
//
//	jobCfg := go_scheduler.JobConfig{
//		ID: "report-job",
//		Fn: func(ctrl go_scheduler.FnControl) error {
//			// perform some logic...
//			ctrl.SaveData(map[string]interface{}{"result": "ok"})
//			return nil
//		},
//		Interval: go_scheduler.IntervalConfig{Time: 5 * time.Second},
//		Retry:    go_scheduler.RetryConfig{Count: 3, Time: 1 * time.Second},
//	}
//
//	s.AddJob(pool, jobCfg)
package orbit

import (
	"context"
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"go-scheduler/internal/job"
	"go-scheduler/internal/pool"
	defaultMonitoring "go-scheduler/monitoring"
)

// PoolConfig encapsulates the configuration settings required to initialize a new scheduler pool.
//
// Parameters:
//   - MaxWorkers: Maximum number of concurrent workers to execute jobs.
//     Default is 1000 if set to 0.
//   - CheckInterval: Time at which the scheduler checks for jobs to execute.
//     Default is 100ms if set to 0.
//   - IdleTimeout: Duration after which idle workers may be terminated.
//     Default is 100 hours if set to 0.
type PoolConfig = domain.Pool

// Pool represents a job execution pool.
//
// Provides methods for managing job execution lifecycle, concurrency,
// job state control, and interaction with monitoring.
type Pool = pool.Pool

// JobConfig defines a job's configuration and execution details.
//
// Parameters:
//   - ID: Unique identifier for the job.
//   - Name: Human-readable name of the job.
//   - Fn: The function executed by the job.
//   - Schedule: Scheduling parameters (interval or cron expression).
//   - Timeout: Maximum allowed execution duration for the job.
//   - StartAt: Earliest time when the job is allowed to start.
//   - EndAt: Latest time when the job can still run.
//   - Retry: Retry behavior in case of job execution failures.
//   - Hooks: Lifecycle hooks for custom logic execution.
type JobConfig = domain.JobDTO

// IntervalConfig encapsulates job scheduling settings.
//
// Parameters:
//   - Time: Time between job executions (set 0 if using cron expression).
//   - CronExpr: Cron expression defining job execution schedule (leave empty if using interval).
type IntervalConfig = domain.Interval

// RetryConfig defines retry behavior for a job.
//
// Parameters:
//   - Count: Number of allowed retries after execution failure.
//   - Time: Time interval between retries.
//   - ResetOnSuccess: Flag to reset retries count after successful job execution.
type RetryConfig = domain.Retry

// HooksFunc provides lifecycle hooks to inject custom logic at different job execution stages.
//
// Available hooks:
//   - OnStart: Executed before job starts.
//   - OnStop: Executed when job is explicitly stopped.
//   - OnError: Executed if job execution encounters an error.
//   - OnSuccess: Executed when job completes successfully.
//   - OnPause: Executed when job is paused.
//   - OnResume: Executed when job resumes after pause.
//   - Finally: Always executed after job ends (successful, error, paused, stopped).
type HooksFunc = domain.Hooks

// FnControl provides job execution control and runtime metadata storage.
//
// Offers control mechanisms:
//   - Context: Execution context (cancellation, deadlines).
//   - PauseChan: Channel signaling pause requests.
//   - ResumeChan: Channel signaling resume requests.
//
// Methods:
//   - SaveData(map[string]interface{}): Stores custom job runtime metadata.
type FnControl = domain.FnControl

// Monitoring interface represents components responsible for collecting, storing,
// and processing job execution metrics.
//
// Implementations can persist metrics using various strategies, including in-memory,
// logging, databases, or external monitoring systems.
//
// Methods:
//   - SaveMetrics(StateDTO): Stores metrics for a given job state.
//   - GetMetrics() map[string]interface{}: Retrieves collected metrics.
type Monitoring = domain.Monitoring

// Scheduler orchestrates the creation and management of job execution pools and scheduled jobs.
//
// Provides a simplified API for pool creation, job addition, and lifecycle management.
//
// Methods:
//   - CreatePool: Initializes a new job execution pool with specified settings.
//   - AddJob: Creates and adds a job to a specified pool.
//
// Usage:
//
//	scheduler := go_scheduler.New(context.Background())
//	pool := scheduler.CreatePool(config, nil)
//	scheduler.AddJob(pool, jobConfig)
type Scheduler struct {
	ctx context.Context // Parent context to manage scheduler lifecycle.
}

// New initializes a new Scheduler instance.
//
// Parameters:
//   - ctx: Parent execution context used for managing graceful shutdown and global cancellation.
//
// Returns:
//   - Pointer to a new Scheduler instance.
func New(ctx context.Context) *Scheduler {
	return &Scheduler{ctx}
}

// CreatePool creates and configures a new job execution pool.
//
// Parameters:
//   - cfg: PoolConfig specifying MaxWorkers, CheckInterval, and IdleTimeout.
//   - mon: Implementation of Monitoring interface for metrics collection.
//     Defaults to internal in-memory monitoring if nil.
//
// Returns:
//   - Initialized and ready-to-use Pool instance.
func (s *Scheduler) CreatePool(cfg PoolConfig, mon Monitoring) (*Pool, error) {
	if mon == nil {
		mon = defaultMonitoring.New()
	}

	return pool.New(s.ctx, cfg, mon)
}

// AddJob creates and registers a new job in the specified scheduler pool.
//
// Validates the job's configuration and state before adding it to the pool.
//
// Parameters:
//   - pool: Target scheduler Pool to which the job will be added.
//   - cfg: JobConfig detailing the execution function, scheduling parameters, retries, and hooks.
//
// Returns:
//   - nil on successful addition.
//   - Error describing the failure reason otherwise.
func (s *Scheduler) AddJob(pool *Pool, cfg JobConfig) error {
	j, err := job.New(cfg, pool.Ctx, pool.Mon)
	if err != nil {
		return errs.New(errs.ErrAddingJob, fmt.Sprintf("err - %v", err))
	}
	return pool.AddJob(j)
}
