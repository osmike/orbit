package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type JobStatus string

const (
	// Waiting indicates the job is scheduled and ready to run.
	// The scheduler will pick up the job when the execution conditions are met.
	// If the job has an interval, it will wait for the next scheduled run.
	Waiting JobStatus = "waiting"

	// Running indicates the job is currently executing.
	// The scheduler actively processes the job and updates its execution time.
	// If the job exceeds the timeout, it will be marked as an error and terminated.
	Running JobStatus = "running"

	// Stopped indicates the job execution has been manually stopped.
	// The job will not run again unless explicitly restarted.
	// This status is usually set when the job's execution window has expired.
	Stopped JobStatus = "stopped"

	// Paused indicates the job is temporarily paused and can be resumed later.
	// While paused, the job retains its execution state but does not progress.
	// The scheduler does not execute paused jobs until they are resumed.
	Paused JobStatus = "paused"

	// Completed indicates the job has finished execution successfully.
	// If the job has an interval, it will be scheduled to run again after the delay.
	// Otherwise, it remains in a completed state indefinitely.
	Completed JobStatus = "completed"

	// Error indicates the job has encountered an error during execution.
	// The job may retry execution if retries are enabled and attempts to remain.
	// If retries are exhausted, the job remains in the error state.
	Error JobStatus = "error"
)

// Job represents a scheduled task that can be executed by the scheduler.
type Job struct {
	// ID is a unique identifier for the job.
	ID string

	// Name is a human-readable name for the job.
	// If not provided, it defaults to the job's ID.
	Name string

	// Fn is the function that will be executed when the job runs.
	// It receives a FnControl instance, allowing the function to manage execution.
	Fn Fn

	// Interval defines the time duration between consecutive executions of the job.
	// If set to 0, the job will run only once.
	Schedule Schedule

	// Timeout is the maximum allowed execution time for the job.
	// If the job exceeds this duration, it will be forcibly stopped.
	Timeout time.Duration

	// StartAt is the earliest time at which the job is allowed to run.
	// If the current time is before this value, the job will remain in the Waiting state.
	StartAt time.Time

	// EndAt is the latest time at which the job is allowed to run.
	// If the current time is after this value, the job will be marked as Stopped.
	EndAt time.Time

	// Retry holds the retry settings for the job in case of execution failure.
	Retry Retry

	// State contains the runtime information of the job, such as execution time and status.
	State State

	Hooks Hooks

	// Ctx is the execution context of the job, allowing cancellation and timeout control.
	ctx context.Context

	// Cancel is the function used to cancel the job's execution.
	cancel context.CancelFunc

	// Mu is a mutex used to synchronize access to the job's state.
	mu sync.Mutex

	// pauseCh is a channel used to pause the execution of the job.
	// When a signal is received, the job should enter the Paused state.
	pauseCh chan struct{}

	// resumeCh is a channel used to resume execution of a paused job.
	// When a signal is received, the job should transition back to Running.
	resumeCh chan struct{}

	cron *CronSchedule
}

// Retry defines the retry policy for a job execution.
type Retry struct {
	// Active specifies whether retrying is enabled for the job.
	Active bool

	// Count is the number of allowed retry attempts before marking the job as failed.
	// Each time a job fails, this counter decreases. When it reaches zero, the job stops retrying.
	Count int64

	// ResetOnSuccess determines whether the retry counter should be reset after a successful execution.
	// If true, a successful execution resets the retry count to its initial value.
	ResetOnSuccess bool
}

// State holds execution-related metadata for a job.
type State struct {
	// JobID is the unique identifier of the job, copied from Job.ID.
	JobID string

	// StartAt records the timestamp of the most recent execution start.
	StartAt time.Time

	// EndAt records the timestamp of the most recent execution completion.
	EndAt time.Time

	// Error holds the last error encountered during execution, if any.
	Error error

	// currentRetry tracks the number of retry attempts left.
	currentRetry int64

	// Status represents the current execution state of the job.
	// It is updated atomically to prevent race conditions.
	Status atomic.Value

	// ExecutionTime stores the duration of the last execution in nanoseconds.
	ExecutionTime int64

	// Data is a map that stores job-specific metadata or user-defined information.
	// This allows jobs to persist and retrieve contextual information across executions.
	Data sync.Map
}

type Schedule struct {
	Interval time.Duration
	CronExpr string
}

type JobMetadata struct {
	ID      string
	Name    string
	StartAt time.Time
	EndAt   time.Time
	State   State
}
