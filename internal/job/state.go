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
// It captures and synchronizes key execution metadata, such as start and end timestamps,
// status, execution time, errors, user-defined data, and retry attempts.
// This struct is not exposed directly outside the job package.
type state struct {
	domain.StateDTO // Embedded data transfer object for simplified state export.

	mu sync.RWMutex // Guards all state fields for concurrent access.

	currentRetry int64 // Tracks the number of retry attempts made for this job.
}

// newState initializes and returns a new job execution state instance.
//
// Parameters:
//   - id: The unique job identifier to associate with the state.
//
// Returns:
//   - A pointer to the initialized state, with default status (Waiting) and empty data map.
func newState(jobId string) *state {
	return &state{
		StateDTO: domain.StateDTO{
			JobID:  jobId,
			Status: domain.Waiting,
			Data:   make(map[string]interface{}),
		},
	}
}

// SetStatus sets the current job execution status in a thread-safe manner.
//
// Parameters:
//   - status: The new status to assign to the job (e.g., Running, Error, etc.).
func (s *state) SetStatus(status domain.JobStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

// GetStatus retrieves the current job execution status in a thread-safe manner.
//
// Returns:
//   - The current job status.
func (s *state) GetStatus() domain.JobStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

// TrySetStatus attempts to update the job status if its current status matches one of the allowed values.
//
// Parameters:
//   - allowed: List of acceptable current statuses for transition.
//   - status: The desired new status.
//
// Returns:
//   - true if the status was updated successfully; false otherwise.
func (s *state) TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, allowedStatus := range allowed {
		if s.Status == allowedStatus {
			s.Status = status
			return true
		}
	}
	return false
}

// UpdateExecutionTime recalculates and updates the execution duration
// as the time elapsed since StartAt, in nanoseconds.
//
// Returns:
//   - The updated execution time.
func (s *state) UpdateExecutionTime() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ExecutionTime = time.Since(s.StartAt).Nanoseconds()
	return s.ExecutionTime
}

// Update applies the provided StateDTO to the current state.
//
// In non-strict mode:
//   - Only non-zero or non-empty fields from the input will overwrite existing values.
//
// In strict mode:
//   - All fields in the current state will be fully overwritten.
//
// Parameters:
//   - state: StateDTO containing updated fields.
//   - strict: Whether to overwrite all fields unconditionally.
func (s *state) Update(state domain.StateDTO, strict bool) {
	s.mu.Lock()
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
	if state.Success > 0 {
		s.Success = state.Success
	}
	if state.Failure > 0 {
		s.Failure = state.Failure
	}
}

// GetState returns a safe, read-only snapshot of the current state.
//
// Returns:
//   - A copy of the current StateDTO with all available execution data.
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

// SetEndState finalizes the job's state after execution ends,
// updating timing, execution duration, final status, and error information.
//
// If the current status is not in [Running, Waiting], it is replaced with Error,
// and a corresponding error is recorded in JobError.
//
// Parameters:
//   - resOnSuccess: Whether to reset the retry counter on successful completion.
//   - status: The final job status to set (e.g., Completed, Error).
//   - err: The execution error, if any.
func (s *state) SetEndState(resOnSuccess bool, status domain.JobStatus, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.EndAt = time.Now()
	s.ExecutionTime = time.Since(s.StartAt).Nanoseconds()

	// Invalid state transition handling
	if !(s.Status == domain.Running || s.Status == domain.Waiting) {
		s.Error.JobError = errs.New(errs.ErrJobWrongStatus,
			fmt.Sprintf("wrong status transition from %s to %s", s.Status, status),
		)
		s.Status = domain.Error
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
}
