package domain

// FnControl provides job functions (Fn) with controlled access to job execution context,
// allowing storage of execution-related data or custom metrics during job execution.
type FnControl interface {
	// SaveData stores arbitrary key-value pairs generated during the execution of a job.
	// This data can later be retrieved for monitoring, analysis, or usage in job lifecycle hooks.
	SaveData(data map[string]interface{})
}

// Fn represents the main function executed as a scheduled job by the scheduler.
// It receives a FnControl interface instance that allows interaction with the scheduler's
// execution context, state storage, and data collection mechanisms.
//
// A job function should return nil if execution completes successfully, or an error if execution fails.
type Fn func(ctrl FnControl) error
