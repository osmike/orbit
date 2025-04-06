package job

import (
	"context"
	"testing"
	"time"

	"go-scheduler/internal/domain"
	"go-scheduler/monitoring"

	"github.com/stretchr/testify/assert"
)

func newTestJobForProcess(t *testing.T) *Job {
	j, err := New(domain.JobDTO{
		ID:       "test-job-process",
		Name:     "process test",
		Interval: domain.Interval{Time: time.Second},
		Timeout:  200 * time.Millisecond,
		Fn: func(ctrl domain.FnControl) error {
			ctrl.SaveData(map[string]interface{}{"info": "start"})
			return nil
		},
	}, context.Background())
	assert.NoError(t, err)
	return j
}

func TestJob_ProcessStart(t *testing.T) {
	j := newTestJobForProcess(t)
	mon := monitoring.New()

	j.ProcessStart(mon)

	state := j.GetState()
	assert.Equal(t, domain.Running, state.Status)
	assert.True(t, !state.StartAt.IsZero())
	assert.True(t, state.EndAt.IsZero())
	assert.Zero(t, state.ExecutionTime)
	assert.Empty(t, state.Data)

	metrics := mon.GetMetrics()
	_, exists := metrics[j.ID]
	assert.True(t, exists)
}

func TestJob_ProcessRun_NoTimeout(t *testing.T) {
	j := newTestJobForProcess(t)
	mon := monitoring.New()

	j.ProcessStart(mon)
	time.Sleep(50 * time.Millisecond)
	err := j.ProcessRun(mon)

	assert.NoError(t, err)
	assert.Less(t, j.GetState().ExecutionTime, int64(j.Timeout))
}

func TestJob_ProcessRun_Timeout(t *testing.T) {
	j := newTestJobForProcess(t)
	mon := monitoring.New()

	j.ProcessStart(mon)
	time.Sleep(300 * time.Millisecond) // Exceeds Timeout
	err := j.ProcessRun(mon)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job timed out")
}

func TestJob_ProcessEnd(t *testing.T) {
	j := newTestJobForProcess(t)
	mon := monitoring.New()

	j.ProcessStart(mon)
	time.Sleep(10 * time.Millisecond)

	j.ProcessEnd(domain.Completed, nil, mon)

	state := j.GetState()
	assert.Equal(t, domain.Completed, state.Status)
	assert.True(t, !state.EndAt.IsZero())
	assert.Greater(t, state.ExecutionTime, int64(0))

	metrics := mon.GetMetrics()
	finalState, ok := metrics[j.ID].(domain.StateDTO)
	assert.True(t, ok)
	assert.Equal(t, domain.Completed, finalState.Status)
}
