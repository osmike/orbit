package domain

import "context"

// FnControl provides controlled access to a job's execution environment,
// including lifecycle control, runtime metadata storage, and scheduler interaction.
//
// This interface is passed into each job's execution function (Fn), enabling the job to:
//
//   - Save and share state or metrics via SaveData.
//   - React to cancellation or timeouts via Context.
//   - Support manual pausing and resuming through PauseChan and ResumeChan.
type FnControl interface {
	// SaveData stores arbitrary key-value pairs generated during the execution of a job.
	// This data can later be accessed for monitoring, logging, or analysis.
	SaveData(data map[string]interface{})

	// Context returns the job's execution context.
	// It is canceled when the job is stopped, times out, or the pool is shut down.
	Context() context.Context

	// PauseChan returns a channel that is used to signal when the job should pause.
	// The job implementation is expected to read from this channel and enter a paused state when needed.
	PauseChan() chan struct{}

	// ResumeChan returns a channel that signals when a paused job should resume execution.
	// The job should wait on this channel to continue processing after a pause.
	ResumeChan() chan struct{}
}

// Fn represents the main function executed as a scheduled job by the scheduler.
//
// The function receives a FnControl instance that exposes mechanisms for:
//   - Saving execution-related data (for monitoring or hooks).
//   - Accessing cancellation context and pause/resume signals.
//
// The function should return nil if execution completes successfully,
// or an error if execution fails.
type Fn func(ctrl FnControl) error
