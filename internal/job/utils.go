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

// NextRun determines the job's next scheduled execution time.
//
// Logic:
//   - If the job uses cron syntax, calculates next run from the cron schedule.
//   - If using a fixed interval, adds interval to the last StartAt time.
//
// Returns:
//   - The calculated time of next execution attempt.
func (j *Job) NextRun() time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.cron != nil {
		return j.cron.NextRun()
	}

	return j.state.StartAt.Add(j.Interval.Time)
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

// CloseChannels terminates all internal job channels safely and cancels the job context.
//
// This method is used during graceful shutdown or finalization. It recovers from
// any panics caused by closing already-closed channels.
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
