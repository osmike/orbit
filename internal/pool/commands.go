package pool

import (
	"orbit/internal/domain"
	errs "orbit/internal/error"
	"time"
)

// AddJob adds a new job to the scheduler pool for future execution.
//
// The method performs the following validations:
//   - Ensures the Job ID is unique within the pool.
//   - Checks that the Job status is Waiting, indicating readiness for execution.
//
// Parameters:
//   - job: The Job instance to add to the scheduler.
//
// Returns:
//   - An error (ErrIDExists, ErrJobWrongStatus) if validation fails.
//   - nil if the job is successfully added to the pool.
func (p *Pool) AddJob(job Job) error {
	meta := job.GetMetadata()

	if _, ok := p.jobs.Load(meta.ID); ok {
		return errs.New(errs.ErrIDExists, meta.ID)
	}
	if job.GetStatus() != domain.Waiting {
		return errs.New(errs.ErrJobWrongStatus, meta.ID)
	}
	if meta.Timeout == 0 {
		job.SetTimeout(p.IdleTimeout)
	}
	p.jobs.Store(meta.ID, job)
	return nil
}

// RemoveJob deletes an existing job from the scheduler pool.
//
// The method:
//   - Removes the job entry from the pool's internal storage.
//   - It does not actively stop a running job; jobs are stopped separately via StopJob.
//
// Parameters:
//   - id: Unique identifier of the job to remove.
//
// Returns:
//   - An error (ErrJobNotFound) if the specified job does not exist.
//   - nil if the job is successfully removed.
func (p *Pool) RemoveJob(id string) error {
	_, err := p.getJobByID(id)
	if err != nil {
		return err
	}
	p.jobs.Delete(id)
	return nil
}

// PauseJob temporarily pauses execution of the specified job.
//
// If no timeout is explicitly provided (timeout = 0), the default pause timeout (DEFAULT_PAUSE_TIMEOUT) is applied.
//
// Parameters:
//   - id: Unique identifier of the job to pause.
//   - timeout: Duration to wait for job acknowledgement of pause request.
//
// Returns:
//   - An error (ErrJobNotFound, ErrJobPaused, ErrJobNotRunning) if the job does not exist,
//     is already paused, or cannot be paused.
//   - nil if the pause request is successful.
func (p *Pool) PauseJob(id string, timeout time.Duration) error {
	job, err := p.getJobByID(id)
	if err != nil {
		return err
	}
	if timeout == 0 {
		timeout = domain.DEFAULT_PAUSE_TIMEOUT
	}
	return job.Pause(timeout)
}

// ResumeJob resumes a paused or stopped job, allowing it to continue execution.
//
// Parameters:
//   - id: Unique identifier of the job to resume.
//
// Returns:
//   - An error (ErrJobNotFound, ErrJobNotPausedOrStopped) if the job does not exist,
//     or is in an invalid state for resumption.
//   - nil if the resume operation is successful.
func (p *Pool) ResumeJob(id string) error {
	job, err := p.getJobByID(id)
	if err != nil {
		return err
	}
	return job.Resume(p.Ctx)
}

// StopJob terminates execution of the specified job.
//
// The method triggers immediate job cancellation via its execution context.
//
// Parameters:
//   - id: Unique identifier of the job to stop.
//
// Returns:
//   - An error (ErrJobNotFound) if the specified job does not exist.
//   - nil if the job is successfully stopped.
func (p *Pool) StopJob(id string) error {
	job, err := p.getJobByID(id)
	if err != nil {
		return err
	}

	return job.Stop()
}

// Kill immediately shuts down the entire scheduler pool.
//
// This method:
//   - Cancels the execution context of the pool, stopping all running, waiting, and scheduled jobs.
//   - Updates all jobs' states to Stopped with an ErrPoolShutdown error.
//   - Deletes all jobs from internal storage.
//   - Marks the pool as permanently shut down; it cannot be restarted afterward.
//
// Use this method with caution, as the pool becomes unusable after calling Kill.
func (p *Pool) Kill() {
	p.cancel()
}
