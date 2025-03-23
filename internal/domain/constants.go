package domain

import "time"

// JobStatus represents the current execution state of a job within the scheduler.
//
// It is used to track and manage the lifecycle and transitions of scheduled tasks.
// Possible values include:
// - Waiting:   The job is waiting to be executed.
// - Running:   The job is currently executing.
// - Completed: The job has finished execution successfully.
// - Paused:    The job is temporarily paused.
// - Stopped:   The job has been explicitly stopped and will not run unless restarted.
// - Ended:     The job reached its defined end condition (e.g., end time or retry limit).
// - Error:     The job encountered an error during execution.
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

	// Ended indicates the job has finished execution.
	// This status is usually set when the job's execution window has expired.
	// The job will not run again unless explicitly restarted.
	Ended JobStatus = "ended"
)

const (
	// DEFAULT_NUM_WORKERS specifies the default number of concurrent workers
	// that the scheduler will use to execute jobs concurrently when no explicit limit is provided.
	// Recommended to adjust according to the workload and system resources.
	DEFAULT_NUM_WORKERS = 1000
	// DEFAULT_CHECK_INTERVAL sets the default time interval between consecutive job state checks in the scheduler.
	// The scheduler evaluates job statuses and conditions every interval defined by this constant.
	// A shorter interval provides faster reaction time but can increase CPU usage.
	DEFAULT_CHECK_INTERVAL = 100 * time.Millisecond
	// DEFAULT_IDLE_TIMEOUT specifies the default duration after which an idle job (not running or scheduled to run soon)
	// will be considered inactive or ready for removal.
	// This timeout prevents resource leaks by removing or disabling long-idle jobs.
	DEFAULT_IDLE_TIMEOUT = 100 * time.Hour
	// DEFAULT_PAUSE_TIMEOUT is the default timeout applied when pausing a job without an explicitly provided duration.
	// It determines how long a job will remain paused before automatically resuming execution.
	DEFAULT_PAUSE_TIMEOUT = 1 * time.Second
)

var (
	// MAX_END_AT represents the maximum allowable date-time for job scheduling.
	// It serves as a predefined maximum execution boundary, allowing tasks to run indefinitely
	// unless explicitly set otherwise. Used to avoid zero-value date confusion or invalid scheduling intervals.
	MAX_END_AT = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
)
