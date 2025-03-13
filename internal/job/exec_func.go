package job

import (
	"go-scheduler/internal/domain"
	"time"
)

// ExecFunc executes the job function, handling lifecycle hooks and updating execution metrics.
//
// This function performs the following steps:
// 1. Initializes FnControl for managing execution state.
// 2. Calls the OnStart hook if defined, aborting execution if it fails.
// 3. Executes the job's main function.
// 4. Calls the OnSuccess hook if the job completes successfully.
// 5. Calls the Finally hook in a defer statement, allowing it to override the final status if needed.
// 6. Ensures that ProcessJobEnd is always executed exactly once, updating job execution results.
//
// The job lifecycle hooks (`OnStart`, `OnSuccess`, `OnError`, and `Finally`) allow for custom behavior
// before, after, and during execution. The `Finally` hook can override the job status if it fails.
func (j *Job) ExecFunc() {
	startTime := time.Now()

	// Initialize execution control
	ctrl := &FnControl{
		Ctx:        j.ctx,
		PauseChan:  j.pauseCh,
		ResumeChan: j.resumeCh,
		data:       &j.state.Data,
	}

	// Default final status and error
	var finalStatus = domain.Completed
	var finalErr error

	// Ensure ProcessJobEnd is always executed, even if the job fails
	defer func() {
		// Execute Finally hook, allowing it to override the final job status
		if j.Hooks.Finally != nil {
			if err := j.Hooks.Finally(ctrl); err != nil {
				finalStatus, finalErr = domain.Error, err
			}
		}
		// Finalize job execution, updating execution time and status
		j.ProcessJobEnd(startTime, finalStatus, finalErr)
	}()

	// Execute the OnStart hook if defined
	if j.Hooks.OnStart != nil {
		if err := j.Hooks.OnStart(ctrl); err != nil {
			finalStatus, finalErr = domain.Error, err
		}
	}

	// Execute the main job function
	if err := j.Fn(ctrl); err != nil {
		finalStatus, finalErr = domain.Error, err
		if j.Hooks.OnError != nil {
			j.Hooks.OnError(ctrl, err)
		}
		return
	}

	// Execute the OnSuccess hook if the job completes successfully
	if j.Hooks.OnSuccess != nil {
		if err := j.Hooks.OnSuccess(ctrl); err != nil {
			finalStatus, finalErr = domain.Error, err
			return
		}
	}
}
