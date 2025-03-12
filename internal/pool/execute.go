package pool

import (
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

func (p *Pool) Execute(job Job, sem chan struct{}, wg *sync.WaitGroup) error {
	wg.Add(1)
	defer wg.Done()
	if !job.CanExecute() {
		return nil
	}
	meta := job.GetMetadata()
	go func() {
		// Acquire a semaphore slot to enforce max worker constraints.
		sem <- struct{}{}
		defer func() { <-sem }() // Release the semaphore after execution.
		startTime := time.Now()
		job.UpdateState(domain.StateDTO{StartAt: startTime})
		// Handle panic recovery to prevent the scheduler from crashing due to job failures.
		defer func() {
			if r := recover(); r != nil {
				job.SetStatus(domain.Error)
				job.UpdateState(domain.StateDTO{
					Error:         errs.New(errs.ErrJobPanicked, fmt.Sprintf("%v, id: %s", r, meta.ID)),
					ExecutionTime: time.Since(startTime).Nanoseconds(),
				})
			}
		}()

	}()
	return nil
}
