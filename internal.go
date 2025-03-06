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
	_, ok := s.jobs.Load(job.ID)
	if ok {
		return fmt.Errorf("job with ID %s already exists", job.ID)
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

func (s *Scheduler) runScheduler() {
	go s.process()
}

func (s *Scheduler) process() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	sem := make(chan struct{}, s.cfg.MaxWorkers)

	for {
		select {
		case <-s.ctx.Done():
			s.log.Info("Scheduler shutting down")
			return
		case <-ticker.C:
			s.jobs.Range(func(key, value any) bool {
				job := value.(*Job)
				switch job.getStatus() {
				case Waiting:
					go func() {
						sem <- struct{}{}
						defer func() { <-sem }()

						job.setStatus(Running)
						startTime := time.Now()

						defer func() {
							if r := recover(); r != nil {
								job.setStatus(Error)
								fmt.Printf("Job panicked: %v\n", r)
							}
						}()

						err := job.Fn()
						job.State.ExecutionTime = time.Since(startTime).Nanoseconds()

						if err != nil {
							job.setStatus(Error)
						} else {
							job.setStatus(Completed)
						}
					}()

				case Running:
					job.State.ExecutionTime = time.Since(job.StartAt).Nanoseconds()

				case Completed:
					delay := job.Interval - time.Duration(job.State.ExecutionTime)
					if delay < 0 {
						delay = 0
					}
					time.AfterFunc(delay, func() {
						val, ok := s.jobs.Load(job.ID)
						if ok {
							job := val.(*Job)
							job.tryChangeStatus([]JobStatus{Completed}, Waiting)
						}
					})
				}
				return true
			})
		}
	}
}
