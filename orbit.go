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
//	s := orbit.New(context.Background())
//
//	poolCfg := orbit.PoolConfig{
//		ID:           "analytics-pool",
//		MaxWorkers:   10,
//		CheckInterval: 200 * time.Millisecond,
//	}
//
//	pool, _ := s.CreatePool(poolCfg, nil)
//
//	jobCfg := orbit.JobConfig{
//		ID: "report-job",
//		Fn: func(ctrl orbit.FnControl) error {
//			// perform some logic...
//			ctrl.SaveData(map[string]interface{}{"result": "ok"})
//			return nil
//		},
//		Interval: orbit.IntervalConfig{Time: 5 * time.Second},
//	}
//
//	s.AddJob(pool, jobCfg)
//
//  pool.Run()
//  defer pool.Kill()
//  select {}

package orbit

import (
	"context"
	"fmt"
	"orbit/internal/domain"
	errs "orbit/internal/error"
	"orbit/internal/job"
	"orbit/internal/pool"
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

// Job defines a job's configuration and execution details.
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
type Job = domain.JobDTO

// JobStatus represents the current lifecycle status of a job.
//
// Possible statuses include:
//   - Waiting
//   - Running
//   - Completed
//   - Error
//   - Ended
//   - Stopped
//   - Paused
//
// Use this type to monitor and control job state transitions.
type JobStatus = domain.JobStatus

// Interval encapsulates job scheduling settings.
//
// Parameters:
//   - Time: Time between job executions (set 0 if using cron expression).
//   - CronExpr: Cron expression defining job execution schedule (leave empty if using interval).
type Interval = domain.Interval

// Retry defines retry behavior for a job.
//
// Parameters:
//   - Count: Number of allowed retries after execution failure.
//   - Time: Time interval between retries.
//   - ResetOnSuccess: Flag to reset retries count after successful job execution.
type Retry = domain.Retry

// Hooks provides lifecycle hooks to inject custom logic at different job execution stages.
//
// Available hooks:
//   - OnStart: Executed before job starts.
//   - OnStop: Executed when job is explicitly stopped.
//   - OnError: Executed if job execution encounters an error.
//   - OnSuccess: Executed when job completes successfully.
//   - OnPause: Executed when job is paused.
//   - OnResume: Executed when job resumes after pause.
//   - Finally: Always executed after job ends (successful, error, paused, stopped).
type Hooks = domain.Hooks

// Hook defines a function that is executed during a specific lifecycle stage of a job.
//
// Each Hook consists of:
//   - Fn: the hook function to be executed.
//   - IgnoreError: whether to suppress errors returned by the hook.
//
// Used in HooksFunc to define custom actions for events like start, stop, error, success, etc.
type Hook = domain.Hook

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

// JobState represents the current runtime state of a job, including metadata such as:
//   - StartAt / EndAt timestamps
//   - Execution duration
//   - Current status (Waiting, Running, Completed, etc.)
//   - Success/failure counts
//   - Custom key-value data stored via FnControl
//   - Any error or hook-related failure that occurred
//
// This type is returned by FnControl `ctrl.GetData()`,
// and is useful for logging, monitoring, debugging, or exposing job status via API.
type JobState = domain.StateDTO

// Monitoring defines an interface for collecting, storing, and retrieving metrics related to job execution.
//
// Implementations of this interface can persist metrics in various ways, such as:
// - In-memory storage for simple debugging and development purposes.
// - Real-time logging for operational monitoring.
// - External systems like dashboards, time-series databases, or analytics platforms.
type Monitoring interface {
	// SaveMetrics stores execution metrics derived from a job's execution state (StateDTO).
	//
	// Implementations typically capture metrics like execution timestamps, duration,
	// final status, encountered errors, and user-defined runtime metadata.
	//
	// Parameters:
	//   - dto: StateDTO instance containing the job's execution details and metadata.
	SaveMetrics(dto JobState)

	// GetMetrics retrieves all stored execution metrics.
	//
	// Returns:
	//   - A map keyed by JobID, where each entry contains the StateDTO representing
	//     execution metrics for a specific job.
	GetMetrics() map[string]interface{}
}

// Orbit orchestrates the creation and management of job execution pools and scheduled jobs.
//
// Provides a simplified API for pool creation, job addition, and lifecycle management.
//
// Methods:
//   - CreatePool: Initializes a new job execution pool with specified settings.
//   - AddJob: Creates and adds a job to a specified pool.
//
// Usage:
//
//	scheduler := orbit.New(context.Background())
//	pool := scheduler.CreatePool(config, nil)
//	scheduler.AddJob(pool, jobConfig)
type Orbit struct {
	ctx context.Context // Parent context to manage scheduler lifecycle.
}

// New initializes a new Scheduler instance.
//
// Parameters:
//   - ctx: Parent execution context used for managing graceful shutdown and global cancellation.
//
// Returns:
//   - Pointer to a new Scheduler instance.
func New(ctx context.Context) *Orbit {
	return &Orbit{ctx}
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
func (s *Orbit) CreatePool(cfg PoolConfig, mon Monitoring) *Pool {
	if mon == nil {
		mon = newDefaultMon()
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
func (s *Orbit) AddJob(pool *Pool, cfg Job) error {
	j, err := job.New(cfg, pool.Ctx, pool.Mon)
	if err != nil {
		return errs.New(errs.ErrAddingJob, fmt.Sprintf("err - %v", err))
	}
	return pool.AddJob(j)
}
