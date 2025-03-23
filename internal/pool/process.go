package pool

import (
	"go-scheduler/internal/domain"
	"sync"
	"time"
)

// processWaiting checks if a job in the "Waiting" state is ready to execute.
//
// If the current time is past the job's scheduled next run time,
// the job is dispatched for execution.
//
// Parameters:
//   - job: The Job instance currently in the Waiting state.
//   - sem: Semaphore used to limit concurrent execution based on Pool configuration.
//   - wg: WaitGroup to synchronize the execution lifecycle of active jobs.
func (p *Pool) processWaiting(job Job, sem chan struct{}, wg *sync.WaitGroup) {
	if time.Now().After(job.NextRun()) {
		p.execute(job, sem, wg)
	}
}

// processRunning monitors a job currently in the "Running" state,
// checking for execution timeouts or runtime errors.
//
// If the job exceeds its configured timeout, it is marked as Error,
// triggering its finalization and metric recording.
//
// Parameters:
//   - job: The Job instance currently executing.
func (p *Pool) processRunning(job Job) {
	err := job.ProcessRun(p.mon)
	if err != nil {
		job.ProcessEnd(domain.Error, err, p.mon)
	}
}

// processCompleted handles the state of a job marked as "Completed".
//
// It checks if the job has future scheduled executions. If another execution
// is pending, the job state is reset to "Waiting". Otherwise, the job is marked
// as "Ended", indicating no further executions are planned.
//
// Parameters:
//   - job: The Job instance that has completed its execution.
func (p *Pool) processCompleted(job Job) {
	if job.NextRun().After(time.Now()) {
		job.UpdateState(domain.StateDTO{
			Status: domain.Waiting,
		})
		return
	}
	job.UpdateState(domain.StateDTO{
		Status: domain.Ended,
	})
}

// processError manages jobs that have encountered an error during execution.
//
// It attempts to retry the job execution if retries are still allowed. If the retry
// limit is reached, no further action is taken. Otherwise, the job state is reset
// to "Waiting", scheduling it for the next execution attempt.
//
// Parameters:
//   - job: The Job instance currently in an Error state.
func (p *Pool) processError(job Job) {
	err := job.Retry()
	if err != nil {
		return
	}
	job.UpdateState(domain.StateDTO{
		Status: domain.Waiting,
	})
}
