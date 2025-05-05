package job

import (
	"errors"
	"orbit/internal/domain"
	"orbit/monitoring"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestState_Init(t *testing.T) {
	mon := monitoring.New()
	s := newState("job-123", mon)
	assert.Equal(t, "job-123", s.JobID)
	assert.Equal(t, domain.Waiting, s.Status)
	assert.NotNil(t, s.Data)
}

func TestState_SetAndGetStatus(t *testing.T) {
	mon := monitoring.New()
	s := newState("job-1", mon)
	s.SetStatus(domain.Running)
	assert.Equal(t, domain.Running, s.GetStatus())
}

func TestState_TrySetStatus(t *testing.T) {
	mon := monitoring.New()
	s := newState("job-1", mon)

	s.SetStatus(domain.Waiting)
	ok := s.TrySetStatus([]domain.JobStatus{domain.Waiting}, domain.Running)
	assert.True(t, ok)
	assert.Equal(t, domain.Running, s.GetStatus())

	fail := s.TrySetStatus([]domain.JobStatus{domain.Waiting}, domain.Completed)
	assert.False(t, fail)
	assert.Equal(t, domain.Running, s.GetStatus())
}

func TestState_UpdateExecutionTime(t *testing.T) {
	mon := monitoring.New()
	s := newState("job-1", mon)
	s.StartAt = time.Now()

	time.Sleep(5 * time.Millisecond)
	et := s.UpdateExecutionTime()

	assert.Greater(t, et, int64(0))
	assert.Equal(t, et, s.GetState().ExecutionTime)
}

func TestState_Update(t *testing.T) {
	mon := monitoring.New()
	s := newState("job-1", mon)

	now := time.Now()
	newState := domain.StateDTO{
		StartAt:       now,
		EndAt:         now.Add(1 * time.Second),
		Status:        domain.Completed,
		Error:         domain.StateError{JobError: errors.New("fail")},
		ExecutionTime: 1000,
		Data: map[string]interface{}{
			"key": "val",
		},
	}

	s.Update(newState, false)
	stored := s.GetState()
	assert.Equal(t, domain.Completed, stored.Status)
	assert.Equal(t, "fail", stored.Error.JobError.Error())
	assert.Equal(t, "val", stored.Data["key"])
	assert.Equal(t, int64(1000), stored.ExecutionTime)
}

func TestState_GetState(t *testing.T) {
	mon := monitoring.New()
	s := newState("job-1", mon)
	s.SetStatus(domain.Running)
	st := s.GetState()
	assert.Equal(t, domain.Running, st.Status)
	assert.Equal(t, "job-1", st.JobID)
}

func TestState_SetEndState_Valid(t *testing.T) {
	mon := monitoring.New()
	s := newState("job-1", mon)
	s.SetStatus(domain.Running)
	s.StartAt = time.Now()

	time.Sleep(20 * time.Millisecond)

	s.SetEndState(true, domain.Completed, nil)
	time.Sleep(20 * time.Millisecond)
	state := s.GetState()

	assert.Equal(t, domain.Completed, state.Status)
	assert.True(t, state.EndAt.Before(time.Now()))
	assert.True(t, state.ExecutionTime > 0)
	assert.True(t, state.Error.IsEmpty())
}

func TestState_SetEndState_InvalidTransition(t *testing.T) {
	mon := monitoring.New()
	s := newState("job-1", mon)
	s.SetStatus(domain.Completed)
	s.StartAt = time.Now()

	s.SetEndState(true, domain.Completed, nil)

	state := s.GetState()
	assert.Equal(t, domain.Completed, state.Status)

	s.SetStatus(domain.Paused)
	s.StartAt = time.Now()
	s.SetEndState(true, domain.Completed, nil)
	state = s.GetState()
	assert.Equal(t, domain.Completed, state.Status)

	s.SetStatus(domain.Stopped)
	s.StartAt = time.Now()
	s.SetEndState(true, domain.Completed, nil)
	state = s.GetState()
	assert.Equal(t, domain.Stopped, state.Status)
}

func TestState_SetEndState_ResetsRetry(t *testing.T) {
	mon := monitoring.New()
	s := newState("job-1", mon)
	s.SetStatus(domain.Running)
	s.StartAt = time.Now()

	s.Error.JobError = errors.New("fail")
	s.currentRetry = 3
	s.SetEndState(true, domain.Completed, nil)

	assert.Equal(t, int(0), s.currentRetry)
}
