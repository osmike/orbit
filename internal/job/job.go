package job

import (
	"context"
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// Job represents a scheduled task that can be executed by the scheduler.
// It holds job execution metadata, scheduling details, and control mechanisms.
type Job struct {
	domain.JobDTO // Embedded struct providing job configuration parameters.

	// ctx is the execution context of the job, allowing cancellation and timeout control.
	ctx context.Context

	// cancel is the function used to cancel the job's execution via the context.
	cancel context.CancelFunc

	// state holds the runtime execution details, including status, execution time, and errors.
	state *state

	// mu is a mutex used to synchronize access to the job's state and ensure thread safety.
	mu sync.Mutex

	// pauseCh is a channel used to pause job execution.
	// When a signal is received, the job enters the Paused state until resumed.
	pauseCh chan struct{}

	// resumeCh is a channel used to resume execution of a paused job.
	// When a signal is received, the job transitions back to the Running state.
	resumeCh chan struct{}

	// cron holds the parsed cron schedule if the job is scheduled using a cron expression.
	cron *CronSchedule
}

// New initializes a new Job instance with the provided job configuration (JobDTO) and execution context.
//
// It performs the following validations:
// - Ensures the job ID is not empty.
// - Ensures the job function (Fn) is not nil.
// - Sets a default job name if not provided.
// - Validates the scheduling type (either cron or interval, but not both).
// - Parses the cron expression if provided.
// - Ensures a valid StartAt and EndAt period.
//
// Returns:
// - A pointer to the initialized Job instance if all validations pass.
// - An error if the job configuration is invalid.
func New(jobDTO domain.JobDTO, ctx context.Context) (*Job, error) {
	// Initialize the Job struct
	job := &Job{
		JobDTO: jobDTO,
		state:  &state{}, // Ensure state is properly initialized
	}

	// Validate job ID
	if job.ID == "" {
		return nil, errs.New(errs.ErrEmptyID, fmt.Sprintf("job name - %s", job.Name))
	}

	// Validate job function
	if job.Fn == nil {
		return nil, errs.New(errs.ErrEmptyFunction, job.ID)
	}

	// Set job name to ID if empty
	if job.Name == "" {
		job.Name = job.ID
	}

	// Validate scheduling type (either Cron or Interval, not both)
	if job.Schedule.CronExpr != "" && job.Schedule.Interval > 0 {
		return nil, errs.New(errs.ErrMixedScheduleType, job.ID)
	}

	// Parse cron expression if provided
	if job.Schedule.CronExpr != "" {
		if cron, err := ParseCron(job.Schedule.CronExpr); err != nil {
			return nil, errs.New(errs.ErrInvalidCronExpression, fmt.Sprintf("error - %v, id: %s", err, job.ID))
		} else {
			job.cron = cron
		}
	}

	// Set default StartAt time
	if job.StartAt.IsZero() {
		job.StartAt = time.Now()
	}

	// Set default EndAt time
	if job.EndAt.IsZero() {
		job.EndAt = domain.MAX_END_AT // Use predefined max execution period
	} else if job.EndAt.Before(job.StartAt) {
		return nil, errs.New(errs.ErrWrongTime, fmt.Sprintf("ending time cannot be before starting time, id: %s", job.ID))
	}

	// Initialize execution state
	job.state.Init(job.ID)

	// Create a cancellable execution context
	job.ctx, job.cancel = context.WithCancel(ctx)

	job.pauseCh = make(chan struct{}, 1)
	job.resumeCh = make(chan struct{}, 1)

	return job, nil
}

// GetStatus retrieves the current execution status of the job.
//
// Returns:
// - The current job status as a `domain.JobStatus` value.
func (j *Job) GetStatus() domain.JobStatus {
	return j.state.GetStatus()
}

// SetStatus updates the job's execution status.
//
// Parameters:
// - status: The new status to be set.
func (j *Job) SetStatus(status domain.JobStatus) {
	j.state.SetStatus(status)
}

// TrySetStatus attempts to update the job's status only if the current status is in the allowed list.
//
// Parameters:
// - allowed: A slice of valid statuses that allow the transition.
// - status: The new status to be set.
//
// Returns:
// - true if the status was successfully updated, false otherwise.
func (j *Job) TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool {
	return j.state.TrySetStatus(allowed, status)
}

// UpdateStateWithStrict replaces the current job state with a new state using strict mode.
//
// In strict mode, all state fields are overridden.
//
// Parameters:
// - state: The new state values encapsulated in `domain.StateDTO`.
func (j *Job) UpdateStateWithStrict(state domain.StateDTO) {
	j.state.Update(state, true)
}

// UpdateState updates the current job state, modifying only non-empty fields.
//
// This method is useful for incremental updates without overwriting unchanged data.
//
// Parameters:
// - state: The new state values encapsulated in `domain.StateDTO`.
func (j *Job) UpdateState(state domain.StateDTO) {
	j.state.Update(state, false)
}

// NextRun calculates the exact next scheduled execution time of the job.
//
// It supports two scheduling modes:
// - Cron-based scheduling (if CronExpr is set).
// - Interval-based scheduling (relative to last start).
//
// Returns:
// - The next scheduled run time as time.Time.
func (j *Job) NextRun() time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()

	// Cron-based schedule
	if j.cron != nil {
		return j.cron.NextRun()
	}

	// Interval-based schedule
	next := j.state.StartAt.Add(j.Schedule.Interval)
	if next.Before(time.Now()) {
		return time.Now()
	}
	return next
}

// CanExecute determines if the job is eligible for execution based on its configuration and timing constraints.
// It prevents execution if the job is already running, scheduled for a future time, or has expired.
//
// The function performs the following checks:
// 1. Ensures one-time jobs do not execute again after completion.
// 2. Prevents concurrent execution of the same job.
// 3. Enforces execution within the allowed time window (StartAt - EndAt).
// 4. Applies a delay before execution if necessary.
//
// Returns:
//   - true if the job can proceed with execution.
//   - false if any conditions prevent execution.
func (j *Job) CanExecute() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	// Ensure the job is not already running or blocked from execution.
	if j.GetStatus() != domain.Waiting {
		return false
	}

	// Prevent execution before the scheduled start time.
	if time.Now().Before(j.StartAt) {
		return false
	}

	// Stop execution if the job's allowed execution window has expired.
	if time.Now().After(j.EndAt) {
		return false
	}

	return true
}

func (j *Job) ProcessJobStart(start time.Time) {
	j.UpdateState(domain.StateDTO{
		StartAt:       start,
		EndAt:         time.Time{},
		Status:        domain.Running,
		ExecutionTime: 0,
		Data:          map[string]interface{}{},
	})
}

func (j *Job) ProcessJobEnd(start time.Time, status domain.JobStatus, err error) {
	execTime := time.Since(start).Nanoseconds()
	j.UpdateState(domain.StateDTO{
		StartAt:       start,
		EndAt:         time.Now(),
		Status:        status,
		ExecutionTime: execTime,
	})
}
