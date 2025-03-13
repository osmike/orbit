package pool

import (
	"errors"
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// Execute runs the given job, ensuring it respects scheduling constraints and manages execution concurrency.
//
// The function:
// - Validates whether the job is eligible for execution.
// - Handles execution errors gracefully.
// - Manages worker pool slots using a semaphore.
// - Executes the job in a separate goroutine with panic recovery.
//
// Parameters:
// - job: The job instance to execute.
// - sem: A buffered channel acting as a semaphore to control concurrency.
// - wg: A wait group to synchronize job execution.
//
// Returns:
// - An error if the job cannot be executed.
func (p *Pool) Execute(job Job, sem chan struct{}, wg *sync.WaitGroup) error {
	meta := job.GetMetadata()

	// Check if the job is allowed to execute
	err := job.CanExecute()
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrJobExecTooEarly):
			return nil
		case errors.Is(err, errs.ErrJobExecAfterEnd):
			job.SetStatus(domain.Ended)
		case errors.Is(err, errs.ErrJobWrongStatus):
			return errs.New(err, meta.ID)
		default:
			return err
		}
	}

	// Launch execution in a goroutine
	wg.Add(1) // Ensure the job is accounted for
	go func() {
		defer wg.Done()

		startTime := time.Now()

		// Acquire a semaphore slot to enforce max worker constraints
		select {
		case sem <- struct{}{}:
			// Successfully acquired a slot
		case <-time.After(5 * time.Second): // Prevents deadlocks
			job.UpdateState(domain.StateDTO{
				Status:        domain.Error,
				Error:         errs.New(errs.ErrTooManyJobs, meta.ID),
				ExecutionTime: time.Since(startTime).Nanoseconds(),
			})
			return
		}

		defer func() { <-sem }() // Release the semaphore after execution

		// Handle panic recovery to prevent scheduler crashes
		defer func() {
			if r := recover(); r != nil {
				err := errs.New(errs.ErrJobPanicked, fmt.Sprintf("%v, id: %s", r, meta.ID))
				job.UpdateState(domain.StateDTO{
					Status:        domain.Error,
					Error:         err,
					ExecutionTime: time.Since(startTime).Nanoseconds(),
				})
			}
		}()

		// Execute the job
		job.ExecFunc()
	}()
	return nil
}
