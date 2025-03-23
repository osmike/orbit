package job

import (
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"time"
)

// Pause attempts to pause the currently running job by sending a pause signal via pauseCh.
// The job's main execution logic should explicitly handle this pause signal by reading from pauseCh.
// If the job does not handle the pause signal within the specified timeout, it is assumed that
// the job does not support pause functionality, and the status is reverted back to Running.
//
// Parameters:
//   - timeout: Duration to wait for the job to acknowledge and process the pause signal.
//     If zero, the default timeout of 1 second is used.
//
// Returns:
//   - An error if the job is not in the Running state, is already paused,
//     or fails to handle the pause within the specified timeout.
func (j *Job) Pause(timeout time.Duration) error {
	if !j.TrySetStatus([]domain.JobStatus{domain.Running}, domain.Paused) {
		return errs.New(errs.ErrJobNotRunning, j.ID)
	}

	if timeout == 0 {
		timeout = domain.DEFAULT_PAUSE_TIMEOUT
	}

	select {
	case j.pauseCh <- struct{}{}:
		select {
		case <-time.After(timeout):
			<-j.pauseCh
			j.TrySetStatus([]domain.JobStatus{domain.Paused}, domain.Running)
			return errs.New(errs.ErrJobPaused, "pause timeout exceeded, job did not handle pause signal")
		default:
			return j.runHook(j.Hooks.OnPause, errs.ErrOnPauseHook)
		}
	default:
		return errs.New(errs.ErrJobPaused, j.ID)
	}
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
