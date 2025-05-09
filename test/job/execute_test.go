package job

import (
	"context"
	"errors"
	"github.com/osmike/orbit/monitoring"
	"testing"
	"time"

	"github.com/osmike/orbit/internal/domain"
	errs "github.com/osmike/orbit/internal/error"

	"github.com/stretchr/testify/assert"
)

func newExecutableJob(t *testing.T, hooks domain.Hooks, fn domain.Fn) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mon := monitoring.New()

	job, err := New(domain.JobDTO{
		ID:   "exec-job",
		Name: "execution test",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn:    fn,
		Hooks: hooks,
	}, ctx, mon)
	assert.NoError(t, err)

	return job
}

func TestJob_Execute_Success(t *testing.T) {
	var startCalled, successCalled, finallyCalled bool

	job := newExecutableJob(t, domain.Hooks{
		OnStart: domain.Hook{
			Fn: func(ctrl domain.FnControl, err error) error {
				startCalled = true
				return nil
			},
			IgnoreError: false,
		},
		OnSuccess: domain.Hook{
			Fn: func(ctrl domain.FnControl, err error) error {
				successCalled = true
				return nil
			},
			IgnoreError: false,
		},
		Finally: domain.Hook{
			Fn: func(ctrl domain.FnControl, err error) error {
				finallyCalled = true
				return nil
			},
			IgnoreError: false,
		},
	}, func(ctrl domain.FnControl) error {
		return nil
	})

	err := job.Execute()
	assert.NoError(t, err)
	assert.True(t, startCalled)
	assert.True(t, successCalled)
	assert.True(t, finallyCalled)
}

func TestJob_Execute_FnFails(t *testing.T) {
	var errorCalled, finallyCalled bool

	job := newExecutableJob(t, domain.Hooks{
		OnError: domain.Hook{
			Fn: func(ctrl domain.FnControl, err error) error {
				errorCalled = true
				return nil
			},
			IgnoreError: false,
		},
		Finally: domain.Hook{
			Fn: func(ctrl domain.FnControl, err error) error {
				finallyCalled = true
				return nil
			},
			IgnoreError: false,
		},
	}, func(ctrl domain.FnControl) error {
		return errors.New("job failure")
	})

	err := job.Execute()

	assert.Error(t, err)
	assert.True(t, errorCalled)
	assert.True(t, finallyCalled)
}

func TestJob_Execute_HookFails(t *testing.T) {
	job := newExecutableJob(t, domain.Hooks{
		OnStart: domain.Hook{
			Fn: func(ctrl domain.FnControl, err error) error {
				return errs.ErrOnStartHook
			},
			IgnoreError: false,
		},
	}, func(ctrl domain.FnControl) error {
		return nil
	})

	err := job.Execute()

	assert.ErrorIs(t, err, errs.ErrOnStartHook)
}

func TestJob_Execute_FinallyFails(t *testing.T) {
	job := newExecutableJob(t, domain.Hooks{
		Finally: domain.Hook{
			Fn: func(ctrl domain.FnControl, err error) error {
				return errors.New("finally hook failure")
			},
			IgnoreError: true,
		},
	}, func(ctrl domain.FnControl) error {
		return nil
	})

	err := job.Execute()
	state := job.GetState()

	assert.NoError(t, err)
	assert.EqualError(t, state.Error.HookError.Finally, "error in finally hook: job id: exec-job, error: finally hook failure")
}

func TestJob_Execute_AlreadyRunning(t *testing.T) {
	ctx := context.Background()
	mon := monitoring.New()
	job, err := New(domain.JobDTO{
		ID:   "already-running",
		Name: "test job",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error {
			time.Sleep(2 * time.Second)
			return nil
		},
	}, ctx, mon)
	assert.NoError(t, err)

	go func() {
		_ = job.Execute()
	}()

	time.Sleep(100 * time.Millisecond)

	err = job.Execute()
	assert.ErrorIs(t, err, errs.ErrJobStillRunning)
}

func TestJob_Execute_WithPauseAndResume(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	resumed := make(chan struct{}, 1)
	mon := monitoring.New()
	job, err := New(domain.JobDTO{
		ID:       "job-pause-resume",
		Name:     "pause/resume job",
		Interval: domain.Interval{Time: time.Second},
		Fn: func(ctrl domain.FnControl) error {
			select {
			case <-ctrl.PauseChan():
				<-ctrl.ResumeChan()
				resumed <- struct{}{}
				return nil
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			}
		},
	}, ctx, mon)
	assert.NoError(t, err)

	job.SetStatus(domain.Running)

	go func() {
		time.Sleep(100 * time.Millisecond)
		job.Pause(300 * time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		job.Resume(ctx)
	}()

	err = job.Execute()
	assert.NoError(t, err)

	select {
	case <-resumed:
		assert.True(t, true, "job resumed successfully")
	case <-time.After(time.Second):
		t.Fatal("job did not resume in time")
	}
}

func TestJob_Execute_StoppedDuringFn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mon := monitoring.New()
	job, err := New(domain.JobDTO{
		ID:       "job-stop-test",
		Name:     "job to stop",
		Interval: domain.Interval{Time: time.Second},
		Fn: func(ctrl domain.FnControl) error {
			select {
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			case <-time.After(time.Second):
				return nil
			}
		},
	}, ctx, mon)
	assert.NoError(t, err)

	go func() {
		time.Sleep(100 * time.Millisecond)
		err := job.Stop()
		assert.NoError(t, err)
	}()

	err = job.Execute()
	state := job.GetState()

	assert.ErrorIs(t, err, errs.ErrJobExecution)
	assert.Contains(t, err.Error(), "context canceled")
	assert.Equal(t, domain.Stopped, state.Status)
	assert.EqualError(t, state.Error.JobError, "error in job execution: job id: job-stop-test, error: context canceled")
}
