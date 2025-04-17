package domain

import "time"

// Pool represents configuration settings for managing the execution environment of scheduled jobs.
// It controls worker concurrency, resource usage, and job-checking frequency.
type Pool struct {
	ID string
	// IdleTimeout specifies the duration after which a job that remains idle
	// (not executed or scheduled for immediate execution) will be marked as inactive.
	// This helps optimize resource usage and prevents accumulation of stale tasks.
	IdleTimeout time.Duration

	// MaxWorkers sets the maximum number of concurrent workers allowed to execute jobs simultaneously.
	// Higher values can improve throughput for CPU-bound or I/O-bound tasks, but might consume more system resources.
	MaxWorkers int

	// CheckInterval defines how frequently the pool checks for jobs that are ready for execution or require status updates.
	// Short intervals result in more responsive job execution at the expense of slightly increased CPU utilization.
	CheckInterval time.Duration
}
