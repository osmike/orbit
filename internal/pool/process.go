package pool

import (
	"errors"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// processWaiting checks if a job in the "Waiting" state is ready to execute.
//
// If the current time is past the job's scheduled next run time,
// the job is dispatched for execution.
//
// Parameters:
//   - job: The Job instance currently in the Waiting state.
//   - sem: Semaphore used to limit concurrent execution based on Pool configuration.
//   - wg: WaitGroup to synchronize the execution lifecycle of active jobs.
func (p *Pool) processWaiting(job Job, sem chan struct{}, wg *sync.WaitGroup) {
	if !job.LockJob() {
		// someone is already working with this job
		return
	}
	defer job.UnlockJob()
	meta := job.GetMetadata()
	execErr := job.CanExecute()
	if errors.Is(execErr, errs.ErrJobExecTooEarly) {
		return
	}
	// Acquire semaphore slot or timeout to avoid deadlocks.
	select {
	case sem <- struct{}{}:
		// Slot acquired successfully; proceed with execution.
	case <-time.After(5 * time.Second):
		// Failed to acquire slot within timeout; mark as error.
		job.UpdateState(domain.StateDTO{
			Status: domain.Error,
			Error:  domain.StateError{JobError: errs.New(errs.ErrTooManyJobs, meta.ID)},
		})
		return
	}
	// Determine if the job can be executed now.
	if execErr != nil {
		switch {
		case errors.Is(execErr, errs.ErrJobExecAfterEnd):
			// The job's scheduled execution window has expired; mark as ended.
			job.UpdateState(domain.StateDTO{
				Status: domain.Ended,
			})
			return
		case errors.Is(execErr, errs.ErrJobWrongStatus):
			// The job is in an invalid state for execution; record the error.
			job.UpdateState(domain.StateDTO{
				Status: domain.Error,
				Error:  domain.StateError{JobError: errs.New(execErr, meta.ID)},
			})
			return
		}
	}
	nextRun := job.NextRun()
	if time.Now().After(nextRun) || time.Now().Equal(nextRun) {
		// Mark the job as started, update metrics.
		job.ProcessStart()

		p.execute(job, sem, wg)
	} else {
		select {
		case <-sem:
		default:
		}
	}
}

// processRunning monitors a job currently in the "Running" state,
// checking for execution timeouts or runtime errors.
//
// If the job exceeds its configured timeout, it is marked as Error,
// triggering its finalization and metric recording.
//
// Parameters:
//   - job: The Job instance currently executing.
func (p *Pool) processRunning(job Job) {
	err := job.ProcessRun()
	if err != nil {
		job.ProcessEnd(domain.Error, err)
	}
}

// processCompleted handles the state of a job marked as "Completed".
//
// It checks if the job has future scheduled executions. If another execution
// is pending, the job state is reset to "Waiting". Otherwise, the job is marked
// as "Ended", indicating no further executions are planned.
//
// Parameters:
//   - job: The Job instance that has completed its execution.
func (p *Pool) processCompleted(job Job) {
	nextRun := job.NextRun()

	if nextRun.After(time.Now()) || time.Now().Equal(nextRun) {
		job.UpdateState(domain.StateDTO{
			Status: domain.Waiting,
		})
		return
	}
	job.ProcessEnd(domain.Ended, nil)
}

// processError manages jobs that have encountered an error during execution.
//
// It attempts to retry the job execution if retries are still allowed. If the retry
// limit is reached, no further action is taken. Otherwise, the job state is reset
// to "Waiting", scheduling it for the next execution attempt.
//
// Parameters:
//   - job: The Job instance currently in an Error state.
func (p *Pool) processError(job Job) error {
	return job.ProcessError()
}
