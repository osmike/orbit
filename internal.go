package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// sanitizeJob validates and initializes a job before adding it to the scheduler.
//
// This function ensures that job parameters are correctly set and do not
// violate any logical constraints (e.g., end time before start time).
// It also initializes necessary fields such as context and control channels.
//
// Parameters:
// - job: A pointer to the Job struct to be validated and initialized.
//
// Returns:
// - An error if the job fails validation (e.g., missing ID, function, or incorrect time settings).
func (s *Scheduler) sanitizeJob(job *Job) error {
	job.mu.Lock()
	defer job.mu.Unlock()

	if job.ID == "" {
		return fmt.Errorf("job ID is empty")
	}
	_, ok := s.jobs.Load(job.ID)
	if ok {
		return fmt.Errorf("job with ID %s already exists", job.ID)
	}
	job.State.JobID = job.ID
	if job.Fn == nil {
		return fmt.Errorf("job function is empty")
	}
	if job.Name == "" {
		job.Name = job.ID
	}
	if job.StartAt.IsZero() {
		job.StartAt = time.Now()
	}
	if job.EndAt.IsZero() {
		job.EndAt = MAX_END_AT // Predefined constant for the maximum job execution period
	}
	if job.EndAt.Before(job.StartAt) {
		return fmt.Errorf("ending time cannot be before starting time")
	}
	if job.Timeout == 0 {
		job.Timeout = s.cfg.IdleTimeout
	}

	// Initialize job execution control mechanisms
	job.ctx, job.cancel = context.WithCancel(s.ctx)
	job.setStatus(Waiting)
	job.pauseCh = make(chan struct{})
	job.resumeCh = make(chan struct{})
	return nil
}

// runScheduler continuously processes jobs in the scheduler.
//
// This function runs an infinite loop that periodically checks all jobs
// and updates their execution state based on their current status.
// It ensures that jobs are executed according to their schedule and handles retries or errors.
//
// The function gracefully shuts down when the scheduler context is canceled.
//
// It uses:
// - A ticker to periodically scan jobs based on CheckInterval.
// - A semaphore (channel) to limit the number of concurrently running jobs.
// - A wait group to ensure all jobs complete before shutdown.
func (s *Scheduler) runScheduler() {
	ticker := time.NewTicker(s.cfg.CheckInterval)
	defer ticker.Stop()
	sem := make(chan struct{}, s.cfg.MaxWorkers)
	var wg sync.WaitGroup

	for {
		select {
		case <-s.ctx.Done(): // Handle graceful shutdown
			wg.Wait()
			s.log.Info("Scheduler shutting down")
			return
		case <-ticker.C: // Periodically process jobs
			s.jobs.Range(func(key, value any) bool {
				job := value.(*Job)
				switch job.getStatus() {
				case Waiting:
					job.execute(&wg, sem) // Start the job execution
				case Running:
					job.processRunning() // Update execution time and monitor for timeouts
				case Completed:
					job.processCompleted() // Handle post-execution logic
				case Error:
					job.processError() // Handle retries or logging of failed jobs
				}
				return true
			})
		}
	}
}
