package job

import (
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
)

// Execute runs the job's main function, managing the full lifecycle, including hooks and error handling.
//
// Execution steps:
//  1. Checks if the job is currently not running. If it's already running, immediately returns ErrJobStillRunning.
//  2. Initializes execution context (`FnControl`).
//  3. Executes the OnStart hook. If this hook returns an error, execution stops immediately.
//  4. Executes the job's main function (`Fn`). If the job function fails, triggers the OnError hook.
//  5. Executes the OnSuccess hook if the job function completed without errors.
//  6. Ensures the Finally hook runs last, regardless of the job's success or failure status.
//
// Returns:
//   - An error indicating the execution outcome or hook failures. If multiple hooks fail, the last encountered error is returned.
func (j *Job) Execute() (err error) {
	// Check if the job is already running
	select {
	case <-j.doneCh:
		j.ctrl.data = &sync.Map{}
	default:
		return errs.New(errs.ErrJobStillRunning, j.ID)
	}

	// Ensure Finally hook always executes
	defer func() {
		if finalizeErr := j.finalizeExecution(); finalizeErr != nil {
			err = finalizeErr
		}
	}()

	// Run OnStart hook, abort execution on error
	if err = j.runHook(j.Hooks.OnStart, errs.ErrOnStartHook); err != nil {
		return err
	}

	// Execute main job function (Fn)
	if execErr := j.Fn(j.ctrl); execErr != nil {
		if j.Hooks.OnError != nil {
			j.Hooks.OnError(j.ctrl, execErr)
		}
		err = errs.New(errs.ErrJobExecution, fmt.Sprintf("job id: %s, error: %v", j.ID, execErr))
		return err
	}

	// Run OnSuccess hook
	if err = j.runHook(j.Hooks.OnSuccess, errs.ErrOnSuccessHook); err != nil {
		return err
	}

	return nil
}

// runHook executes a given lifecycle hook safely and wraps any hook errors.
//
// Parameters:
//   - hook: The hook function to execute (OnStart, OnSuccess, etc.).
//   - errType: The error type to wrap hook execution errors.
//
// Returns:
//   - An error if the hook fails, nil otherwise.
func (j *Job) runHook(hook func(ctrl domain.FnControl) error, errType error) error {
	if hook != nil {
		if err := hook(j.ctrl); err != nil {
			return errs.New(errType, fmt.Sprintf("job id: %s, error: %v", j.ID, err))
		}
	}
	return nil
}

// finalizeExecution executes the Finally hook, ensuring that this final cleanup always occurs after job execution,
// regardless of previous errors or successful completion.
//
// This method also resets the job execution signal (`doneCh`) to allow future executions.
//
// Returns:
//   - An error if the Finally hook encounters an issue.
func (j *Job) finalizeExecution() error {
	if j.Hooks.Finally != nil {
		if err := j.Hooks.Finally(j.ctrl); err != nil {
			return errs.New(errs.ErrFinallyHook, fmt.Sprintf("job id: %s, error: %v", j.ID, err))
		}
	}

	select {
	case <-j.doneCh:
	default:
	}
	j.doneCh <- struct{}{}
	return nil
}
