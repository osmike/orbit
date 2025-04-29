package job

import (
	"context"
	"orbit/monitoring"
	"testing"
	"time"

	"orbit/internal/domain"
	errs "orbit/internal/error"

	"github.com/stretchr/testify/assert"
)

func newTestJob(t *testing.T) *Job {
	mon := monitoring.New()
	job, err := New(domain.JobDTO{
		ID:   "test-job",
		Name: "test",
		Interval: domain.Interval{
			Time: time.Minute,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, context.Background(), mon)
	assert.NoError(t, err)
	return job
}

func TestJob_CanExecute_Success(t *testing.T) {
	j := newTestJob(t)
	j.SetStatus(domain.Waiting)
	j.StartAt = time.Now().Add(-1 * time.Second)
	j.EndAt = time.Now().Add(time.Minute)

	err := j.CanExecute()
	assert.NoError(t, err)
}

func TestJob_CanExecute_WrongStatus(t *testing.T) {
	j := newTestJob(t)
	j.SetStatus(domain.Running)

	err := j.CanExecute()
	assert.ErrorIs(t, err, errs.ErrJobWrongStatus)
}

func TestJob_CanExecute_TooEarly(t *testing.T) {
	j := newTestJob(t)
	j.SetStatus(domain.Waiting)
	j.StartAt = time.Now().Add(10 * time.Second)

	err := j.CanExecute()
	assert.ErrorIs(t, err, errs.ErrJobExecTooEarly)
}

func TestJob_CanExecute_TooLate(t *testing.T) {
	j := newTestJob(t)
	j.SetStatus(domain.Waiting)
	j.StartAt = time.Now().Add(-time.Minute)
	j.EndAt = time.Now().Add(-time.Second)

	err := j.CanExecute()
	assert.ErrorIs(t, err, errs.ErrJobExecAfterEnd)
}

func TestJob_CalcNextRun_CronMode(t *testing.T) {
	j := newTestJob(t)

	cron, err := ParseCron("* * * * *")
	assert.NoError(t, err)

	j.cron = cron
	j.state.StartAt = time.Now()

	next := j.SetNextRun(j.state.StartAt)
	assert.True(t, next.After(time.Now()))
}
