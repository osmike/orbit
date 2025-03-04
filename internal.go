package scheduler

import (
	"context"
	"fmt"
	"time"
)

func (s *Scheduler) sanitizeJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		return fmt.Errorf("job ID is empty")
	}
	for _, _job := range s.Jobs {
		if _job.ID == job.ID {
			return fmt.Errorf("job %v already exists", job.ID)
		}
	}
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

	job.ctx, job.cancel = context.WithCancel(s.ctx)
	job.setStatus(Waiting)
	return nil
}

func (s *Scheduler) executeJob(job *Job) {
	if job.getStatus() == Running {
		return
	}
	job.setStatus(Running)
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("job panicked: %v", r)
			}
		}()
		return job.Fn()
	}()
	if err != nil {
		job.setStatus(Error)
		return
	}
	job.setStatus(Completed)
}

func (s *Scheduler) processJob(job *Job) {
	if job == nil {
		return
	}
	go s.executeJob(job)
	startTime := time.Now()
	updateTicker := time.NewTicker(100 * time.Millisecond)
	defer updateTicker.Stop()

	for {
		select {
		case <-job.ctx.Done():
			return
		case <-updateTicker.C:
			switch job.getStatus() {
			case Running:
				job.State.ExecutionTime = time.Since(startTime)
			case Completed:
				if job.Interval == 0 {
					return
				}
				delay := job.Interval - job.State.ExecutionTime
				if delay < 0 {
					delay = 0
				}
				select {
				case <-time.After(delay):
				case <-job.ctx.Done():
					return
				}
				startTime = time.Now()
				go s.executeJob(job)
			case Error:
				// Заглушка для ретрая
			}
		}
	}
}

func (s *Scheduler) runScheduler() {
	for {
		select {
		case <-s.ctx.Done():
			s.log.Info("Scheduler shutting down")
			return
		case job := <-s.jobChan:
			go s.processJob(job)
		}
	}
}
