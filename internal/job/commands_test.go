package job

import (
	"context"
	"testing"
	"time"

	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"

	"github.com/stretchr/testify/assert"
)

func newRunningJobWithPauseHandler(t *testing.T) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	j, err := New(domain.JobDTO{
		ID:   "test-pause",
		Name: "pause handler job",
		Schedule: domain.Schedule{
			Interval: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error {
			select {
			case <-ctrl.PauseChan():
				return nil
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			}
		},
	}, ctx)
	assert.NoError(t, err)
	j.SetStatus(domain.Running)
	return j
}

func TestJob_Pause_Success(t *testing.T) {
	j := newRunningJobWithPauseHandler(t)

	go func() {
		<-j.ctrl.PauseChan()
	}()

	err := j.Pause(300 * time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, domain.Paused, j.GetStatus())
}

func TestJob_Pause_AlreadyPaused(t *testing.T) {
	j := newRunningJobWithPauseHandler(t)
	j.SetStatus(domain.Paused)

	err := j.Pause(0)
	assert.ErrorIs(t, err, errs.ErrJobNotRunning)
}

func TestJob_Pause_Timeout(t *testing.T) {
	j := newRunningJobWithPauseHandler(t)
	err := j.Pause(100 * time.Millisecond)
	<-time.After(1000 * time.Millisecond)
	assert.ErrorIs(t, err, errs.ErrJobPaused)
	assert.Equal(t, domain.Running, j.GetStatus())
}

func TestJob_Resume_FromPaused(t *testing.T) {
	ctx := context.Background()
	j, err := New(domain.JobDTO{
		ID:   "resume-paused",
		Name: "resume paused job",
		Schedule: domain.Schedule{
			Interval: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx)
	assert.NoError(t, err)

	j.SetStatus(domain.Paused)

	go func() {
		<-j.ctrl.ResumeChan()
	}()

	err = j.Resume()
	assert.NoError(t, err)
}

func TestJob_Resume_FromStopped(t *testing.T) {
	ctx := context.Background()
	j, err := New(domain.JobDTO{
		ID:   "resume-stopped",
		Name: "resume stopped job",
		Schedule: domain.Schedule{
			Interval: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx)
	assert.NoError(t, err)

	j.SetStatus(domain.Stopped)
	err = j.Resume()
	assert.NoError(t, err)
	assert.Equal(t, domain.Waiting, j.GetStatus())
}

func TestJob_Resume_FromInvalidState(t *testing.T) {
	ctx := context.Background()
	j, err := New(domain.JobDTO{
		ID:   "resume-invalid",
		Name: "resume invalid state job",
		Schedule: domain.Schedule{
			Interval: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx)
	assert.NoError(t, err)

	j.SetStatus(domain.Completed)
	err = j.Resume()
	assert.ErrorIs(t, err, errs.ErrJobNotPausedOrStopped)
}

func TestJob_Stop_Transitions(t *testing.T) {
	ctx := context.Background()
	j, err := New(domain.JobDTO{
		ID:   "stop-running",
		Name: "stop transition job",
		Schedule: domain.Schedule{
			Interval: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx)
	assert.NoError(t, err)

	j.SetStatus(domain.Running)
	j.Stop()

	state := j.GetState()
	assert.Equal(t, domain.Stopped, state.Status)
	assert.False(t, state.EndAt.IsZero())
}

func TestJob_Stop_WithHook(t *testing.T) {
	var hookCalled bool
	ctx := context.Background()

	j, err := New(domain.JobDTO{
		ID:   "stop-hook",
		Name: "job with stop hook",
		Schedule: domain.Schedule{
			Interval: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
		Hooks: domain.Hooks{
			OnStop: func(ctrl domain.FnControl) error {
				hookCalled = true
				return nil
			},
		},
	}, ctx)
	assert.NoError(t, err)

	j.SetStatus(domain.Paused)
	j.Stop()

	assert.True(t, hookCalled)
	assert.Equal(t, domain.Stopped, j.GetStatus())
}
