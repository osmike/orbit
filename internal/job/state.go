package job

import (
	"github.com/osmike/orbit/internal/domain"
	"sync"
	"time"
)

// State represents the internal, thread-safe runtime State of a scheduled job.
//
// It manages execution timestamps, status transitions, retry attempts, custom job data,
// and error tracking. All operations are guarded by mutexes to ensure concurrency safety.
type State struct {
	domain.StateDTO // Embedded State DTO for simplified access.
	mu              sync.RWMutex
	currentRetry    int               // Tracks how many retries have been performed.
	mon             domain.Monitoring // Monitoring implementation for metric tracking.
	cron            *CronSchedule
}

// NewState initializes a new job State with default values.
//
// Behavior:
//   - Sets Status to Waiting.
//   - Initializes an empty Data map.
//   - Links the provided Monitoring backend for metric tracking.
//
// Parameters:
//   - jobId: Unique job identifier.
//   - mon: Monitoring implementation for metrics reporting.
//
// Returns:
//   - A pointer to a fully initialized State.
func NewState(jobId string, cron *CronSchedule, mon domain.Monitoring) *State {
	return &State{
		StateDTO: domain.StateDTO{
			JobID:  jobId,
			Status: domain.Waiting,
			Data:   make(map[string]interface{}),
		},
		mon:  mon,
		cron: cron,
	}
}

// SetStatus updates the job's execution status.
//
// Behavior:
//   - Updates the Status field.
//   - Immediately saves the updated State into the Monitoring backend.
func (s *State) SetStatus(status domain.JobStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.mon.SaveMetrics(s.StateDTO)
}

// GetStatus retrieves the current job status in a thread-safe manner.
func (s *State) GetStatus() domain.JobStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

// TrySetStatus attempts to change the status only if the current status is in the allowed list.
//
// Parameters:
//   - allowed: List of acceptable current statuses.
//   - status: New status to set if transition is allowed.
//
// Returns:
//   - true if the status was successfully updated.
func (s *State) TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, allowedStatus := range allowed {
		if s.Status == allowedStatus {
			s.Status = status
			s.mon.SaveMetrics(s.StateDTO)
			return true
		}
	}
	return false
}

// GetNextRun returns the next scheduled run time of the job, based on its current state.
//
// Behavior:
//   - If the job has never been executed (i.e., both Success and Failure counters are zero),
//     the initially assigned NextRun time is returned.
//   - If the job has run at least once but EndAt is still unset, it is treated as a running job,
//     and a far-future date (January 1, 9999) is returned as a sentinel value.
//   - Otherwise, returns the current value of NextRun.
//
// This method uses a read lock to safely access state in concurrent environments.
func (s *State) GetNextRun() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Failure == 0 && s.Success == 0 {
		return s.NextRun
	}
	if s.EndAt.IsZero() {
		return time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	return s.NextRun
}

// SetNextRun updates the job's NextRun field based on the specified scheduling configuration.
//
// Behavior:
//   - If the job is cron-based, computes the next run using the cron schedule and the provided startTime.
//   - If interval-based:
//   - On the first execution (i.e., Success and Failure are zero), NextRun is set to startTime.
//   - On subsequent executions, NextRun is set to EndAt + interval.
//
// Parameters:
//   - startTime: Reference time used for computing the next run (typically StartAt or EndAt).
//   - interval:  Fixed delay to apply when computing the next run for interval-based jobs.
func (s *State) SetNextRun(startTime time.Time, interval time.Duration) {
	if s.cron != nil {
		s.NextRun = s.cron.NextRun(startTime)
		return
	}

	if s.Success == 0 && s.Failure == 0 {
		s.NextRun = startTime
	}
	s.NextRun = s.EndAt.Add(interval)
}

// UpdateExecutionTime calculates and updates execution duration since StartAt.
//
// Also pushes the updated State to the monitoring system.
func (s *State) UpdateExecutionTime() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ExecutionTime = time.Since(s.StartAt).Nanoseconds()
	s.mon.SaveMetrics(s.StateDTO)
	return s.ExecutionTime
}

// UpdateData applies key-value pairs to the job's custom metadata.
//
// Also triggers metric storage after modification.
func (s *State) UpdateData(data map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data == nil {
		s.Data = make(map[string]interface{})
	}
	for k, v := range data {
		s.Data[k] = v
	}
	s.mon.SaveMetrics(s.StateDTO)
}

// Update applies a partial or full update to the job's runtime state using the provided StateDTO.
//
// This method supports two modes:
//   - Strict mode (`strict = true`): All fields in the current state are forcibly overwritten
//     with the values from the provided StateDTO, regardless of whether they are zero-valued.
//     Metrics (e.g., Success, Failure) are NOT automatically recorded in this mode.
//   - Non-strict mode (`strict = false`): Only non-zero or non-nil fields from the DTO are merged
//     into the existing state. This allows incremental updates without wiping unrelated values.
//     Metrics are saved immediately after the merge.
//
// Error handling:
//   - JobError and HookError fields are merged independently.
//   - Individual hook errors (e.g., OnStart, OnError) are only updated if non-nil.
//   - The state lock ensures thread-safe updates.
//
// Parameters:
//   - State: The StateDTO containing new values to apply.
//   - strict: Whether to overwrite all fields (`true`) or only merge meaningful changes (`false`).
func (s *State) Update(State domain.StateDTO, strict bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strict {
		s.StartAt = State.StartAt
		s.EndAt = State.EndAt
		s.Data = State.Data
		s.ExecutionTime = State.ExecutionTime
		s.Status = State.Status
		s.Error = State.Error
		return
	}
	if State.Error.JobError != nil {
		s.Error.JobError = State.Error.JobError
	}
	if State.Error.HookError.OnStart != nil {
		s.Error.HookError.OnStart = State.Error.HookError.OnStart
	}
	if State.Error.HookError.OnStop != nil {
		s.Error.HookError.OnStop = State.Error.HookError.OnStop
	}
	if State.Error.HookError.OnError != nil {
		s.Error.HookError.OnError = State.Error.HookError.OnError
	}
	if State.Error.HookError.OnSuccess != nil {
		s.Error.HookError.OnSuccess = State.Error.HookError.OnSuccess
	}
	if State.Error.HookError.OnPause != nil {
		s.Error.HookError.OnPause = State.Error.HookError.OnPause
	}
	if State.Error.HookError.OnResume != nil {
		s.Error.HookError.OnResume = State.Error.HookError.OnResume
	}
	if State.Error.HookError.Finally != nil {
		s.Error.HookError.Finally = State.Error.HookError.Finally
	}

	if !State.StartAt.IsZero() {
		s.StartAt = State.StartAt
	}
	if !State.EndAt.IsZero() {
		s.EndAt = State.EndAt
	}
	if State.Data != nil {
		s.Data = State.Data
	}
	if State.ExecutionTime > 0 {
		s.ExecutionTime = State.ExecutionTime
	}
	if State.Status != "" {
		s.Status = State.Status
	}
	if !State.NextRun.IsZero() {
		s.NextRun = State.NextRun
	}
	if State.Success > 0 {
		s.Success = State.Success
	}
	if State.Failure > 0 {
		s.Failure = State.Failure
	}
	s.mon.SaveMetrics(s.StateDTO)
}

// GetState returns the current job State.
//
// Warning:
//   - The returned pointer refers to the internal State (not a deep clone).
//   - External code must treat this object as read-only to avoid race conditions.
//
// Returns:
//   - A pointer to the current StateDTO.
func (s *State) GetState() *domain.StateDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &domain.StateDTO{
		JobID:         s.JobID,
		StartAt:       s.StartAt,
		EndAt:         s.EndAt,
		Error:         s.Error,
		Status:        s.Status,
		ExecutionTime: s.ExecutionTime,
		Data:          s.Data,
		Success:       s.Success,
		Failure:       s.Failure,
		NextRun:       s.NextRun,
	}
}

// SetEndState finalizes the job's runtime state after an execution attempt completes.
//
// This method records execution metadata such as end time, duration, outcome,
// retry counters, and the next scheduled run. It also determines whether to update
// the current job status or preserve it based on the prior lifecycle state.
//
// Behavior:
//   - Sets EndAt to the current time.
//   - Calculates ExecutionTime as the duration since StartAt.
//   - If current status is Paused, Running, or Waiting: overrides with the provided `status` and sets JobError.
//   - If current status is Stopped, Ended, or Error: preserves existing status and skips JobError update.
//   - Increments Success or Failure counters based on the presence of `err`.
//   - Resets retry counter if execution was successful and `resOnSuccess` is true.
//   - Computes and sets the next run time using the provided `interval`.
//   - Saves the updated state to the monitoring system.
//
// Parameters:
//   - resOnSuccess: If true, resets the retry counter on success.
//   - status: The target status to apply if the current state allows it.
//   - err: The error returned by job execution, or nil on success.
//   - interval: The repeat interval used to schedule the next execution.
func (s *State) SetEndState(resOnSuccess bool, status domain.JobStatus, err error, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.EndAt = time.Now()
	s.ExecutionTime = time.Since(s.StartAt).Nanoseconds()

	switch s.Status {
	case domain.Paused, domain.Running, domain.Waiting:
		s.Error.JobError = err
		s.Status = status
	case domain.Stopped, domain.Ended, domain.Error:
		// Preserve current status (no override)
	default:
		// Fallback just in case of unknown value
		s.Status = status
	}

	if err == nil && resOnSuccess {
		s.currentRetry = 0
	}

	if err == nil {
		s.Success++
	} else {
		s.Failure++
	}
	s.SetNextRun(s.EndAt, interval)
	s.mon.SaveMetrics(s.StateDTO)
}
