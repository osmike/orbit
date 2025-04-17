package job

import (
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"time"
)

// CanExecute evaluates whether the job is eligible to start execution based on current state and configured constraints.
//
// This method performs the following validation checks:
//   - Ensures the job is currently in the Waiting state and not already running.
//   - Verifies that the current time has reached or passed the scheduled StartAt time.
//   - Ensures the current time has not exceeded the job's configured EndAt deadline.
//
// Returns:
//   - nil if the job can proceed with execution.
//   - An appropriate error (ErrJobWrongStatus, ErrJobExecTooEarly, ErrJobExecAfterEnd) if execution conditions are not met.
func (j *Job) CanExecute() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.GetStatus() != domain.Waiting {
		return errs.New(errs.ErrJobWrongStatus, j.ID)
	}

	if time.Now().Before(j.StartAt) {
		return errs.New(errs.ErrJobExecTooEarly, j.ID)
	}

	if time.Now().After(j.EndAt) {
		return errs.New(errs.ErrJobExecAfterEnd, j.ID)
	}

	return nil
}

// NextRun computes the next scheduled execution time for the job based on its scheduling configuration.
//
// Scheduling modes supported:
//   - Cron-based: calculates the next execution time based on a cron expression.
//   - Time-based: calculates the next execution time based on a fixed interval from the last execution.
//
// Returns:
//   - The next scheduled execution time as a time.Time instance.
//   - If the calculated next execution is in the past, it returns the current time to trigger immediate execution.
func (j *Job) NextRun() time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.cron != nil {
		return j.cron.NextRun()
	}

	next := j.state.StartAt.Add(j.Interval.Time)

	return next
}

// Retry increments the job's retry counter and verifies whether the retry limit has been exceeded.
//
// It must be called after a job fails execution to determine whether another execution attempt should occur.
//
// Returns:
//   - ErrJobRetryLimit if the maximum retry limit is reached, indicating no further retries should occur.
//   - nil if the job is allowed another execution attempt.
func (j *Job) Retry() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if !j.JobDTO.Retry.Active {
		return errs.New(errs.ErrRetryFlagNotActive, j.ID)
	}

	if j.JobDTO.Retry.Count == 0 {
		return nil
	}

	if j.state.currentRetry >= j.JobDTO.Retry.Count {
		return errs.New(errs.ErrJobRetryLimit, j.ID)
	}
	j.state.currentRetry++
	return nil
}

func (j *Job) CloseChannels() {
	defer func() { // closing channels with recover
		recover()
		j.cancel()
	}()
	close(j.processCh)
	close(j.doneCh)
	close(j.resumeCh)
	close(j.pauseCh)
}
