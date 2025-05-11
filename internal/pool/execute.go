package pool

import (
	"errors"
	"github.com/osmike/orbit/internal/domain"
	errs "github.com/osmike/orbit/internal/error"
	"sync"
)

// Execute schedules and runs a job in a separate goroutine, assuming that a worker slot
// in the provided semaphore has already been acquired.
//
// This method ensures that job execution is non-blocking and fully managed,
// including error propagation, lifecycle tracking, and proper synchronization
// with the worker pool via the semaphore and WaitGroup.
//
// Execution flow:
//  1. Validates job readiness using CanExecute():
//     - If the job is not yet due (ErrJobExecTooEarly), execution is skipped silently.
//     - If the job is otherwise invalid, it is skipped and may be marked accordingly.
//  2. Registers the job in the WaitGroup to ensure controlled shutdown.
//  3. Launches a goroutine that:
//     - Calls job.Execute() to run the job logic.
//     - Sets final status to Completed or Error based on the result.
//     - Calls ProcessEnd() to finalize the job state (e.g., update metrics, reset retry).
//     - Releases the semaphore slot (freeing concurrency for another job).
//     - Signals job completion to the WaitGroup.
//
// Parameters:
//   - job: The job instance to execute.
//   - sem: A buffered channel used as a semaphore to limit concurrent execution.
//   - wg: A WaitGroup used to track when all jobs have completed.
func (p *Pool) Execute(job domain.Job, sem chan struct{}, wg *sync.WaitGroup) {

	var (
		err    error
		status = domain.Completed
	)

	// Validate job eligibility for execution.
	execErr := job.CanExecute()

	if errors.Is(execErr, errs.ErrJobExecTooEarly) {
		return
	}

	// Mark the job as started, update metrics.
	job.ProcessStart()

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
