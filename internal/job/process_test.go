package job

import (
	"context"
	"testing"
	"time"

	"orbit/internal/domain"
	"orbit/monitoring"

	"github.com/stretchr/testify/assert"
)

func newTestJobForProcess(t *testing.T) *Job {
	mon := monitoring.New()
	j, err := New(domain.JobDTO{
		ID:       "test-job-process",
		Name:     "process test",
		Interval: domain.Interval{Time: time.Second},
		Timeout:  200 * time.Millisecond,
		Fn: func(ctrl domain.FnControl) error {
			ctrl.SaveData(map[string]interface{}{"info": "start"})
			return nil
		},
	}, context.Background(), mon)
	assert.NoError(t, err)
	return j
}

func TestJob_ProcessStart(t *testing.T) {
	j := newTestJobForProcess(t)

	j.ProcessStart()

	state := j.GetState()
	assert.Equal(t, domain.Running, state.Status)
	assert.True(t, !state.StartAt.IsZero())
	assert.True(t, state.EndAt.IsZero())
	assert.Zero(t, state.ExecutionTime)
	assert.Empty(t, state.Data)
}

func TestJob_ProcessRun_NoTimeout(t *testing.T) {
	j := newTestJobForProcess(t)

	j.ProcessStart()
	time.Sleep(50 * time.Millisecond)
	err := j.ProcessRun()

	assert.NoError(t, err)
	assert.Less(t, j.GetState().ExecutionTime, int64(j.Timeout))
}

func TestJob_ProcessRun_Timeout(t *testing.T) {
	j := newTestJobForProcess(t)

	j.ProcessStart()
	time.Sleep(300 * time.Millisecond) // Exceeds Timeout
	err := j.ProcessRun()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job timed out")
}

func TestJob_ProcessEnd(t *testing.T) {
	j := newTestJobForProcess(t)

	j.ProcessStart()
	time.Sleep(10 * time.Millisecond)

	j.ProcessEnd(domain.Completed, nil)

	state := j.GetState()
	assert.Equal(t, domain.Completed, state.Status)
	assert.True(t, !state.EndAt.IsZero())
	assert.Greater(t, state.ExecutionTime, int64(0))
}
