package error

import (
	"errors"
	"fmt"
)

// ErrEmptyID indicates that a job or pool was created or requested without providing a valid ID.
var ErrEmptyID = errors.New("empty ID")

// General errors related to job creation and validation.
var (
	// ErrIDExists occurs when attempting to create a job with an ID that already exists.
	ErrIDExists = errors.New("job ID not unique")

	// ErrEmptyFunction indicates that the job was defined without an associated function to execute.
	ErrEmptyFunction = errors.New("function is empty")

	// ErrWrongTime occurs when provided job start or end times are logically incorrect (e.g., end before start).
	ErrWrongTime = errors.New("wrong time")

	// ErrMixedScheduleType is returned when both interval and cron scheduling are specified simultaneously.
	ErrMixedScheduleType = errors.New("job schedule is only supported with one type of interval")

	// ErrAddingJob indicates a generic failure during the addition of a new job to the scheduler.
	ErrAddingJob = errors.New("error adding job")

	// ErrJobNotFound is returned when attempting to reference or manipulate a job that does not exist.
	ErrJobNotFound = errors.New("job not found")
)

// Pool and concurrency-related errors.
var (
	// ErrTooManyJobs occurs when the scheduler reaches its maximum allowed number of concurrent jobs.
	ErrTooManyJobs  = errors.New("too many jobs")
	ErrPoolShutdown = errors.New("pool shutdown")
)

// Scheduling errors.
var (
	// ErrInvalidCronExpression indicates the provided cron scheduling expression is invalid or malformed.
	ErrInvalidCronExpression = errors.New("invalid cron expression")
)

// Job execution and runtime errors.
var (
	// ErrJobPanicked indicates that a job execution encountered a panic condition.
	ErrJobPanicked = errors.New("job panicked")

	// ErrJobTimout indicates that a job exceeded its specified execution timeout.
	ErrJobTimout = errors.New("job timed out")

	// ErrJobExecution represents a general execution failure within a job's main function.
	ErrJobExecution = errors.New("error in job execution")

	// ErrJobWrongStatus occurs when a job is in an inappropriate state for the requested operation.
	ErrJobWrongStatus = errors.New("job with wrong status")

	// ErrJobExecTooEarly occurs when the scheduler attempts to run a job before its scheduled start time.
	ErrJobExecTooEarly = errors.New("job execution too early")

	// ErrJobExecAfterEnd occurs when the scheduler attempts to execute a job after its designated end time.
	ErrJobExecAfterEnd = errors.New("job execution after end time")

	// ErrJobPaused indicates that the operation attempted to pause an already paused job.
	ErrJobPaused = errors.New("job is already paused")

	// ErrJobStillRunning indicates a request to execute a job that is already running.
	ErrJobStillRunning = errors.New("job is still running")

	// ErrJobRetryLimit is returned when a job exceeds its configured retry attempts after execution failures.
	ErrJobRetryLimit = errors.New("job retry limit reached")

	// ErrJobNotRunning occurs when attempting to stop or pause a job that is not currently running.
	ErrJobNotRunning = errors.New("job is not running")

	// ErrJobNotPausedOrStopped occurs when an operation intended for a paused or stopped job is attempted on an active job.
	ErrJobNotPausedOrStopped = errors.New("job is not paused or stopped")

	ErrRetryFlagNotActive = errors.New("retry flag is not active")
)

// Lifecycle hook errors.
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

	ErrOnErrorHook = errors.New("error in on error hook")

	ErrOnStopHook = errors.New("error in on stop hook")
)

// New wraps a given error with additional contextual information.
// Useful for providing details such as Job IDs or execution specifics alongside base error messages.
func New(err error, str string) error {
	return fmt.Errorf("%w: %s", err, str)
}
