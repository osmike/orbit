package domain

import "time"

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

const (
	DEFAULT_NUM_WORKERS    = 1000
	DEFAULT_CHECK_INTERVAL = 100 * time.Millisecond
	DEFAULT_IDLE_TIMEOUT   = 100 * time.Hour
)

var (
	MAX_END_AT = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
)
