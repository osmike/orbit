package job

import (
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"time"
)

// Pause attempts to pause the currently running job by sending a pause signal via pauseCh.
// It waits for the job to acknowledge the pause by checking if the status remains "Paused".
// If the job does not handle the pause signal within the given timeout, the status is reverted back to "Running".
//
// Parameters:
//   - timeout: Duration to wait for the job to acknowledge the pause.
//     If zero, a default timeout is used.
//
// Returns:
//   - An error if the job is not in the Running state,
//     or if the pause is not acknowledged within the timeout.
func (j *Job) Pause(timeout time.Duration) error {
	if !j.TrySetStatus([]domain.JobStatus{domain.Running}, domain.Paused) {
		return errs.New(errs.ErrJobNotRunning, j.ID)
	}

	if timeout == 0 {
		timeout = domain.DEFAULT_PAUSE_TIMEOUT
	}

	// Try to send pause signal without blocking
	select {
	case j.pauseCh <- struct{}{}:
		// Sent pause signal, wait for the job to process it
	case <-time.After(timeout):
		// Could not send signal, treat as already paused/busy
		j.SetStatus(domain.Running)
		return errs.New(errs.ErrJobPaused, "pause signal channel busy or blocked")
	default:
		// pauseCh is already full (unprocessed pause), reject pause attempt
		j.SetStatus(domain.Running)
		return errs.New(errs.ErrJobPaused, j.ID)
	}

	// Wait for job to process pause
	time.Sleep(timeout)

	// If job didn't stay in Paused — revert
	if j.GetStatus() != domain.Paused {
		j.SetStatus(domain.Running)
		return errs.New(errs.ErrJobPaused, "pause timeout exceeded, job did not handle pause signal")
	}

	// Run hook if provided
	return j.runHook(j.Hooks.OnPause, errs.ErrOnPauseHook)
}

// Resume sends a resume signal to a paused or stopped job, allowing it to continue execution.
//
// If the job was explicitly paused, this method triggers the OnResume hook and moves the job back
// into the Running state. If the job was stopped, it transitions the job into the Waiting state.
//
// Returns:
//   - An error if the job is not currently paused or stopped, or if resumption fails.
func (j *Job) Resume() error {
	switch j.GetStatus() {
	case domain.Paused:
		select {
		case j.resumeCh <- struct{}{}:
			return j.runHook(j.Hooks.OnResume, errs.ErrOnResumeHook)
		default:
			return errs.New(errs.ErrJobWrongStatus, "failed to send resume signal")
		}
	case domain.Stopped:
		j.SetStatus(domain.Waiting)
		return j.runHook(j.Hooks.OnResume, errs.ErrOnResumeHook)
	default:
		return errs.New(errs.ErrJobNotPausedOrStopped, j.ID)
	}
}

// Stop immediately cancels the job's execution context, signaling termination,
// and updates its execution status accordingly.
//
// For jobs currently running, paused, or waiting, it updates the status to Stopped,
// records the end timestamp, and finalizes the state.
//
// This operation is safe to call multiple times.
func (j *Job) Stop() {
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
	if j.Hooks.OnStop != nil {
		_ = j.runHook(j.Hooks.OnStop, errs.ErrJobWrongStatus)
	}
}
