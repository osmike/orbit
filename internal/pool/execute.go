package pool

import (
	"errors"
	"orbit/internal/domain"
	errs "orbit/internal/error"
	"sync"
)

// execute schedules and executes a job in a separate goroutine, respecting pool constraints and job validity.
//
// Execution flow:
//  1. Validates job readiness via CanExecute():
//     - Skips execution if job is not yet due (ErrJobExecTooEarly).
//     - If the job is ineligible due to timing or status issues, execution is skipped or state is updated.
//  2. Acquires a semaphore slot to enforce MaxWorkers limit.
//     - If no slot is available, job execution is postponed until one frees up.
//  3. Spawns a new goroutine to run the job:
//     - Calls job.Execute(), handling errors and panics.
//     - Calls ProcessEnd() with final status and error for proper cleanup.
//  4. Ensures synchronization with WaitGroup.
//
// Parameters:
//   - job: Job to be executed.
//   - sem: Semaphore channel limiting concurrent executions to MaxWorkers.
//   - wg: WaitGroup used to wait for all job executions to complete.
func (p *Pool) execute(job Job, sem chan struct{}, wg *sync.WaitGroup) {

	var (
		err    error
		status = domain.Completed
	)

	// Validate job eligibility for execution.
	execErr := job.CanExecute()
	if errors.Is(execErr, errs.ErrJobExecTooEarly) {
		return
	}

	wg.Add(1) // Register this job in the WaitGroup.

	go func() {
		defer func() {
			<-sem // Release the worker slot.

			job.ProcessEnd(status, err)
			wg.Done() // Mark job as done in WaitGroup.
		}()

		err = job.Execute()
		if err != nil {
			status = domain.Error
		}
	}()
}
