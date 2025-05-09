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
func NewState(jobId string, mon domain.Monitoring) *State {
	return &State{
		StateDTO: domain.StateDTO{
			JobID:  jobId,
			Status: domain.Waiting,
			Data:   make(map[string]interface{}),
		},
		mon: mon,
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

func (s *State) GetNextRun() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.NextRun
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

	for k, v := range data {
		s.Data[k] = v
	}
	s.mon.SaveMetrics(s.StateDTO)
}

// Update applies a partial or full State update from the provided DTO.
//
// Behavior:
//   - In strict mode: All fields are forcefully overwritten. Metrics are not updated automatically (caller is responsible).
//   - In non-strict mode: Only non-zero or non-empty fields are merged selectively. Metrics are updated immediately.
//
// Parameters:
//   - State: New values to apply to the State.
//   - strict: Whether to fully overwrite (true) or merge selectively (false).
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
		s.NextRun = State.NextRun
		return
	}

	if !State.Error.IsEmpty() {
		s.Error = State.Error
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

// SetEndState finalizes the execution State of a job after it finishes running,
// applying post-execution metadata such as status, error information, and execution duration.
//
// Behavior:
//   - Updates the job’s EndAt timestamp and total ExecutionTime.
//   - Determines the correct final status based on current State:
//   - If status was Running, Waiting, or Paused: sets the new status from input.
//   - If status was Stopped, Ended, or Error: preserves the current status.
//   - For unknown States, applies the provided final status defensively.
//   - Records any job execution error for diagnostics.
//   - Increments Success or Failure counters.
//   - Resets the retry counter if the execution was successful and reset-on-success is enabled.
//   - Saves the final State to the monitoring backend.
//
// Parameters:
//   - resOnSuccess: If true, resets retry counter after a successful execution.
//   - status: The target status to assign if eligible.
//   - err: The execution error encountered, or nil if the job completed successfully.
func (s *State) SetEndState(resOnSuccess bool, status domain.JobStatus, err error) {
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
	s.mon.SaveMetrics(s.StateDTO)
}
