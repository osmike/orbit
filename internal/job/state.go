package job

import (
	"fmt"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

// State represents the execution state of a job.
// It tracks execution metadata, including start and end timestamps, status, error details, and execution time.
type state struct {
	domain.StateDTO
	mu sync.Mutex // Protects all fields of State from race conditions.
	// currentRetry keeps track of the remaining retry attempts.
	currentRetry int64
}

func (s *state) Init(id string) *state {
	return &state{
		StateDTO: domain.StateDTO{
			JobID:  id,
			Status: domain.Waiting,
			Data:   make(map[string]interface{}),
		},
	}
}

// SetStatus safely updates the job's execution status.
func (s *state) SetStatus(status domain.JobStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

// GetStatus safely retrieves the current job execution status.
func (s *state) GetStatus() domain.JobStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status
}

// TrySetStatus attempts to update the job status only if it is in the allowed state.
// It returns true if the status was successfully updated, otherwise false.
func (s *state) TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	currentStatus := s.Status
	for _, allowedStatus := range allowed {
		if currentStatus == allowedStatus {
			s.Status = status
			return true
		}
	}
	return false
}

// UpdateExecutionTime safely updates the execution time of the job.
func (s *state) UpdateExecutionTime() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ExecutionTime = time.Since(s.StartAt).Nanoseconds()
	return s.ExecutionTime
}

// Update modifies the job state based on a given StateDTO.
// If strict mode is enabled, all fields are overwritten.
// Otherwise, only non-zero values from the DTO are applied.
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
}

// GetState returns a snapshot of the current job execution state as a StateDTO.
// This method ensures thread safety by acquiring a lock before copying fields.
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

func (s *state) SetEndState(resOnSuccess bool, start time.Time, status domain.JobStatus, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EndAt = time.Now()
	s.ExecutionTime = time.Since(start).Nanoseconds()
	if !s.TrySetStatus([]domain.JobStatus{domain.Running, domain.Waiting}, status) {
		s.Status = domain.Error
		s.Error = errs.New(errs.ErrJobWrongStatus, fmt.Sprintf("wrong status transition from %s to %s", s.Status, status))
	}
	if s.Error != nil && resOnSuccess && err == nil {
		s.currentRetry = 0
	}
	s.Status = domain.Completed
	s.Error = err
}
