package job

import (
	"orbit/internal/domain"
	errs "orbit/internal/error"
	"time"
)

// Pause attempts to pause the currently running job by sending a non-blocking signal to pauseCh.
//
// Once the pause signal is sent, a timeout watcher is started in a separate goroutine.
// If the job fails to acknowledge the pause (by reading from pauseCh) within the specified timeout,
// the job's status is automatically reverted back to Running.
//
// This mechanism ensures that jobs which do not support pause/resume don't get stuck in the Paused state.
//
// Parameters:
//   - timeout: Duration to wait for pause acknowledgment. If zero, DEFAULT_PAUSE_TIMEOUT is used.
//
// Returns:
//   - An error if the job is not in the Running state,
//     or if the pause signal could not be delivered because the job has already received one.
func (j *Job) Pause(timeout time.Duration) error {
	if !j.TrySetStatus([]domain.JobStatus{domain.Running}, domain.Paused) {
		return errs.New(errs.ErrJobNotRunning, j.ID)
	}

	if timeout == 0 {
		timeout = domain.DEFAULT_PAUSE_TIMEOUT
	}

	// Attempt to signal pause to the job.
	select {
	case j.pauseCh <- struct{}{}:
		// If sent, wait asynchronously for the job to acknowledge pause.
		go func() {
			time.Sleep(timeout)
			select {
			case <-j.pauseCh:
				// Pause signal was not consumed — job is not respecting pause, revert to Running
				j.TrySetStatus([]domain.JobStatus{domain.Paused}, domain.Running)
			default:
				// pauseCh already consumed — no action needed
			}
		}()
	default:
		return errs.New(errs.ErrJobPaused, j.ID)
	}

	return j.runHook(j.Hooks.OnPause, errs.ErrOnPauseHook)
}

// Resume sends a resume signal to a paused or stopped job, allowing it to continue execution.
//
// If the job was paused, it sends a signal to resumeCh and triggers the OnResume hook.
// If the job was stopped, it sets the status to Waiting and triggers the hook.
//
// Returns:
//   - An error if the job is not in the Paused or Stopped state,
//     or if the resume signal cannot be delivered (e.g., if the channel is full).
func (j *Job) Resume() error {
	switch j.GetStatus() {
	case domain.Paused:
		select {
		case j.resumeCh <- struct{}{}:
			j.SetStatus(domain.Running)
			return j.runHook(j.Hooks.OnResume, errs.ErrOnResumeHook)
		default:
			// resumeCh is full — resume already in progress or not handled
			return errs.New(errs.ErrJobWrongStatus, "failed to send resume signal")
		}
	case domain.Stopped:
		j.SetStatus(domain.Waiting)
		return j.runHook(j.Hooks.OnResume, errs.ErrOnResumeHook)
	default:
		return errs.New(errs.ErrJobNotPausedOrStopped, j.ID)
	}
}

// Stop immediately cancels the job's execution context and updates its status to Stopped.
//
// For Running, Paused, or Waiting jobs — it forcibly marks them as Stopped and records the EndAt timestamp.
// For Completed or Error jobs — it updates the status to Stopped without altering timestamps.
//
// This method is idempotent and can be safely called multiple times.
//
// If the OnStop hook is defined, it is executed after updating the state.
func (j *Job) Stop() error {
	j.cancel()

	switch j.GetStatus() {
	case domain.Completed, domain.Error:
		j.SetStatus(domain.Stopped)
	default:
		j.UpdateState(domain.StateDTO{
			EndAt:  time.Now(),
			Status: domain.Stopped,
		})
	}

	// Execute the OnStop hook if provided (for cleanup or state saving).
	return j.runHook(j.Hooks.OnStop, errs.ErrOnStopHook)
}
