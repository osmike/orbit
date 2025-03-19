package job

import (
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"time"
)

// Pause sends a pause signal to the job, transitioning it to the Paused state.
func (j *Job) Pause(timeout time.Duration) error {
	if !j.TrySetStatus([]domain.JobStatus{domain.Running}, domain.Paused) {
		return errs.New(errs.ErrJobNotRunning, j.ID)
	}
	select {
	case j.pauseCh <- struct{}{}:
		select {
		case <-time.After(timeout):
			<-j.pauseCh
			j.TrySetStatus([]domain.JobStatus{domain.Paused}, domain.Running)
		default:
		}
	default:
	}
	return nil
}

// Resume sends a resume signal to the job, allowing it to continue execution if it was paused.
func (j *Job) Resume() error {
	switch j.GetStatus() {
	case domain.Paused:
		select {
		case j.resumeCh <- struct{}{}:
		default:
		}
	case domain.Stopped:
		j.SetStatus(domain.Waiting)
	default:
		return errs.New(errs.ErrJobNotPausedOrStopped, j.ID)
	}
	return nil
}

// Stop cancels the job execution and updates its status accordingly.
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
}
