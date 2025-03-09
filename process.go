package scheduler

import (
	"time"
)

// processRunning updates the execution time of the running job and checks for timeout conditions.
// If the job exceeds its allowed execution time, it is canceled and marked as an error.
//
// Steps performed:
// 1. Updates the execution time since the job started.
// 2. Resets any previous error state.
// 3. Checks if execution time has exceeded the configured timeout.
//   - If exceeded, the job is forcefully canceled and marked as an error.
//
// This function is called periodically while the job is in the Running state.
func (j *Job) processRunning() {
	// Update execution time based on the time elapsed since the job started.
	j.State.ExecutionTime = time.Since(j.State.StartAt).Nanoseconds()
	j.State.Error = nil

	// If the job exceeds its timeout, cancel it and set an error status.
	if time.Duration(j.State.ExecutionTime) > j.Timeout {
		j.cancel() // Stop job execution.
		j.setStatus(Error)
		j.State.Error = newErr(ErrJobTimout, j.ID)
		return
	}
}

// processCompleted finalizes the job by setting its end time and preparing it for the next execution cycle.
//
// Steps performed:
// 1. Updates the job's end time.
// 2. Transitions the job back to the Waiting state if it is scheduled to run again.
//
// This function is called when a job successfully completes execution.
func (j *Job) processCompleted() {
	// Record the end time of the job execution.
	j.State.EndAt = time.Now()

	// Move the job back to Waiting if it is configured to run periodically.
	j.tryChangeStatus([]JobStatus{Completed}, Waiting)
}

// processError handles job failures by either retrying the job or permanently marking it as failed.
//
// Steps performed:
// 1. Updates the job's end time.
// 2. If retries are enabled and available, moves the job back to the Waiting state for re-execution.
// 3. Decrements the retry counter.
//
// This function is called when a job encounters an execution error.
func (j *Job) processError() {
	// Record the end time of the job execution.
	j.State.EndAt = time.Now()

	// If retries are enabled and there are retries remaining, reattempt execution.
	if j.Retry.Active && j.State.currentRetry > 0 {
		j.setStatus(Waiting)
		j.State.currentRetry--
	}
}

func (j *Job) processStop() {
	j.State.EndAt = time.Now()
	j.cancel()
}
