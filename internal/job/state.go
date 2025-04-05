package job

import (
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// state represents the internal execution state of a job.
//
// It tracks runtime details, including timestamps, current status,
// execution duration, errors, custom metadata, and retry attempts.
// Access to all state fields is synchronized via an internal mutex
// to ensure thread safety.
type state struct {
	domain.StateDTO

	mu sync.Mutex // Protects state fields from concurrent access.

	// currentRetry tracks the number of retry attempts made for the job.
	currentRetry int64
}

// Init initializes and returns a new job execution state.
//
// Parameters:
//   - id: The unique identifier of the job.
//
// Returns:
//   - A pointer to a state instance initialized with Waiting status
//     and an empty metadata map.
func (s *state) Init(id string) *state {
	return &state{
		StateDTO: domain.StateDTO{
			JobID:  id,
			Status: domain.Waiting,
			Data:   make(map[string]interface{}),
		},
	}
}

// SetStatus safely sets the current execution status of the job.
//
// Parameters:
//   - status: The new job execution status to set.
func (s *state) SetStatus(status domain.JobStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

// GetStatus safely retrieves the current execution status of the job.
//
// Returns:
//   - The current job execution status.
func (s *state) GetStatus() domain.JobStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status
}

// TrySetStatus attempts to update the job's execution status only if its current status
// matches one of the allowed statuses provided.
//
// Parameters:
//   - allowed: Slice of statuses considered valid for transitioning.
//   - status: The desired new status.
//
// Returns:
//   - true if the status update succeeds; false otherwise.
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

// UpdateExecutionTime updates and returns the elapsed execution time of the job.
//
// It calculates execution duration as the time elapsed since the job started.
//
// Returns:
//   - Execution duration in nanoseconds.
func (s *state) UpdateExecutionTime() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ExecutionTime = time.Since(s.StartAt).Nanoseconds()
	return s.ExecutionTime
}

// Update modifies the state fields based on provided StateDTO.
//
// In strict mode, all state fields are overwritten with the provided values.
// In non-strict mode, only non-zero fields from the provided state are updated.
//
// Parameters:
//   - state: StateDTO with new values to apply.
//   - strict: If true, applies all fields; otherwise, only non-zero fields.
func (s *state) Update(state domain.StateDTO, strict bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strict {
		s.EndAt = state.EndAt
		s.StartAt = state.StartAt
		s.Data = state.Data
		s.Error = state.Error
		s.ExecutionTime = state.ExecutionTime
		return
	}

	if state.Error != nil {
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
}

// GetState safely retrieves a snapshot of the current state as a StateDTO.
//
// Returns:
//   - A copy of the current state containing execution details, timestamps, errors,
//     execution status, and metadata.
func (s *state) GetState() domain.StateDTO {
	s.mu.Lock()
	defer s.mu.Unlock()

	return domain.StateDTO{
		JobID:         s.JobID,
		StartAt:       s.StartAt,
		EndAt:         s.EndAt,
		Error:         s.Error,
		Status:        s.Status,
		ExecutionTime: s.ExecutionTime,
		Data:          s.Data,
	}
}

// SetEndState finalizes the job state at the end of execution.
//
// It updates the EndAt timestamp, calculates the final execution duration,
// and attempts to transition to the provided final status. If an invalid
// status transition occurs, it sets the status to Error.
//
// Additionally, it handles retry logic:
//   - If the job succeeded and ResetOnSuccess is true, retry count resets.
//
// Parameters:
//   - resOnSuccess: Indicates whether retries should reset upon successful execution.
//   - status: The final execution status to set.
//   - err: An error encountered during execution, if any.
func (s *state) SetEndState(resOnSuccess bool, status domain.JobStatus, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.EndAt = time.Now()
	s.ExecutionTime = time.Since(s.StartAt).Nanoseconds()

	if !s.TrySetStatus([]domain.JobStatus{domain.Running, domain.Waiting}, status) {
		s.Status = domain.Error
		s.Error = errs.New(errs.ErrJobWrongStatus, fmt.Sprintf("wrong status transition from %s to %s", s.Status, status))
	}

	if s.Error != nil && resOnSuccess && err == nil {
		s.currentRetry = 0
	}

	s.Error = err
}
