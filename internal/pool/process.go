package pool

import (
	"errors"
	"github.com/osmike/orbit/internal/domain"
	errs "github.com/osmike/orbit/internal/error"
	"sync"
	"time"
)

// ProcessWaiting evaluates whether a job in the "Waiting" state is ready for execution,
// and dispatches it if all conditions are met.
//
// This method is intended to be called periodically by the pool scheduler to
// identify and start due jobs. It ensures exclusive job processing via locking
// and limits concurrency using a semaphore with timeout protection.
//
// Behavior:
//   - Acquires an internal lock to ensure exclusive handling of the job.
//   - Checks whether the current time is equal to or after the job's NextRun time.
//   - Evaluates the job's eligibility for execution using CanExecute().
//   - If execution is not yet due or is otherwise invalid, the job is skipped.
//   - Attempts to acquire a semaphore slot within a timeout window (default: 5s).
//   - If slot acquisition fails, marks the job as errored (ErrTooManyJobs).
//   - If job's CanExecute returns execution-blocking errors (e.g., expired, bad status):
//   - Marks job as Ended or Error accordingly.
//   - If eligible and a slot is available, schedules the job for execution via Execute().
//
// Parameters:
//   - job: The job instance currently in the "Waiting" state.
//   - sem: A buffered channel used as a semaphore to limit concurrent execution.
//   - wg: A WaitGroup used to track active job executions and support graceful shutdown.
func (p *Pool) ProcessWaiting(job domain.Job, sem chan struct{}, wg *sync.WaitGroup) {
	if !job.LockJob() {
		// someone is already working with this job
		return
	}
	defer job.UnlockJob()
	meta := job.GetMetadata()
	execErr := job.CanExecute()
	nextRun := job.GetNextRun()
	if !(time.Now().After(nextRun) || time.Now().Equal(nextRun)) {
		return
	}
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

	p.Execute(job, sem, wg)
}

// ProcessRunning monitors a job currently in the "Running" state,
// checking for execution timeouts or runtime errors.
//
// If the job exceeds its configured timeout, it is marked as Error,
// triggering its finalization and metric recording.
//
// Parameters:
//   - job: The Job instance currently executing.
func (p *Pool) ProcessRunning(job domain.Job) {
	err := job.ProcessRun()
	if err != nil {
		job.ProcessEnd(domain.Error, err)
	}
}

// ProcessCompleted handles the state of a job marked as "Completed".
//
// It checks if the job has future scheduled executions. If another execution
// is pending, the job state is reset to "Waiting". Otherwise, the job is marked
// as "Ended", indicating no further executions are planned.
//
// Parameters:
//   - job: The Job instance that has completed its execution.
func (p *Pool) ProcessCompleted(job domain.Job) {
	nextRun := job.GetNextRun()

	if nextRun.After(time.Now()) {
		job.UpdateState(domain.StateDTO{
			Status: domain.Waiting,
		})
		return
	}
	job.ProcessEnd(domain.Ended, nil)
}

// ProcessError manages jobs that have encountered an error during execution.
//
// Behavior:
//   - Attempts to retry the job execution if retries are still allowed.
//   - If retries are exhausted, the job is removed from the pool.
//   - If retry is allowed, the job is reset to "Waiting" for another execution attempt.
//
// Parameters:
//   - job: The Job instance currently in an Error state.
//
// Returns:
//   - An error if the job could not be removed from the pool.
//   - nil otherwise.
func (p *Pool) ProcessError(job domain.Job) error {
	if !job.LockJob() {
		// someone is already working with this job
		return nil
	}
	defer job.UnlockJob()
	err := job.ProcessError()
	if err != nil {
		md := job.GetMetadata()

		err = p.RemoveJob(md.ID)
		return err
	}
	return nil
}
