package job

import (
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
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
		if finalizeErr := j.finalizeExecution(); finalizeErr != nil {
			err = finalizeErr
		}
	}()

	// Run OnStart hook
	if err = j.runHook(j.Hooks.OnStart, errs.ErrOnStartHook); err != nil {
		return err
	}

	// Run main job function
	if execErr := j.Fn(j.ctrl); execErr != nil {
		// Run OnError hook (errors from it are logged, not returned)
		_ = j.runHook(j.Hooks.OnError, errs.ErrOnErrorHook)

		// Propagate original function error
		err = errs.New(errs.ErrJobExecution, fmt.Sprintf("job id: %s, error: %v", j.ID, execErr))
		j.handleError(err)
		return
	}

	// Run OnSuccess hook
	err = j.runHook(j.Hooks.OnSuccess, errs.ErrOnSuccessHook)

	return
}

// runHook safely executes a lifecycle hook with panic recovery and optional error suppression.
//
// Parameters:
//   - hook: The Hook struct containing the function and IgnoreError flag.
//   - errType: A base error used for wrapping in case of failure.
//
// Behavior:
//   - Executes the provided hook function if defined.
//   - Wraps and logs any returned error.
//   - Recovers from panics, wraps them as hook errors.
//   - If `IgnoreError` is true, the error is suppressed and not returned.
//
// Returns:
//   - nil if the hook is nil or completed successfully, or if errors are ignored.
//   - A wrapped error if the hook fails and is not suppressed.
func (j *Job) runHook(hook domain.Hook, errType error) (err error) {
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

	if err = hook.Fn(j.ctrl, nil); err != nil {
		err = errs.New(errType, fmt.Sprintf("job id: %s, error: %v", j.ID, err))
		return
	}

	return
}

// finalizeExecution finalizes job lifecycle after execution completes,
// ensuring post-execution cleanup and signaling future availability.
//
// Behavior:
//   - Executes the `Finally` hook regardless of success/failure.
//   - Signals the job is available again by pushing to the `doneCh` channel.
//
// Returns:
//   - Error from the `Finally` hook, if any.
//   - nil if cleanup completed successfully.
func (j *Job) finalizeExecution() error {
	if err := j.runHook(j.Hooks.Finally, errs.ErrFinallyHook); err != nil {
		return err
	}

	select {
	case <-j.doneCh:
		// Ensure channel is drained before reuse
	default:
	}

	j.doneCh <- struct{}{}
	return nil
}
