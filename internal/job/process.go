package job

import (
	"github.com/osmike/orbit/internal/domain"
	errs "github.com/osmike/orbit/internal/error"
	"time"
)

// ProcessStart initializes the job's internal state at the beginning of execution.
//
// This method performs:
//   - Records the current timestamp as StartAt.
//   - Resets execution-related fields (EndAt, ExecutionTime, Data).
//   - Sets the job status to Running.
//   - Prepares the state for clean metrics collection and data accumulation.
func (j *Job) ProcessStart() {
	startTime := time.Now()

	j.UpdateStateStrict(domain.StateDTO{
		StartAt:       startTime,
		Status:        domain.Running,
		ExecutionTime: 0,
		Data:          map[string]interface{}{},
		NextRun:       j.SetNextRun(startTime),
	})
}

// ProcessEnd finalizes the job after execution, saving timing and status metadata.
//
// This method performs:
//   - Records EndAt timestamp and calculates ExecutionTime.
//   - Sets the final status (Completed, Error, Ended).
//   - Increments Success or Failure counter.
//   - Stores execution error if any.
//   - Triggers graceful cleanup if status is Ended.
//
// Parameters:
//   - status: Final job status (Completed, Error, or Ended).
//   - err: Error encountered during execution, if any.
func (j *Job) ProcessEnd(status domain.JobStatus, err error) {
	if status == domain.Ended {
		j.CloseChannels()
	}
	j.state.SetEndState(j.JobDTO.Retry.ResetOnSuccess, status, err)
}

// ProcessRun monitors job execution time to detect timeouts.
//
// This method should be called during execution (typically inside the pool),
// and ensures the job hasn't exceeded its configured Timeout.
//
// Returns:
//   - ErrJobTimeout if execution exceeds Timeout.
//   - nil if within time limit.
func (j *Job) ProcessRun() error {
	execTime := j.state.UpdateExecutionTime()
	if time.Duration(execTime) > j.Timeout {
		return errs.New(errs.ErrJobTimout, j.ID)
	}
	return nil
}

// ProcessError applies retry logic after a job execution failure.
//
// Behavior:
//   - If retries are available, the job is rescheduled by setting status to Completed.
//   - If retries are exhausted or disabled, the job is finalized (Ended) and removed from the pool.
//     (Note: In this case, execution errors are not stored in the final state.)
//
// Returns:
//   - An error if no more retries are allowed (indicating job termination).
//   - nil if the job will be retried.
func (j *Job) ProcessError() error {
	err := j.Retry()
	if err != nil {
		j.ProcessEnd(domain.Ended, nil)
		return err
	}
	j.SetStatus(domain.Completed)
	return nil
}
