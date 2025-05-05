package domain

// Retry defines the retry policy for a job execution.
type Retry struct {
	// Active specifies whether retrying is enabled for the job.
	Active bool

	// Count is the number of allowed retry attempts before marking the job as failed.
	// Each time a job fails, this counter decreases. When it reaches zero, the job stops retrying.
	Count int

	// ResetOnSuccess determines whether the retry counter should be reset after a successful execution.
	// If true, a successful execution resets the retry count to its initial value.
	ResetOnSuccess bool
}
