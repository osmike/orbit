package job

import (
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// ProcessStart initializes and updates the job state at the beginning of job execution.
//
// This method performs the following actions:
//   - Records the exact start time of the job.
//   - Resets job state fields (EndAt, ExecutionTime, Data) to their initial defaults.
//   - Sets the job status explicitly to Running.
//   - Initializes a new metadata storage (`sync.Map`) for runtime data.
//   - Immediately saves initial metrics to the provided Monitoring interface.
//
// Parameters:
//   - mon: Monitoring interface instance used for tracking job metrics.
func (j *Job) ProcessStart(mon domain.Monitoring) {
	startTime := time.Now()
	j.UpdateStateStrict(domain.StateDTO{
		StartAt:       startTime,
		EndAt:         time.Time{},
		Status:        domain.Running,
		ExecutionTime: 0,
		Data:          map[string]interface{}{},
	})
	j.ctrl.data = &sync.Map{} // Reset runtime metadata storage.
	j.SaveMetrics(mon)
}

// ProcessEnd finalizes the job state upon job completion.
//
// It performs the following steps:
//   - Updates the job state with the final execution status (e.g., Completed, Error).
//   - Records the job's end timestamp and final execution duration.
//   - Handles retry logic based on the job's Retry settings.
//   - Captures any errors encountered during execution.
//   - Saves final execution metrics via the Monitoring interface.
//
// Parameters:
//   - status: The final execution status of the job.
//   - err: Any error encountered during job execution (nil if successful).
//   - mon: Monitoring interface instance used for final job metrics tracking.
func (j *Job) ProcessEnd(status domain.JobStatus, err error, mon domain.Monitoring) {
	if status == domain.Ended {
		j.CloseChannels()
	}
	j.state.SetEndState(j.JobDTO.Retry.ResetOnSuccess, status, err)
	j.SaveMetrics(mon)
}

// ProcessRun monitors the job's execution duration to ensure it stays within the configured timeout limit.
//
// This method should be invoked periodically during job execution to verify timeout constraints.
//
// It performs the following steps:
//   - Updates the job's current execution duration.
//   - Records interim metrics for monitoring purposes.
//   - Checks if execution duration has exceeded the allowed timeout.
//
// Parameters:
//   - mon: Monitoring interface instance used for real-time job metrics tracking.
//
// Returns:
//   - An ErrJobTimeout error if the job's execution exceeds the allowed timeout.
//   - nil if the execution duration is within acceptable limits.
func (j *Job) ProcessRun(mon domain.Monitoring) error {
	execTime := j.state.UpdateExecutionTime()
	j.SaveMetrics(mon)
	if time.Duration(execTime) > j.Timeout {
		return errs.New(errs.ErrJobTimout, j.ID)
	}
	return nil
}

func (j *Job) ProcessError(mon domain.Monitoring) error {
	err := j.Retry()
	if err != nil {
		j.ProcessEnd(domain.Ended, err, mon)
		j.CloseChannels()
		return err
	}
	j.SetStatus(domain.Completed)
	return nil
}
