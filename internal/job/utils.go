package job

import (
	"orbit/internal/domain"
	errs "orbit/internal/error"
	"time"
)

// CanExecute evaluates whether the job is eligible for execution based on scheduling constraints and current status.
//
// Validation checks include:
//   - The job must be in the Waiting state.
//   - The current time must be after StartAt and before EndAt.
//
// Returns:
//   - nil if the job is eligible for execution.
//   - A wrapped error indicating the reason why execution is not allowed.
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

// SetNextRun determines and schedules the next job execution time.
//
// Behavior:
//   - If the job uses a cron schedule, fetches the next run time and advances the internal cron iterator.
//   - If using a fixed interval, adds Interval.Time to the provided startTime.
//
// Parameters:
//   - startTime: The baseline time to calculate the next run from.
//
// Returns:
//   - The calculated next execution time.

func (j *Job) SetNextRun(startTime time.Time) time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.cron != nil {
		res := j.cron.NextRun()
		return res
	}

	return startTime.Add(j.Interval.Time)
}

// GetNextRun retrieves the scheduled time for the job's next execution.
//
// Returns:
//   - time.Time: The timestamp of the next planned run.
func (j *Job) GetNextRun() time.Time {
	return j.state.GetNextRun()
}

// Retry increments the job's retry counter and evaluates retry eligibility.
//
// Logic:
//   - If retrying is disabled, returns ErrRetryFlagNotActive.
//   - If max retry count is reached, returns ErrJobRetryLimit.
//   - If unlimited retries (Count = 0), allows retry indefinitely.
//
// Returns:
//   - nil if retry is permitted.
//   - Error if retrying is disallowed or exhausted.
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

// CloseChannels safely closes all internal job channels and cancels the job context.
//
// Behavior:
//   - Attempts to close all coordination channels (pause, resume, done, process).
//   - Recovers from any panics due to closing already-closed channels.
//   - Cancels the associated job context.
//
// Warning:
//   - processCh is mistakenly attempted to be closed twice — calling CloseChannels more than once is unsafe and should be avoided.
func (j *Job) CloseChannels() {
	defer func() {
		recover()
		j.cancel()
	}()
	close(j.processCh)
	close(j.doneCh)
	close(j.resumeCh)
	close(j.pauseCh)
}
