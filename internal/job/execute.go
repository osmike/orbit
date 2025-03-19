package job

import (
	"fmt"
	"go-scheduler/internal/domain"
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
func (j *Job) Execute() (err error) {
	select {
	case <-j.doneCh:
		j.ctrl.data = &map[string]interface{}{}
	default:
		return errs.New(errs.ErrJobStillRunning, j.ID)
	}

	// Ensure Finally hook always executes
	defer j.finalizeExecution()

	// Run OnStart hook
	if err := j.runHook(j.Hooks.OnStart, errs.ErrOnStartHook); err != nil {
		return err
	}

	// Execute the job function
	if execErr := j.Fn(j.ctrl); execErr != nil {
		if j.Hooks.OnError != nil {
			j.Hooks.OnError(j.ctrl, execErr)
		}
		return errs.New(errs.ErrJobExecution, fmt.Sprintf("job id: %s, error: %v", j.ID, execErr))
	}

	// Run OnSuccess hook
	return j.runHook(j.Hooks.OnSuccess, errs.ErrOnSuccessHook)
}

func (j *Job) runHook(hook func(ctrl domain.FnControl) error, errType error) error {
	if hook != nil {
		if err := hook(j.ctrl); err != nil {
			return errs.New(errType, fmt.Sprintf("job id: %s, error: %v", j.ID, err))
		}
	}
	return nil
}

func (j *Job) finalizeExecution() {
	if j.Hooks.Finally != nil {
		j.Hooks.Finally(j.ctrl)
	}

	select {
	case <-j.doneCh:
	default:
	}
	j.doneCh <- struct{}{}
}
