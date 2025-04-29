package pool

import (
	"errors"
	"orbit/internal/domain"
	errs "orbit/internal/error"
	"sync"
)

// execute schedules and runs a job inside a separate goroutine, assuming a semaphore slot is already acquired.
//
// Execution flow:
//  1. Validates job readiness via CanExecute():
//     - If the job is not yet due (ErrJobExecTooEarly), execution is skipped.
//     - If the job is otherwise invalid, execution is skipped or job status is updated.
//  2. Registers the job execution in the WaitGroup to track active jobs.
//  3. Launches a new goroutine:
//     - Calls job.Execute().
//     - Updates final status based on execution result.
//     - Calls ProcessEnd() to finalize the job state.
//     - Releases the semaphore slot.
//     - Marks the job as done in the WaitGroup.
//
// Parameters:
//   - job: Job to execute.
//   - sem: Semaphore channel that controls maximum concurrency.
//   - wg: WaitGroup to synchronize the completion of active jobs.
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
