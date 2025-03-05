package scheduler

import (
	"context"
	"fmt"
	"time"
)

func (s *Scheduler) sanitizeJob(job *Job) error {
	job.mu.Lock()
	defer job.mu.Unlock()

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
	if !job.tryChangeStatus([]JobStatus{Waiting, Stopped, Completed}, Running) {
		return
	}
	startTime := time.Now()
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
	execTime := time.Since(startTime).Nanoseconds()
	job.State.ExecutionTime.Store(execTime)
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
				job.State.ExecutionTime.Store(time.Since(startTime).Nanoseconds())
			case Completed:
				execTime := job.State.ExecutionTime.Load()
				if job.Interval == 0 {
					return
				}
				delay := job.Interval - time.Duration(execTime)
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
	sem := make(chan struct{}, s.cfg.MaxWorkers)
	for {
		select {
		case <-s.ctx.Done():
			s.log.Info("Scheduler shutting down")
			return
		case job, ok := <-s.jobChan:
			if !ok {
				return
			}
			go func(j *Job) {
				sem <- struct{}{}
				defer func() { <-sem }()
				s.processJob(j)
			}(job)
		}
	}
}

func (s *Scheduler) executeJobs() {
	for _, job := range s.Jobs {
		select {
		case err := <-errChan:
			execTime := time.Since(startTime).Nanoseconds()
			job.State.ExecutionTime.Store(execTime)

			if err != nil {
				job.setStatus(Error)
			} else {
				job.setStatus(Completed)
			}

			if job.Interval > 0 {
				delay := job.Interval - time.Duration(execTime)
				if delay < 0 {
					delay = 0
				}
				select {
				case <-time.After(delay):
				case <-job.ctx.Done():
					return
				}
				s.executeJob(job)
			}
			return
		}
	}
}
