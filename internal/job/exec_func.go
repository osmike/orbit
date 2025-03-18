package job

import (
	"fmt"
	errs "go-scheduler/internal/error"
)

// ExecFunc executes the job function, triggering lifecycle hooks and returning execution errors.
//
// This function follows these steps:
// 1. Initializes execution control with FnControl.
// 2. Runs OnStart hook, aborting execution if it fails.
// 3. Executes the main job function.
// 4. Runs OnSuccess hook if the job succeeds.
// 5. Executes the Finally hook in defer, allowing it to modify the final error.
// 6. Ensures the returned error reflects the actual execution outcome.
func (j *Job) ExecFunc() (err error) {
	select {
	case <-j.doneCh:
	default:
		return errs.New(errs.ErrJobStillRunning, j.ID)
	}
	// Initialize execution control
	ctrl := &FnControl{
		Ctx:        j.ctx,
		PauseChan:  j.pauseCh,
		ResumeChan: j.resumeCh,
		data:       &j.state.Data,
	}

	// Ensure Finally hook always executes
	defer func() {
		if j.Hooks.Finally != nil {
			if finallyErr := j.Hooks.Finally(ctrl); finallyErr != nil {
				err = errs.New(errs.ErrFinallyHook, fmt.Sprintf("job id: %s, error: %v", j.ID, finallyErr))
			}
		}
		j.doneCh <- struct{}{}
	}()

	// Run OnStart hook
	if j.Hooks.OnStart != nil {
		if hookErr := j.Hooks.OnStart(ctrl); hookErr != nil {
			return errs.New(errs.ErrOnStartHook, fmt.Sprintf("job id: %s, error: %v", j.ID, hookErr))
		}
	}

	// Execute the job function
	if execErr := j.Fn(ctrl); execErr != nil {
		if j.Hooks.OnError != nil {
			j.Hooks.OnError(ctrl, execErr)
		}
		return errs.New(errs.ErrJobExecution, fmt.Sprintf("job id: %s, error: %v", j.ID, execErr))
	}

	// Run OnSuccess hook
	if j.Hooks.OnSuccess != nil {
		if successErr := j.Hooks.OnSuccess(ctrl); successErr != nil {
			return errs.New(errs.ErrOnSuccessHook, fmt.Sprintf("job id: %s, error: %v", j.ID, successErr))
		}
	}

	return nil
}
