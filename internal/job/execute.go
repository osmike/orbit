package job

import (
	"fmt"
	"github.com/osmike/orbit/internal/domain"
	errs "github.com/osmike/orbit/internal/error"
)

// Execute runs the job's main function, managing the full lifecycle including
// pre-execution checks, hook invocations, error propagation, and cleanup logic.
//
// Execution flow:
//  1. Checks that the job is not already running by verifying the `doneCh` channel.
//     Returns ErrJobStillRunning if the job is currently in progress.
//  2. Ensures the `Finally` hook is always executed after job logic (deferred).
//  3. Executes the `OnStart` hook. If it fails and `IgnoreError` is false, job execution stops.
//  4. Executes the user-defined function (`Fn`). If it returns an error, the `OnError` hook is run.
//     The error is wrapped and passed to the job's error handler.
//  5. If execution is successful, runs the `OnSuccess` hook.
//  6. Any error from `Finally` overrides prior ones.
//
// Returns:
//   - The resulting error from the main function or any hook, or nil on success.
func (j *Job) Execute() (err error) {
	// Ensure the job is not already running
	select {
	case <-j.doneCh:
		// Job is not running — allow execution to proceed.
	default:
		// Job is already running
		err = errs.New(errs.ErrJobStillRunning, j.ID)
		return
	}

	// Always execute Finally hook regardless of prior success or failure
	defer func() {
		if r := recover(); r != nil {
			err = errs.New(errs.ErrJobPanicked, fmt.Sprintf("panic: %v, job id: %s", r, j.ID))
		}
		if finalizeErr := j.finalizeExecution(); finalizeErr != nil {
			err = finalizeErr
		}
		j.handleError(err)
	}()

	// Run OnStart hook
	if err = j.runHook(j.Hooks.OnStart, errs.ErrOnStartHook, nil); err != nil {
		return err
	}

	// Run main job function
	if execErr := j.Fn(j.ctrl); execErr != nil {
		// Run OnError hook (errors from it are logged, not returned)
		_ = j.runHook(j.Hooks.OnError, errs.ErrOnErrorHook, execErr)

		// Propagate original function error
		err = errs.New(errs.ErrJobExecution, fmt.Sprintf("job id: %s, error: %v", j.ID, execErr))
		return
	}

	// Run OnSuccess hook
	err = j.runHook(j.Hooks.OnSuccess, errs.ErrOnSuccessHook, nil)

	return
}

// runHook safely executes a lifecycle hook with panic recovery and optional error suppression.
//
// Parameters:
//   - hook: The Hook struct containing the function and IgnoreError flag.
//   - errType: A base error used for wrapping in case of failure.
//   - execErr: A execution error, that handles in OnError hook
//
// Behavior:
//   - Executes the provided hook function if defined.
//   - Recovers from panics, wrapping them as hook errors.
//   - Any encountered error is passed to the internal error handler (handleError).
//   - If `IgnoreError` is true, the error is suppressed and not returned by this method (but still reported internally).
//
// Returns:
//   - nil if the hook is nil or completed successfully, or if errors are ignored.
//   - A wrapped error if the hook fails and is not suppressed.
func (j *Job) runHook(hook domain.Hook, errType error, execErr error) (err error) {
	if hook.Fn == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = errs.New(errType, fmt.Sprintf("job id: %s, panic: %v", j.ID, r))
		}
		j.handleError(err)
		if hook.IgnoreError {
			err = nil
		}
	}()

	if err = hook.Fn(j.ctrl, execErr); err != nil {
		err = errs.New(errType, fmt.Sprintf("job id: %s, error: %v", j.ID, err))
		return
	}

	return
}

// finalizeExecution finalizes the job lifecycle after the main function completes,
// regardless of whether it ended in success, failure, or was canceled.
//
// This method ensures proper cleanup and reusability of the job instance in the following way:
//
// Behavior:
//   - Always executes the `Finally` hook, even if the main function panicked or returned an error.
//   - Clears the `pauseCh` and `resumeCh` channels in case the job was paused/resumed
//     during execution but didn't handle the signals (e.g., finished too early).
//   - Drains and resets the `doneCh` semaphore to mark the job as available for future runs.
//
// Returns:
//   - An error returned by the `Finally` hook, if any.
//   - nil if cleanup completes successfully.
func (j *Job) finalizeExecution() error {
	if err := j.runHook(j.Hooks.Finally, errs.ErrFinallyHook, nil); err != nil {
		return err
	}

	select {
	case <-j.doneCh:
		// Ensure channel is drained before reuse
	default:
	}

	j.doneCh <- struct{}{}

	select {
	case <-j.pauseCh:
		// Clean up pause channel in case of calling Pause() during execution
	default:
	}

	select {
	case <-j.resumeCh:
		// Clean up pause channel in case of calling Resume() during execution
	default:
	}

	return nil
}
