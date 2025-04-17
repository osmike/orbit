package domain

import (
	"time"
)

// StateDTO is a data transfer object representing the runtime state of a job.
//
// It encapsulates execution metadata, status flags, timing information, user-defined data,
// and error traces without exposing internal synchronization or logic.
type StateDTO struct {
	// JobID is the unique identifier of the job to which this state belongs.
	JobID string

	// StartAt is the timestamp marking when the job execution began.
	// It is zero if the job has not started yet.
	StartAt time.Time

	// EndAt is the timestamp marking when the job finished execution.
	// It is zero if the job is currently running or has not yet started.
	EndAt time.Time

	// Error captures errors encountered during job execution or in lifecycle hooks.
	// If no errors occurred, this field remains empty.
	Error StateError

	// Status indicates the current execution state of the job (e.g., Waiting, Running, Completed, Error).
	Status JobStatus

	// ExecutionTime is the duration in nanoseconds that the job took to complete.
	// It is zero if the job hasn't completed or hasn't started yet.
	ExecutionTime int64

	// Data holds user-defined key-value pairs collected during job execution.
	// This is commonly used for computed results, metrics, or side-channel information.
	Data map[string]interface{}

	// Success tracks how many times the job has completed successfully.
	Success int

	// Failure tracks how many times the job has failed.
	Failure int

	// NextRun indicates the estimated time of the next scheduled execution.
	// For one-time jobs, this will remain zero after execution.
	NextRun time.Time
}

// StateError groups together execution errors encountered during job runtime or in hook callbacks.
//
// It distinguishes between core job execution errors and hook-specific failures.
type StateError struct {
	// JobError is the error returned by the main job function (Fn), if any.
	JobError error

	// HookError captures individual errors raised by lifecycle hooks.
	HookError
}

// IsEmpty checks whether the error state is effectively clean, i.e., no job or hook errors occurred.
//
// Returns:
//   - true if both JobError and all HookError fields are nil; false otherwise.
func (e StateError) IsEmpty() bool {
	return e.JobError == nil &&
		e.HookError.OnStart == nil &&
		e.HookError.OnStop == nil &&
		e.HookError.OnError == nil &&
		e.HookError.OnSuccess == nil &&
		e.HookError.OnPause == nil &&
		e.HookError.OnResume == nil &&
		e.HookError.Finally == nil
}

// JobDTO defines the configuration and execution parameters of a scheduled job.
//
// It serves as the main input structure used to register jobs with the scheduler.
type JobDTO struct {
	// ID is a required unique identifier for the job.
	ID string

	// Name is an optional human-readable label for the job.
	// If omitted, it defaults to the value of ID.
	Name string

	// Fn is the function that encapsulates the job's execution logic.
	// It receives a FnControl object that provides control over execution flow and data storage.
	Fn Fn

	// Interval specifies the time delay between consecutive executions.
	// If set to 0, the job runs only once.
	Interval Interval

	// Timeout defines the maximum duration allowed for a single execution of the job.
	// If exceeded, the job will be terminated.
	Timeout time.Duration

	// StartAt defines the earliest point in time the job is allowed to run.
	// Jobs scheduled before this timestamp will remain in Waiting state.
	StartAt time.Time

	// EndAt defines the latest point in time the job is allowed to run.
	// Jobs scheduled after this timestamp will be considered expired and marked as Stopped.
	EndAt time.Time

	// Retry specifies retry behavior in case of job execution failure.
	Retry Retry

	// Hooks defines callback functions triggered at key lifecycle events
	// such as OnStart, OnSuccess, OnError, OnStop, etc.
	Hooks Hooks
}
