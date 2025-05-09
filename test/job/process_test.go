package job_test

import (
	"context"
	"github.com/osmike/orbit/internal/job"
	"github.com/osmike/orbit/test/monitoring"
	"testing"
	"time"

	"github.com/osmike/orbit/internal/domain"
	"github.com/stretchr/testify/assert"
)

func newTestJobForProcess(t *testing.T) domain.Job {
	mon := monitoring.New()
	j, err := job.New(domain.JobDTO{
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
