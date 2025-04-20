package job

import (
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// state represents the internal, thread-safe runtime state of a scheduled job.
//
// It manages execution timestamps, status transitions, retry attempts, custom job data,
// and error tracking. All operations are guarded by mutexes to ensure concurrency safety.
type state struct {
	domain.StateDTO // Embedded state DTO for simplified access.
	mu              sync.RWMutex
	currentRetry    int64             // Tracks how many retries have been performed.
	mon             domain.Monitoring // Monitoring implementation for metric tracking.
}

// newState initializes a new state instance with default values and monitoring support.
//
// Parameters:
//   - jobId: Unique identifier for the job.
//   - mon: Monitoring system to which state updates are reported.
//
// Returns:
//   - A pointer to a fully initialized state.
func newState(jobId string, mon domain.Monitoring) *state {
	return &state{
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
// Also sends metrics to the monitoring system after setting the status.
func (s *state) SetStatus(status domain.JobStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.mon.SaveMetrics(s.StateDTO)
}

// GetStatus retrieves the current job status in a thread-safe manner.
func (s *state) GetStatus() domain.JobStatus {
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
func (s *state) TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool {
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

// UpdateExecutionTime calculates and updates execution duration since StartAt.
//
// Also pushes the updated state to the monitoring system.
func (s *state) UpdateExecutionTime() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ExecutionTime = time.Since(s.StartAt).Nanoseconds()
	s.mon.SaveMetrics(s.StateDTO)
	return s.ExecutionTime
}

// UpdateData applies key-value pairs to the job's custom metadata.
//
// Also triggers metric storage after modification.
func (s *state) UpdateData(data map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range data {
		s.Data[k] = v
	}
	s.mon.SaveMetrics(s.StateDTO)
}

// Update applies a partial or full state update from the provided DTO.
//
// In strict mode, all fields are overwritten. In non-strict mode, only non-zero fields are updated.
//
// Parameters:
//   - state: New values to apply to the state.
//   - strict: Whether to overwrite all fields or merge selectively.
func (s *state) Update(state domain.StateDTO, strict bool) {
	s.mu.Lock()
	defer s.mon.SaveMetrics(s.StateDTO)
	defer s.mu.Unlock()

	if strict {
		s.StartAt = state.StartAt
		s.EndAt = state.EndAt
		s.Data = state.Data
		s.ExecutionTime = state.ExecutionTime
		s.Status = state.Status
		s.Error = state.Error
		return
	}

	if !state.Error.IsEmpty() {
		s.Error = state.Error
	}
	if !state.StartAt.IsZero() {
		s.StartAt = state.StartAt
	}
	if !state.EndAt.IsZero() {
		s.EndAt = state.EndAt
	}
	if state.Data != nil {
		s.Data = state.Data
	}
	if state.ExecutionTime > 0 {
		s.ExecutionTime = state.ExecutionTime
	}
	if state.Status != "" {
		s.Status = state.Status
	}
	if !state.NextRun.IsZero() {
		s.NextRun = state.NextRun
	}
	if state.Success > 0 {
		s.Success = state.Success
	}
	if state.Failure > 0 {
		s.Failure = state.Failure
	}
}

// GetState returns a snapshot of the current job state.
//
// Returns:
//   - A pointer to a cloned StateDTO, safe for external read-only access.
func (s *state) GetState() *domain.StateDTO {
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

// SetEndState finalizes the job's execution and updates its result state.
//
// This method:
//   - Calculates final execution time.
//   - Detects invalid status transitions (e.g., if job was not running).
//   - Increments retry or success/failure counters accordingly.
//   - Stores any execution error for diagnostics.
//   - Reports final state to monitoring.
func (s *state) SetEndState(resOnSuccess bool, status domain.JobStatus, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.EndAt = time.Now()
	s.ExecutionTime = time.Since(s.StartAt).Nanoseconds()

	if !(s.Status == domain.Running || s.Status == domain.Waiting) {
		s.Error.JobError = errs.New(errs.ErrJobWrongStatus,
			fmt.Sprintf("wrong status transition from %s to %s", s.Status, status),
		)
		s.Status = domain.Error
		s.mon.SaveMetrics(s.StateDTO)
		return
	}

	s.Status = status

	if err == nil && resOnSuccess {
		s.currentRetry = 0
	}

	s.Error.JobError = err

	if err == nil {
		s.Success++
	} else {
		s.Failure++
	}
	s.mon.SaveMetrics(s.StateDTO)
}
