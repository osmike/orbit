// Package job implements the core logic for scheduled task execution within the orbit library.
//
// A Job encapsulates everything required to define, execute, and monitor a scheduled task.
// It supports interval-based and cron-based execution, lifecycle control (pause, resume, stop, retry),
// runtime state tracking, and lifecycle hooks (OnStart, OnSuccess, OnError, Finally, etc.).
//
// Each Job exposes:
//   - Fn: the main execution function of the task.
//   - FnControl: a runtime context object passed to Fn and all lifecycle hooks,
//     allowing cancellation awareness, pause/resume coordination, custom metadata storage (SaveData),
//     and now full state access (via GetData).
//   - StateDTO: internal immutable state representation capturing execution metadata
//     such as start/end time, errors, current status, success/failure counters, custom user data, and next run time.
//   - Monitoring: interface for real-time metric collection and persistence.
//
// Key Features of the job package:
//   - Full lifecycle execution with structured state mutation.
//   - Retry mechanism with configurable reset-on-success behavior.
//   - Thread-safe state access through fine-grained locking.
//   - Isolation between business logic (Fn) and internal state management.
//   - State inspection from within job logic and hooks via FnControl.GetData().
//   - Defensive execution with panic recovery, status protection, and data consistency.
//
// Jobs are created via the New function and registered into execution pools by the scheduler layer.
// Once started, each job's behavior is controlled by interval/cron settings and monitored over its lifetime.

package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"orbit/internal/domain"
	errs "orbit/internal/error"
	"sync"
	"time"
)

// Job represents a scheduled task managed and executed by the scheduler.
//
// It encapsulates the job's execution function (Fn), configuration, lifecycle state,
// context cancellation, pause/resume signaling, retry logic, and runtime metrics.
type Job struct {
	domain.JobDTO // Embedded job configuration provided during creation.

	PoolID string // Optional: pool identifier if needed for grouping or routing.

	ctx    context.Context    // Job-scoped context for cancellation and timeout control.
	cancel context.CancelFunc // Cancels the job's context.

	mon domain.Monitoring // Monitoring interface for metrics tracking.

	state *state       // Internal runtime state tracking job lifecycle and metrics.
	mu    sync.RWMutex // Guards mutable configuration fields (e.g., Timeout).

	pauseCh  chan struct{} // Signals the job to pause execution.
	resumeCh chan struct{} // Signals the job to resume from pause.

	doneCh    chan struct{} // Used as a semaphore to prevent overlapping executions.
	processCh chan struct{} // Guards against duplicate concurrent lifecycle transitions.

	cron *CronSchedule // Parsed cron expression if applicable.

	ctrl *FnControl // Runtime control interface passed into the job's execution function.
}

// New creates and initializes a new Job instance from the given JobDTO and context.
//
// Steps:
//   - Validates configuration (ID, Fn, scheduling exclusivity).
//   - Sets default values for Name, StartAt, and EndAt.
//   - Parses Cron expression if provided.
//   - Initializes state, channels, and internal control object.
//
// Parameters:
//   - jobDTO: Configuration settings for the job.
//   - ctx: Parent context inherited from the scheduler.
//   - mon: Monitoring implementation used for metrics reporting.
//
// Returns:
//   - Pointer to a new Job instance.
//   - Error if validation fails or initialization is inconsistent.
func New(jobDTO domain.JobDTO, ctx context.Context, mon domain.Monitoring) (*Job, error) {
	job := &Job{
		JobDTO: jobDTO,
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

	job.mon = mon
	job.state = newState(job.ID, job.mon)
	job.state.NextRun = nextRun
	job.ctx, job.cancel = context.WithCancel(ctx)

	job.pauseCh = make(chan struct{}, 1)
	job.resumeCh = make(chan struct{}, 1)
	job.doneCh = make(chan struct{}, 1)
	job.processCh = make(chan struct{}, 1)

	job.doneCh <- struct{}{}

	job.ctrl = &FnControl{
		ctx:        job.ctx,
		pauseChan:  job.pauseCh,
		resumeChan: job.resumeCh,
		saveData:   job.state.UpdateData,
		getData:    job.GetStateCopy,
	}

	return job, nil
}

// GetMetadata returns the original job configuration (JobDTO).
func (j *Job) GetMetadata() domain.JobDTO {
	return j.JobDTO
}

// GetState returns a snapshot of the job's current execution state.
func (j *Job) GetState() *domain.StateDTO {
	return j.state.GetState()
}

// GetStateCopy returns a deep copy of the current job execution state.
//
// This method ensures that the returned StateDTO contains an isolated copy
// of the Data field, which prevents accidental external mutations of the internal state.
//
// Returns:
//   - A fully copied StateDTO instance representing the latest known state of the job.
//   - An error if deep-copying the Data map fails due to serialization issues (e.g. unsupported types).
func (j *Job) GetStateCopy() (res domain.StateDTO, err error) {
	st := j.state.GetState()

	bytes, err := json.Marshal(st.Data)
	if err != nil {
		return res, errs.New(errs.ErrMarshalStateData, fmt.Sprintf("error - %v, data - %v", err, st.Data))
	}

	var cpData map[string]interface{}
	if err = json.Unmarshal(bytes, &cpData); err != nil {
		return res, errs.New(errs.ErrUnmarshalStateData, fmt.Sprintf("error - %v, data - %v", err, st.Data))
	}

	res = domain.StateDTO{
		JobID:         st.JobID,
		StartAt:       st.StartAt,
		EndAt:         st.EndAt,
		Error:         st.Error,
		Status:        st.Status,
		ExecutionTime: st.ExecutionTime,
		Data:          cpData,
		Success:       st.Success,
		Failure:       st.Failure,
		NextRun:       st.NextRun,
	}

	return res, nil
}

// GetStatus retrieves the job's current status (e.g., Waiting, Running).
func (j *Job) GetStatus() domain.JobStatus {
	return j.state.GetStatus()
}

// SetStatus forcefully sets the job's current status.
func (j *Job) SetStatus(status domain.JobStatus) {
	j.state.SetStatus(status)
}

// TrySetStatus attempts to transition to a new status if the current status
// is within the allowed list. Returns true if the transition succeeds.
func (j *Job) TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool {
	return j.state.TrySetStatus(allowed, status)
}

// UpdateState applies a partial update to the job's internal state.
// Only non-zero/non-nil fields in the provided StateDTO will be applied.
func (j *Job) UpdateState(state domain.StateDTO) {
	j.state.Update(state, false)
}

// UpdateStateStrict performs a full overwrite of the job's current state
// using all fields from the provided StateDTO.
func (j *Job) UpdateStateStrict(state domain.StateDTO) {
	j.state.Update(state, true)
}

// SetTimeout updates the job's execution timeout.
// If exceeded, the job may be canceled during ProcessRun.
func (j *Job) SetTimeout(timeout time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Timeout = timeout
}

// LockJob attempts to acquire an internal processing lock for lifecycle transitions.
// Returns true if lock was acquired.
func (j *Job) LockJob() bool {
	select {
	case j.processCh <- struct{}{}:
		return true
	default:
		return false
	}
}

// UnlockJob releases the internal lifecycle lock.
func (j *Job) UnlockJob() {
	select {
	case <-j.processCh:
	default:
		// Already unlocked
	}
}

// handleError stores the given error in the appropriate part of the job state,
// depending on whether it originates from a hook or general execution failure.
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
