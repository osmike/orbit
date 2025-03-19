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
// The function follows these steps:
// 1. Validates whether the job is eligible for execution using CanExecute().
// 2. If execution is not allowed, determines the appropriate handling (ignore, mark as ended, or return an error).
// 3. Uses a semaphore to control concurrency limits, ensuring no job exceeds the allowed number of workers.
// 4. Runs the job inside a goroutine, ensuring it starts execution only if the semaphore slot is acquired.
// 5. Handles execution errors and panics gracefully, preventing crashes and ensuring proper cleanup.
// 6. Updates job execution metadata and triggers lifecycle hooks.
//
// Parameters:
// - job: The job instance to execute.
// - sem: A buffered channel acting as a semaphore to control concurrency.
// - wg: A wait group to synchronize job execution.
func (p *Pool) Execute(job Job, sem chan struct{}, wg *sync.WaitGroup) {
	meta := job.GetMetadata()

	var (
		err      error
		status   = domain.Completed
		userData map[string]interface{}
	)

	// Check if the job is allowed to execute
	execErr := job.CanExecute()
	if execErr != nil {
		switch {
		case errors.Is(execErr, errs.ErrJobExecTooEarly):
			// Skip execution silently
			return
		case errors.Is(execErr, errs.ErrJobExecAfterEnd):
			// Mark job as ended
			status = domain.Ended
		case errors.Is(execErr, errs.ErrJobWrongStatus):
			// Report incorrect job state
			err = errs.New(execErr, meta.ID)
			status = domain.Error
		}
	}

	// Launch execution in a goroutine
	wg.Add(1) // Ensure the job is accounted for
	go func() {

		defer func() {
			// Release the semaphore slot
			<-sem

			// Handle panic recovery to prevent scheduler crashes
			if r := recover(); r != nil {
				status = domain.Error
				err = errs.New(errs.ErrJobPanicked, fmt.Sprintf("panic: %v, job id: %s", r, meta.ID))
			}

			// Ensure job is marked as finished, with final execution status
			wg.Done()
			job.ProcessEnd(status, err)
			p.mon.SaveMetrics(userData)
		}()

		// Acquire a semaphore slot to enforce max worker constraints
		select {
		case sem <- struct{}{}:
			// Successfully acquired a slot, execution continues
		case <-time.After(5 * time.Second): // Prevents deadlocks
			// Job could not acquire a slot, mark execution as failed
			err = errs.New(errs.ErrTooManyJobs, meta.ID)
			status = domain.Error
			return
		}

		// Mark job as started
		job.ProcessStart()

		// Execute the job function
		err = job.Execute()
		if err != nil {
			status = domain.Error
		}
	}()
}
