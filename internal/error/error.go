package error

import (
	"errors"
	"fmt"
)

// ErrEmptyID is returned when a job or pool is created or referenced without a valid non-empty ID.
var ErrEmptyID = errors.New("empty ID")

// Job definition and configuration errors.
var (
	// ErrIDExists indicates that a job with the same ID already exists in the scheduler.
	ErrIDExists = errors.New("job ID not unique")

	// ErrEmptyFunction occurs when a job is defined without an executable function.
	ErrEmptyFunction = errors.New("function is empty")

	// ErrWrongTime is returned when job start or end times are chronologically incorrect (e.g., end before start).
	ErrWrongTime = errors.New("wrong time")

	// ErrMixedScheduleType occurs when both interval and cron expressions are specified simultaneously.
	ErrMixedScheduleType = errors.New("job schedule is only supported with one type of interval")

	// ErrAddingJob represents a general error while adding a job to the pool.
	ErrAddingJob = errors.New("error adding job")

	// ErrJobNotFound is returned when a requested job cannot be found in the pool.
	ErrJobNotFound = errors.New("job not found")
)

// Pool and concurrency control errors.
var (
	// ErrTooManyJobs indicates that the maximum number of concurrent jobs has been reached.
	ErrTooManyJobs = errors.New("too many jobs")

	// ErrPoolShutdown is returned when an operation is attempted on a pool that has already been shut down.
	ErrPoolShutdown = errors.New("pool shutdown")

	// ErrJobFactoryIsNil is returned when job factory is not provided
	ErrJobFactoryIsNil = errors.New("job factory is nil")
)

// Scheduling-related validation errors.
var (
	// ErrInvalidCronExpression occurs when a provided cron expression is invalid or cannot be parsed.
	ErrInvalidCronExpression = errors.New("invalid cron expression")
)

// Runtime job execution and control errors.
var (
	// ErrJobPanicked indicates that the job encountered a panic during execution.
	ErrJobPanicked = errors.New("job panicked")

	// ErrJobTimout is returned when a job exceeds its configured timeout duration.
	ErrJobTimout = errors.New("job timed out")

	// ErrJobExecution represents a generic failure during job execution.
	ErrJobExecution = errors.New("error in job execution")

	// ErrJobWrongStatus indicates that the job is in an invalid state for the requested operation.
	ErrJobWrongStatus = errors.New("job with wrong status")

	// ErrJobExecTooEarly occurs when a job is triggered before its scheduled start time.
	ErrJobExecTooEarly = errors.New("job execution too early")

	// ErrJobExecAfterEnd occurs when a job is triggered after its configured end time.
	ErrJobExecAfterEnd = errors.New("job execution after end time")

	// ErrJobPaused indicates an attempt to pause a job that is already paused.
	ErrJobPaused = errors.New("job is already paused")

	// ErrJobStillRunning is returned when trying to start a job that is currently executing.
	ErrJobStillRunning = errors.New("job is still running")

	// ErrJobRetryLimit is returned when a job exceeds the configured retry limit.
	ErrJobRetryLimit = errors.New("job retry limit reached")

	// ErrJobNotRunning occurs when trying to stop or pause a job that is not currently running.
	ErrJobNotRunning = errors.New("job is not running")

	// ErrJobNotPausedOrStopped is returned when attempting to resume a job that is not paused or stopped.
	ErrJobNotPausedOrStopped = errors.New("job is not paused or stopped")

	// ErrRetryFlagNotActive indicates that retry is disabled for this job.
	ErrRetryFlagNotActive = errors.New("retry flag is not active")

	ErrMarshalStateData = errors.New("error marshaling state data")

	ErrUnmarshalStateData = errors.New("error unmarshaling state data")
)

// Lifecycle hook execution errors.
var (
	// ErrFinallyHook indicates an error occurred during execution of the Finally hook.
	ErrFinallyHook = errors.New("error in finally hook")

	// ErrOnStartHook indicates an error occurred during execution of the OnStart hook.
	ErrOnStartHook = errors.New("error in onstart hook")

	// ErrOnSuccessHook indicates an error occurred during execution of the OnSuccess hook.
	ErrOnSuccessHook = errors.New("error in onsuccess hook")

	// ErrOnPauseHook indicates an error occurred during execution of the OnPause hook.
	ErrOnPauseHook = errors.New("error in onpause hook")

	// ErrOnResumeHook indicates an error occurred during execution of the OnResume hook.
	ErrOnResumeHook = errors.New("error in resume hook")

	// ErrOnErrorHook indicates an error occurred during execution of the OnError hook.
	ErrOnErrorHook = errors.New("error in on error hook")

	// ErrOnStopHook indicates an error occurred during execution of the OnStop hook.
	ErrOnStopHook = errors.New("error in on stop hook")
)

// New wraps the provided base error with additional context.
// The resulting error will retain the original for use with errors.Is and errors.Unwrap.
//
// Parameters:
//   - err: Base error to be wrapped.
//   - str: Additional context message.
//
// Returns:
//   - A new error that wraps the original with additional context.
func New(err error, str string) error {
	return fmt.Errorf("%w: %s", err, str)
}
