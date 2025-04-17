package job

import (
	"context"
	"testing"
	"time"

	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"

	"github.com/stretchr/testify/assert"
)

func newTestJob(t *testing.T) *Job {
	job, err := New("pool-id", domain.JobDTO{
		ID:   "test-job",
		Name: "test",
		Interval: domain.Interval{
			Time: time.Minute,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, context.Background())
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

func TestJob_CalcNextRun_IntervalMode(t *testing.T) {
	j := newTestJob(t)
	j.state.StartAt = time.Now()
	next := j.NextRun()

	assert.WithinDuration(t, time.Now(), next, 1*time.Minute)
}

func TestJob_CalcNextRun_CronMode(t *testing.T) {
	j := newTestJob(t)

	cron, err := ParseCron("* * * * *")
	assert.NoError(t, err)

	j.cron = cron
	j.state.StartAt = time.Now()

	next := j.NextRun()
	assert.True(t, next.After(time.Now()))
}

//func TestJob_Retry_WithinLimit(t *testing.T) {
//	j := newTestJob(t)
//	j.JobDTO.Retry.Count = 3
//
//	for i := 0; i <= 3; i++ {
//		err := j.Retry()
//		if i < 3 {
//			assert.NoError(t, err)
//		} else {
//			assert.ErrorIs(t, err, errs.ErrJobRetryLimit)
//		}
//	}
//}
//
//func TestJob_Retry_ZeroLimit(t *testing.T) {
//	j := newTestJob(t)
//	j.JobDTO.Retry.Count = 0
//
//	err := j.Retry()
//	assert.ErrorIs(t, err, errs.ErrJobRetryLimit)
//}
