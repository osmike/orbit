package scheduler

import (
	"fmt"
	"sync"
	"time"
)

// execute launches the job execution in a separate goroutine while managing concurrency limits.
// It ensures that the job is allowed to execute before proceeding and tracks execution metrics.
//
// The function performs the following actions:
// 1. Adds the job to the wait group to ensure proper synchronization.
// 2. Calls `canExecute` to check if execution conditions are met.
// 3. Runs the job function asynchronously within a controlled execution environment.
// 4. Handles panic recovery to prevent scheduler crashes.
// 5. Records execution time and error state for monitoring and retries.
//
// Parameters:
//   - wg: A pointer to a sync.WaitGroup to synchronize job execution.
//   - sem: A buffered channel acting as a semaphore to control concurrency.
func (j *Job) execute(wg *sync.WaitGroup, sem chan struct{}) {
	wg.Add(1)
	defer wg.Done()

	// Ensure the job is eligible for execution before proceeding.
	if err := j.canExecute(); err != nil {
		return
	}

	select {
	case <-j.ctx.Done():
		j.setStatus(Stopped)
		return
	default:
	}

	// Execute the job asynchronously in a separate goroutine.
	go func() {
		// Acquire a semaphore slot to enforce max worker constraints.
		sem <- struct{}{}
		defer func() { <-sem }() // Release the semaphore after execution.

		startTime := time.Now()
		j.State.StartAt = startTime

		// Handle panic recovery to prevent the scheduler from crashing due to job failures.
		defer func() {
			if r := recover(); r != nil {
				j.setStatus(Error)
				j.State.Error = newErr(ErrJobPanicked, fmt.Sprintf("%v, id: %s", r, j.ID))
				j.State.ExecutionTime = time.Since(startTime).Nanoseconds()
			}
		}()

		// Create a control interface to manage job execution state.
		ctrl := FnControl{
			Ctx:        j.ctx,
			PauseChan:  j.pauseCh,
			ResumeChan: j.resumeCh,
			data:       &j.State.Data,
		}

		// Execute the job function and track execution duration.
		if j.Hooks.OnStart != nil {
			if err := j.Hooks.OnStart(ctrl); err != nil {
				j.setStatus(Error)
				j.State.Error = err
			}
		}
		err := j.Fn(ctrl)
		j.State.ExecutionTime = time.Since(startTime).Nanoseconds()

		// Handle job completion and error tracking.
		if err != nil {
			j.setStatus(Error)
			j.State.Error = err
			if j.Hooks.OnError != nil {
				j.Hooks.OnError(ctrl, err)
			}
		} else {
			j.setStatus(Completed)

			// Reset retry counter on successful execution if enabled.
			if j.State.Error != nil {
				if j.Retry.ResetOnSuccess {
					j.State.currentRetry = j.Retry.Count
				}
				j.State.Error = nil
			}

			if j.Hooks.OnSuccess != nil {
				if err := j.Hooks.OnSuccess(ctrl); err != nil {
					j.setStatus(Error)
					j.State.Error = err
				}
			}
		}
	}()
}

// canExecute determines if the job is eligible for execution based on its configuration and timing constraints.
// It prevents execution if the job is already running, scheduled for a future time, or has expired.
//
// The function performs the following checks:
// 1. Ensures one-time jobs do not execute again after completion.
// 2. Prevents concurrent execution of the same job.
// 3. Enforces execution within the allowed time window (StartAt - EndAt).
// 4. Applies a delay before execution if necessary.
//
// Returns:
//   - true if the job can proceed with execution.
//   - false if any conditions prevent execution.
func (j *Job) canExecute() error {
	// Prevent one-time jobs from running again after completion.
	delay := j.getDelay()
	if delay > 0 && !j.State.EndAt.IsZero() {
		j.setStatus(Stopped)
		return fmt.Errorf("job has expired")
	}

	// Ensure the job is not already running or blocked from execution.
	if !j.tryChangeStatus([]JobStatus{Waiting}, Running) {
		return fmt.Errorf("job is already running")
	}

	// Prevent execution before the scheduled start time.
	if time.Now().Before(j.StartAt) {
		return fmt.Errorf("job is not scheduled to run yet")
	}

	// Stop execution if the job's allowed execution window has expired.
	if time.Now().After(j.EndAt) {
		j.setStatus(Stopped)
		return fmt.Errorf("job has expired")
	}

	// Calculate execution delay and apply it if necessary.
	if delay > 0 {
		select {
		case <-time.After(delay): // Wait for the specified delay before running.
			return nil
		case <-j.ctx.Done(): // Abort if the job is canceled during the wait.
			return fmt.Errorf("job canceled")
		}
	}

	return nil
}
