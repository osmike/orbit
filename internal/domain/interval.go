package domain

import "time"

// Interval defines how frequently or at what specific times a job should be executed.
// It supports two scheduling methods: interval-based scheduling and cron-based scheduling.
// Only one scheduling method should be used per job configuration to avoid conflicts.
type Interval struct {
	// Time specifies the duration between consecutive executions of the job.
	// For example, an interval of 1 hour executes the job every hour.
	// If Time is set, CronExpr must be empty.
	Time time.Duration

	// CronExpr specifies a cron expression that defines precise scheduling times (e.g., "0 0 * * *" for daily execution at midnight).
	// Supports standard cron syntax with minute, hour, day-of-month, month, and day-of-week fields.
	// If CronExpr is set, Time must be zero.
	CronExpr string
}
