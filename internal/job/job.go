package job

import (
	"context"
	"errors"
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// Job represents a scheduled task managed and executed by the scheduler.
//
// It encapsulates the execution logic, scheduling parameters, lifecycle control,
// runtime state, and associated metadata.
type Job struct {
	domain.JobDTO // Embedded configuration data provided at registration.

	PoolID string

	ctx    context.Context    // Execution context for cancellation and timeouts.
	cancel context.CancelFunc // Function to cancel execution.

	state *state       // Internal state tracking execution progress and metadata.
	mu    sync.RWMutex // Protects concurrent access to mutable fields.

	pauseCh  chan struct{} // Channel for signaling pause events.
	resumeCh chan struct{} // Channel for signaling resume events.

	doneCh chan struct{} // Channel indicating job availability for execution.

	cron *CronSchedule // Parsed cron expression (if provided).

	//currentRetry int        // Current retry attempt number.
	ctrl *FnControl // Control interface passed to the job's main function.

	processCh chan struct{}
}

// New creates and initializes a new Job instance based on the provided configuration and context.
//
// Validation & Initialization steps:
//   - Ensures required fields (ID, Fn) are present.
//   - Defaults job Name to ID if empty.
//   - Ensures mutually exclusive scheduling fields (Interval vs Cron).
//   - Parses and validates cron expressions (if present).
//   - Initializes timing bounds (StartAt, EndAt) and internal state.
//
// Parameters:
//   - jobDTO: Job configuration object.
//   - ctx: Parent context for job lifecycle.
//
// Returns:
//   - A pointer to the created Job instance.
//   - An error if validation fails.
func New(poolID string, jobDTO domain.JobDTO, ctx context.Context) (*Job, error) {
	job := &Job{
		JobDTO: jobDTO,
	}
	if poolID == "" {
		return nil, errs.New(errs.ErrEmptyID, fmt.Sprintf("pool id on job id - %s, job name- %s is empty", job.ID, job.Name))
	}
	job.PoolID = poolID
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
	nextRun := time.Now()
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

	job.state = newState(job.ID)
	job.state.NextRun = nextRun
	job.ctx, job.cancel = context.WithCancel(ctx)

	job.pauseCh = make(chan struct{}, 1)
	job.resumeCh = make(chan struct{}, 1)
	job.doneCh = make(chan struct{}, 1)
	job.processCh = make(chan struct{}, 1)

	job.doneCh <- struct{}{}

	job.ctrl = &FnControl{
		ctx:        job.ctx,
		data:       &sync.Map{},
		pauseChan:  job.pauseCh,
		resumeChan: job.resumeCh,
	}

	return job, nil
}

// GetMetadata returns the job's immutable configuration metadata.
//
// Returns:
//   - The original JobDTO used to define the job.
func (j *Job) GetMetadata() domain.JobDTO {
	return j.JobDTO
}

// GetState returns the current state snapshot of the job.
//
// Returns:
//   - A pointer to the StateDTO containing runtime execution metadata.
func (j *Job) GetState() *domain.StateDTO {
	return j.state.GetState()
}

// GetStatus retrieves the current execution status of the job.
//
// Returns:
//   - The JobStatus (e.g., Waiting, Running, Completed, etc.).
func (j *Job) GetStatus() domain.JobStatus {
	return j.state.GetStatus()
}

// SetStatus sets the job’s current execution status explicitly.
//
// Parameters:
//   - status: The new status to assign.
func (j *Job) SetStatus(status domain.JobStatus) {
	j.state.SetStatus(status)
}

func (j *Job) LockJob() bool {
	select {
	case j.processCh <- struct{}{}:
		return true
	default:
		return false // already locked
	}
}

func (j *Job) UnlockJob() {
	select {
	case <-j.processCh:
	default:
		// do nothing if already unlocked
	}
}

// TrySetStatus attempts to change the job's status if its current status matches one of the allowed values.
//
// Parameters:
//   - allowed: List of allowed current statuses.
//   - status: Desired new status.
//
// Returns:
//   - true if the transition was successful; false otherwise.
func (j *Job) TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool {
	return j.state.TrySetStatus(allowed, status)
}

// UpdateState performs a partial update of the job’s internal state.
//
// Fields in the provided StateDTO will only be applied if non-zero (e.g., nil errors are ignored).
//
// Parameters:
//   - state: Partial state update.
func (j *Job) UpdateState(state domain.StateDTO) {
	j.state.Update(state, false)
}

// UpdateStateStrict forcefully overwrites the job’s current state with all fields from the given StateDTO.
//
// This is useful for full-state replacements during restoration or forced overrides.
//
// Parameters:
//   - state: Complete state object to replace the current one.
func (j *Job) UpdateStateStrict(state domain.StateDTO) {
	j.state.Update(state, true)
}

// SaveUserDataToState transfers all runtime key-value data stored during execution
// (via FnControl.SaveData) into the job state for later retrieval or monitoring.
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

// SaveMetrics captures the job’s current runtime state and sends it to the provided Monitoring interface.
//
// Parameters:
//   - mon: Monitoring interface that stores runtime metrics externally.
func (j *Job) SaveMetrics(mon domain.Monitoring) {
	j.SaveUserDataToState()
	s := j.GetState()
	mon.SaveMetrics(*s)
}

// SetTimeout updates the maximum allowed execution duration for the job.
//
// If the job exceeds this timeout, it may be forcefully stopped by the scheduler.
//
// Parameters:
//   - timeout: Maximum allowed execution duration.
func (j *Job) SetTimeout(timeout time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Timeout = timeout
}

// handleError processes a given error by categorizing it into hook-specific or execution-level error types,
// and updating the job’s state accordingly.
//
// Parameters:
//   - err: The error encountered during execution or hook invocation.
func (j *Job) handleError(err error) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, errs.ErrOnStartHook):
		j.UpdateState(domain.StateDTO{
			Error: domain.StateError{
				HookError: domain.HookError{OnStart: err},
			},
		})
	case errors.Is(err, errs.ErrOnStopHook):
		j.UpdateState(domain.StateDTO{
			Error: domain.StateError{
				HookError: domain.HookError{OnStop: err},
			},
		})
	case errors.Is(err, errs.ErrOnErrorHook):
		j.UpdateState(domain.StateDTO{
			Error: domain.StateError{
				HookError: domain.HookError{OnError: err},
			},
		})
	case errors.Is(err, errs.ErrOnSuccessHook):
		j.UpdateState(domain.StateDTO{
			Error: domain.StateError{
				HookError: domain.HookError{OnSuccess: err},
			},
		})
	case errors.Is(err, errs.ErrOnResumeHook):
		j.UpdateState(domain.StateDTO{
			Error: domain.StateError{
				HookError: domain.HookError{OnResume: err},
			},
		})
	case errors.Is(err, errs.ErrOnPauseHook):
		j.UpdateState(domain.StateDTO{
			Error: domain.StateError{
				HookError: domain.HookError{OnPause: err},
			},
		})
	case errors.Is(err, errs.ErrFinallyHook):
		j.UpdateState(domain.StateDTO{
			Error: domain.StateError{
				HookError: domain.HookError{Finally: err},
			},
		})
	default:
		j.UpdateState(domain.StateDTO{
			Error: domain.StateError{
				JobError: err,
			},
		})
	}
}
