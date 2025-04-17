package job

import (
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
)

// Execute runs the job's main function, managing the full lifecycle including state control, hooks, and error propagation.
//
// Execution Flow:
//  1. Verifies that the job is not already running. If it is, returns ErrJobStillRunning.
//  2. Resets internal state (`FnControl.data`) for the new execution.
//  3. Executes the OnStart hook. If this hook fails and IgnoreError is false, execution is aborted.
//  4. Runs the job’s main logic (`Fn`). On failure, triggers the OnError hook and propagates the error.
//  5. If the job function completes successfully, invokes the OnSuccess hook.
//  6. Regardless of outcome, executes the Finally hook as a cleanup step.
//
// Returns:
//   - An error indicating the result of the execution or hook failures. If multiple failures occur,
//     the last one takes precedence.
func (j *Job) Execute() (err error) {
	// Ensure the job is not already running
	select {
	case <-j.doneCh:
		j.ctrl.data = &sync.Map{}
	default:
		err = errs.New(errs.ErrJobStillRunning, j.ID)
		return
	}

	// Ensure the Finally hook always runs
	defer func() {
		if finalizeErr := j.finalizeExecution(); finalizeErr != nil {
			err = finalizeErr
		}
	}()

	// Execute OnStart hook
	if err = j.runHook(j.Hooks.OnStart, errs.ErrOnStartHook); err != nil {
		return err
	}

	// Execute the job’s main function
	if execErr := j.Fn(j.ctrl); execErr != nil {
		err = j.runHook(j.Hooks.OnError, errs.ErrOnErrorHook)
		err = errs.New(errs.ErrJobExecution, fmt.Sprintf("job id: %s, error: %v", j.ID, execErr))
		j.handleError(err)
		return
	}

	// Execute OnSuccess hook
	err = j.runHook(j.Hooks.OnSuccess, errs.ErrOnSuccessHook)

	return
}

// runHook safely executes a lifecycle hook, handles any errors, and conditionally suppresses them based on the hook configuration.
//
// Parameters:
//   - hook: The lifecycle hook to execute (e.g., OnStart, OnSuccess, etc.).
//   - errType: The logical error type to wrap the hook error.
//
// Behavior:
//   - Recovers from panics and wraps them into typed hook errors.
//   - Delegates hook errors to the job’s error handler.
//   - Respects the `IgnoreError` flag: if true, hook errors are suppressed.
//
// Returns:
//   - A wrapped error if the hook fails and is not suppressed, nil otherwise.
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

// finalizeExecution ensures that the job’s Finally hook is executed after every run,
// and resets the execution control channel to allow future runs.
//
// Behavior:
//   - Executes the Finally hook regardless of prior errors or success.
//   - Resets the `doneCh` channel used to mark job availability.
//
// Returns:
//   - An error if the Finally hook fails, nil otherwise.
func (j *Job) finalizeExecution() error {
	if err := j.runHook(j.Hooks.Finally, errs.ErrFinallyHook); err != nil {
		return err
	}

	select {
	case <-j.doneCh:
	default:
	}
	j.doneCh <- struct{}{}
	return nil
}
