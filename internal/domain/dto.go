package domain

import (
	"time"
)

// StateDTO is a lightweight representation of State used for data transfer.
// It provides a thread-safe way to expose job execution state without direct access to the internal structure.
type StateDTO struct {
	// JobID is a unique identifier for the job whose state is represented.
	JobID string

	// StartAt indicates the timestamp when the job started execution.
	// It is zero if the job hasn't started yet.
	StartAt time.Time

	// EndAt marks the timestamp when the job finished execution.
	// It remains zero if the job is currently running or has not yet started.
	EndAt time.Time

	// Error holds information about any errors encountered during job execution.
	// It is nil if no errors occurred.
	Error error

	// Status represents the current state of the job execution (e.g., Waiting, Running, Completed, Error).
	Status JobStatus

	// ExecutionTime records the duration (in nanoseconds) that the job took to complete.
	// It is zero if the job hasn't finished executing or hasn't started yet.
	ExecutionTime int64

	// Data stores arbitrary key-value pairs associated with the job state.
	// It is commonly used for custom metrics or runtime data collected during execution.
	Data map[string]interface{}
}

// JobDTO represents a scheduled task that can be executed by the scheduler.
type JobDTO struct {
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

	// Hooks contains callback functions triggered at various stages of the job lifecycle.
	Hooks Hooks
}
