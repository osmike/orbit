package job

import (
	"context"
	"go-scheduler/monitoring"
	"testing"
	"time"

	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"

	"github.com/stretchr/testify/assert"
)

func TestJob_New_Valid(t *testing.T) {
	ctx := context.Background()
	mon := monitoring.New()
	j, err := New(domain.JobDTO{
		ID:   "valid-job",
		Name: "test",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx, mon)

	assert.NoError(t, err)
	assert.NotNil(t, j)
	assert.Equal(t, "valid-job", j.ID)
	assert.Equal(t, "test", j.Name)
}

func TestJob_New_InvalidID(t *testing.T) {
	ctx := context.Background()
	mon := monitoring.New()
	_, err := New(domain.JobDTO{
		ID: "",
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx, mon)
	assert.ErrorIs(t, err, errs.ErrEmptyID)
}

func TestJob_New_InvalidFn(t *testing.T) {
	ctx := context.Background()
	mon := monitoring.New()
	_, err := New(domain.JobDTO{
		ID: "no-fn",
	}, ctx, mon)
	assert.ErrorIs(t, err, errs.ErrEmptyFunction)
}

func TestJob_New_InvalidCronAndInterval(t *testing.T) {
	ctx := context.Background()
	mon := monitoring.New()
	_, err := New(domain.JobDTO{
		ID: "mixed",
		Interval: domain.Interval{
			Time:     time.Second,
			CronExpr: "*/5 * * * *",
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx, mon)
	assert.ErrorIs(t, err, errs.ErrMixedScheduleType)
}

func TestJob_GetSetStatus(t *testing.T) {
	ctx := context.Background()
	mon := monitoring.New()
	j, _ := New(domain.JobDTO{
		ID:       "status-check",
		Fn:       func(ctrl domain.FnControl) error { return nil },
		Interval: domain.Interval{Time: time.Second},
	}, ctx, mon)

	j.SetStatus(domain.Running)
	assert.Equal(t, domain.Running, j.GetStatus())

	success := j.TrySetStatus([]domain.JobStatus{domain.Running}, domain.Completed)
	assert.True(t, success)
	assert.Equal(t, domain.Completed, j.GetStatus())
}

func TestJob_SaveUserDataToState(t *testing.T) {
	ctx := context.Background()
	mon := monitoring.New()
	j, _ := New(domain.JobDTO{
		ID:       "data-job",
		Fn:       func(ctrl domain.FnControl) error { return nil },
		Interval: domain.Interval{Time: time.Second},
	}, ctx, mon)

	j.ctrl.SaveData(map[string]interface{}{"foo": "bar"})
	state := j.GetState()

	val, ok := state.Data["foo"]
	assert.True(t, ok)
	assert.Equal(t, "bar", val)
}

func TestJob_UpdateState(t *testing.T) {
	mon := monitoring.New()
	job, err := New(domain.JobDTO{
		ID:   "update-state-job",
		Name: "update test",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, context.Background(), mon)
	assert.NoError(t, err)

	now := time.Now()
	job.UpdateState(domain.StateDTO{
		StartAt: now,
	})

	state := job.GetState()
	assert.WithinDuration(t, now, state.StartAt, time.Millisecond)
	assert.Equal(t, domain.Waiting, state.Status) // Не должен измениться
}

func TestJob_SaveMetrics(t *testing.T) {
	mon := monitoring.New()
	job, err := New(domain.JobDTO{
		ID:   "metrics-job",
		Name: "metrics save",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error {
			ctrl.SaveData(map[string]interface{}{
				"test_key": "test_value",
			})
			return nil
		},
	}, context.Background(), mon)
	assert.NoError(t, err)

	err = job.Execute()
	assert.NoError(t, err)

	metrics := mon.GetMetrics()
	state, ok := metrics["metrics-job"].(domain.StateDTO)
	assert.True(t, ok)
	assert.Equal(t, "test_value", state.Data["test_key"])
}
