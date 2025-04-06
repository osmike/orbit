package job

import (
	"context"
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// Job represents a scheduled task managed and executed by the scheduler.
//
// It encapsulates execution logic, scheduling details, lifecycle control,
// runtime state, and job metadata.
type Job struct {
	domain.JobDTO // Embedded job configuration parameters.

	ctx    context.Context    // Execution context for cancellation and timeouts.
	cancel context.CancelFunc // Function to explicitly cancel job execution.

	state *state     // Runtime job execution state (status, errors, execution time).
	mu    sync.Mutex // Mutex to ensure thread-safe state manipulation.

	pauseCh  chan struct{} // Channel to signal job pause requests.
	resumeCh chan struct{} // Channel to signal job resume requests.

	doneCh chan struct{} // Channel signaling completion of job execution.

	cron *CronSchedule // Parsed cron schedule if job is cron-based.

	currentRetry int        // Number of retries attempted after job failures.
	ctrl         *FnControl // Control interface passed to the job's main function.
}

// New creates and initializes a Job instance from the provided configuration and execution context.
//
// Performs validation and default value initialization:
//   - Ensures Job ID and function are provided.
//   - Sets Job Name to Job ID if not specified.
//   - Verifies scheduling parameters (interval or cron, but not both).
//   - Parses and validates cron expressions.
//   - Initializes default StartAt and EndAt times.
//   - Sets up the execution context with cancellation support.
//
// Parameters:
//   - jobDTO: Job configuration details.
//   - ctx: Execution context, allowing external cancellation control.
//
// Returns:
//   - A pointer to a fully initialized Job.
//   - An error if provided configuration parameters are invalid or incomplete.
func New(jobDTO domain.JobDTO, ctx context.Context) (*Job, error) {
	job := &Job{
		JobDTO: jobDTO,
		state:  &state{},
	}

	if job.ID == "" {
		return nil, errs.New(errs.ErrEmptyID, fmt.Sprintf("job name - %s", job.Name))
	}

	if job.Fn == nil {
		return nil, errs.New(errs.ErrEmptyFunction, job.ID)
	}

	if job.Name == "" {
		job.Name = job.ID
	}

	if job.Interval.CronExpr != "" && job.Interval.Time > 0 {
		return nil, errs.New(errs.ErrMixedScheduleType, job.ID)
	}

	if job.Interval.CronExpr != "" {
		cron, err := ParseCron(job.Interval.CronExpr)
		if err != nil {
			return nil, errs.New(errs.ErrInvalidCronExpression, fmt.Sprintf("error - %v, id: %s", err, job.ID))
		}
		job.cron = cron
	}

	if job.StartAt.IsZero() {
		job.StartAt = time.Now()
	}

	if job.EndAt.IsZero() {
		job.EndAt = domain.MAX_END_AT
	} else if job.EndAt.Before(job.StartAt) {
		return nil, errs.New(errs.ErrWrongTime, fmt.Sprintf("ending time cannot be before starting time, id: %s", job.ID))
	}

	job.state = job.state.Init(job.ID)
	job.ctx, job.cancel = context.WithCancel(ctx)

	job.pauseCh = make(chan struct{}, 1)
	job.resumeCh = make(chan struct{}, 1)
	job.doneCh = make(chan struct{}, 1)
	job.doneCh <- struct{}{}

	job.ctrl = &FnControl{
		ctx:        job.ctx,
		data:       &sync.Map{},
		pauseChan:  job.pauseCh,
		resumeChan: job.resumeCh,
	}

	return job, nil
}

// GetMetadata returns a copy of the job's configuration metadata.
//
// Returns:
//   - domain.JobDTO containing configuration details.
func (j *Job) GetMetadata() domain.JobDTO {
	return j.JobDTO
}

// GetState returns the current execution state of the job.
//
// Returns:
//   - domain.StateDTO containing current state details.
func (j *Job) GetState() domain.StateDTO {
	return j.state.GetState()
}

// GetStatus retrieves the job's current execution status.
//
// Returns:
//   - Current job status (e.g., Waiting, Running, Completed, etc.).
func (j *Job) GetStatus() domain.JobStatus {
	return j.state.GetStatus()
}

// SetStatus explicitly sets the job's execution status.
//
// Parameters:
//   - status: New job execution status to apply.
func (j *Job) SetStatus(status domain.JobStatus) {
	j.state.SetStatus(status)
}

// TrySetStatus attempts to set a new job status if current status matches any allowed states.
//
// Parameters:
//   - allowed: Slice of allowed current statuses for the transition.
//   - status: Desired new status.
//
// Returns:
//   - true if the status update was successful; false otherwise.
func (j *Job) TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool {
	return j.state.TrySetStatus(allowed, status)
}

// UpdateState updates the current state, applying only non-zero values from the provided state.
//
// Useful for incremental updates without overwriting unchanged state data.
//
// Parameters:
//   - state: domain.StateDTO containing state updates.
func (j *Job) UpdateState(state domain.StateDTO) {
	j.state.Update(state)
}

// SaveUserDataToState transfers stored user data from FnControl to the job state,
// making it available for external monitoring or reporting purposes.
func (j *Job) SaveUserDataToState() {
	result := make(map[string]interface{})
	j.ctrl.data.Range(func(key, value interface{}) bool {
		result[key.(string)] = value
		return true
	})
	j.UpdateState(domain.StateDTO{
		Data: result,
	})
}

// SaveMetrics records job execution metrics using the provided Monitoring interface.
//
// This method ensures job-specific data is saved to the state and then passed to the monitoring system.
//
// Parameters:
//   - mon: Monitoring interface responsible for persisting job metrics.
func (j *Job) SaveMetrics(mon domain.Monitoring) {
	j.SaveUserDataToState()
	state := j.GetState()
	mon.SaveMetrics(state)
}
