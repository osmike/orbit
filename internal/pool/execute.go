package pool

import (
	"errors"
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
)

// execute runs a specified job asynchronously, respecting scheduling constraints and concurrency limits.
//
// The execution workflow includes:
//  1. Checking job eligibility via CanExecute():
//     - If the job is scheduled for a future time, execution is skipped.
//     - If the job's scheduled execution period has passed, it is marked as Ended.
//     - If the job is in an invalid state, it is marked as Error.
//  2. Acquiring a semaphore slot to respect the configured maximum concurrency (MaxWorkers):
//     - If a slot isn't available within a predefined timeout (5s), the job fails with ErrTooManyJobs.
//  3. Executing the job safely in a goroutine:
//     - Captures and handles panics, preventing scheduler-wide crashes.
//     - Updates job status and records metrics before, during, and after execution.
//  4. Ensuring proper synchronization via the provided WaitGroup.
//
// Parameters:
//   - job: The job instance to be executed.
//   - sem: A buffered channel serving as a semaphore to enforce concurrency limits.
//   - wg: A WaitGroup instance to manage execution synchronization.
func (p *Pool) execute(job Job, sem chan struct{}, wg *sync.WaitGroup) {
	meta := job.GetMetadata()

	var (
		err    error
		status = domain.Completed
	)
	execErr := job.CanExecute()
	if errors.Is(execErr, errs.ErrJobExecTooEarly) {
		return
	}

	// Track the job execution via WaitGroup.
	wg.Add(1)

	go func() {
		// Defer finalization logic to ensure cleanup, semaphore release, and metrics updates.
		defer func() {
			// Release the semaphore slot after execution.
			<-sem

			// Recover from potential panics during job execution.
			if r := recover(); r != nil {
				status = domain.Error
				err = errs.New(errs.ErrJobPanicked, fmt.Sprintf("panic: %v, job id: %s", r, meta.ID))
			}

			// Mark the job as completed with the final status and record metrics.
			job.ProcessEnd(status, err, p.mon)

			// Indicate that the job has fully completed.
			wg.Done()
		}()

		// Execute the job's main function, handle execution errors.
		err = job.Execute()
		if err != nil {
			status = domain.Error
		}
	}()
}
