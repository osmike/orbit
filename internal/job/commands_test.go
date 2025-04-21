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

func newRunningJobWithoutPauseHandler(t *testing.T) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mon := monitoring.New()

	j, err := New(domain.JobDTO{
		ID:   "test-pause-no-handler",
		Name: "job without pause handler",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error {
			select {
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			}
		},
	}, ctx, mon)
	assert.NoError(t, err)
	j.SetStatus(domain.Running)
	return j
}

func newRunningJobWithPauseHandler(t *testing.T) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mon := monitoring.New()

	j, err := New(domain.JobDTO{
		ID:   "test-pause",
		Name: "pause handler job",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error {
			select {
			case <-ctrl.PauseChan():
				return nil
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			}
		},
	}, ctx, mon)
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
	j := newRunningJobWithoutPauseHandler(t)

	err := j.Pause(100 * time.Millisecond)
	assert.NoError(t, err) // Pause now never returns error even on timeout

	time.Sleep(200 * time.Millisecond) // allow internal goroutine to revert

	assert.Equal(t, domain.Running, j.GetStatus())
}

func TestJob_Resume_FromPaused(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resumed := make(chan struct{})
	mon := monitoring.New()

	j, err := New(domain.JobDTO{
		ID:   "resume-paused",
		Name: "job that handles pause/resume",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error {
			for {
				select {
				case <-ctrl.PauseChan():
					<-ctrl.ResumeChan()
					resumed <- struct{}{}
					return nil
				case <-ctrl.Context().Done():
					return ctrl.Context().Err()
				}
			}
		},
	}, ctx, mon)
	assert.NoError(t, err)

	j.SetStatus(domain.Running)
	err = j.Pause(300 * time.Millisecond)

	go func() {
		_ = j.Fn(j.ctrl)
	}()

	time.Sleep(100 * time.Millisecond)
	err = j.Resume()
	assert.NoError(t, err)

	select {
	case <-resumed:
		assert.True(t, true, "Resume handled successfully")
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for resume handling")
	}
}

func TestJob_EndedContext_FromResumed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resumed := make(chan struct{})
	mon := monitoring.New()

	j, err := New(domain.JobDTO{
		ID:   "context-ended-in-resume",
		Name: "job that handles handle context end while paused",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error {
			for {
				select {
				case <-ctrl.PauseChan():
					<-ctrl.ResumeChan()
					resumed <- struct{}{}
					return nil
				case <-ctrl.Context().Done():
					return ctrl.Context().Err()
				}
			}
		},
	}, ctx, mon)
	assert.NoError(t, err)

	j.SetStatus(domain.Running)
	err = j.Pause(300 * time.Millisecond)

	go func() {
		_ = j.Fn(j.ctrl)
	}()

	time.Sleep(100 * time.Millisecond)
	j.Stop()
	assert.NoError(t, err)

	select {
	case <-resumed:
		assert.True(t, true, "Resume handled successfully")
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for resume handling")
	}
}

func TestJob_Resume_FromStopped(t *testing.T) {
	mon := monitoring.New()
	ctx := context.Background()
	j, err := New(domain.JobDTO{
		ID:   "resume-stopped",
		Name: "resume stopped job",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx, mon)
	assert.NoError(t, err)

	j.SetStatus(domain.Stopped)
	err = j.Resume()
	assert.NoError(t, err)
	assert.Equal(t, domain.Waiting, j.GetStatus())
}

func TestJob_Resume_FromInvalidState(t *testing.T) {
	mon := monitoring.New()
	ctx := context.Background()
	j, err := New(domain.JobDTO{
		ID:   "resume-invalid",
		Name: "resume invalid state job",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx, mon)
	assert.NoError(t, err)

	j.SetStatus(domain.Completed)
	err = j.Resume()
	assert.ErrorIs(t, err, errs.ErrJobNotPausedOrStopped)
}

func TestJob_Stop_Transitions(t *testing.T) {
	mon := monitoring.New()
	ctx := context.Background()
	j, err := New(domain.JobDTO{
		ID:   "stop-running",
		Name: "stop transition job",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
	}, ctx, mon)
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
	mon := monitoring.New()

	j, err := New(domain.JobDTO{
		ID:   "stop-hook",
		Name: "job with stop hook",
		Interval: domain.Interval{
			Time: time.Second,
		},
		Fn: func(ctrl domain.FnControl) error { return nil },
		Hooks: domain.Hooks{
			OnStop: domain.Hook{
				Fn: func(ctrl domain.FnControl, err error) error {
					hookCalled = true
					return nil
				},
				IgnoreError: false,
			},
		},
	}, ctx, mon)
	assert.NoError(t, err)

	j.SetStatus(domain.Paused)
	j.Stop()

	assert.True(t, hookCalled)
	assert.Equal(t, domain.Stopped, j.GetStatus())
}
