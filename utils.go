package scheduler

import "time"

// setStatus updates the job's current status atomically.
//
// This function should be used to transition a job between states safely.
// It ensures thread-safe modification of the job status.
//
// Parameters:
// - status: The new status to assign to the job.
func (j *Job) setStatus(status JobStatus) {
	j.State.Status.Store(status)
}

// getStatus retrieves the current status of the job atomically.
//
// If the status has not been set before, it defaults to Waiting.
//
// Returns:
// - The current job status.
func (j *Job) getStatus() JobStatus {
	val := j.State.Status.Load()
	if val == nil {
		return Waiting
	}
	return val.(JobStatus)
}

// tryChangeStatus attempts to change the job's status atomically only if it matches one of the allowed states.
//
// This function is useful to ensure that a job transitions only through valid state changes.
// It prevents race conditions where multiple goroutines might attempt to update the status simultaneously.
//
// Parameters:
// - allowed: A slice of valid job statuses that the job can transition from.
// - newStatus: The desired new status for the job.
//
// Returns:
// - true if the status was successfully changed, false otherwise.
func (j *Job) tryChangeStatus(allowed []JobStatus, newStatus JobStatus) bool {
	current := j.getStatus()
	for _, status := range allowed {
		if current == status {
			return j.State.Status.CompareAndSwap(current, newStatus)
		}
	}
	return false
}

// getDelay calculates the remaining time before the next execution of the job.
//
// If the execution time of the previous run exceeded the interval, it returns 0 to avoid negative delays.
//
// Returns:
// - The duration to wait before the next execution.
func (j *Job) getDelay() time.Duration {
	delay := j.Interval - time.Duration(j.State.ExecutionTime)
	if delay < 0 {
		delay = 0
	}
	return delay
}
