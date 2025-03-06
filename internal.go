package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

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
		job.EndAt = time.Date(2050, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	if job.EndAt.Before(job.StartAt) {
		return fmt.Errorf("ending time cannot be before starting time")
	}
	if job.Timeout == 0 {
		job.Timeout = s.cfg.IdleTimeout
	}

	job.ctx, job.cancel = context.WithCancel(s.ctx)
	job.setStatus(Waiting)
	job.pauseCh = make(chan struct{})
	job.resumeCh = make(chan struct{})
	return nil
}

func (s *Scheduler) runScheduler() {
	ticker := time.NewTicker(s.cfg.CheckInterval)
	defer ticker.Stop()
	sem := make(chan struct{}, s.cfg.MaxWorkers)
	var wg sync.WaitGroup

	for {
		select {
		case <-s.ctx.Done():
			wg.Wait()
			s.log.Info("Scheduler shutting down")
			return
		case <-ticker.C:
			s.jobs.Range(func(key, value any) bool {
				job := value.(*Job)
				switch job.getStatus() {
				case Waiting:
					job.exec(&wg, sem)
				case Running:
					job.processRunning()
				case Completed:
					job.processCompleted()
				case Error:
					job.processError()
				}

				return true
			})
		}
	}
}
