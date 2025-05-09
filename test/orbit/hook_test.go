package orbit_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/osmike/orbit"
	"github.com/osmike/orbit/test"
	"github.com/osmike/orbit/test/monitoring"
	"github.com/stretchr/testify/assert"
	"log"
	"strconv"
	"testing"
	"time"
)

func TestHookOnStartError_RespectsIgnoreFlag(t *testing.T) {
	mon := monitoring.New()
	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)

	assert.NoError(t, err)

	called := false

	withHook := func(ignore bool) orbit.Job {
		return orbit.Job{
			ID: "job-hook-ignore-" + strconv.FormatBool(ignore),
			Fn: func(ctrl orbit.FnControl) error {
				called = true
				return nil
			},
			Hooks: orbit.Hooks{
				OnStart: orbit.Hook{
					Fn: func(ctrl orbit.FnControl, err error) error {
						return errors.New("onStart failed")
					},
					IgnoreError: ignore,
				},
			},
			Interval: orbit.Interval{Time: 0},
		}
	}

	jobOK := withHook(true)
	jobFail := withHook(false)

	assert.NoError(t, pool.AddJob(jobOK))
	assert.NoError(t, pool.AddJob(jobFail))

	err = pool.Run()
	assert.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	assert.True(t, called, "Job with IgnoreError=true should have executed")

	metrics := mon.GetMetrics()
	failState := metrics[jobFail.ID].(orbit.JobState)
	assert.Equal(t, orbit.JobStatus("error"), failState.Status)
	assert.Contains(t, failState.Error.OnStart.Error(), "onStart failed")
}

func TestHookOnError_IsCalled(t *testing.T) {
	mon := monitoring.New()

	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)
	assert.NoError(t, err)

	var capturedErr error

	job := orbit.Job{
		ID: "job-with-error-hook",
		Fn: func(ctrl orbit.FnControl) error {
			return errors.New("simulated failure")
		},
		Hooks: orbit.Hooks{
			OnError: orbit.Hook{
				Fn: func(ctrl orbit.FnControl, err error) error {
					capturedErr = err
					return nil
				},
			},
		},
		Interval: orbit.Interval{Time: 0},
	}

	assert.NoError(t, pool.AddJob(job))
	pool.Run()

	test.WaitForCondition(t, 1*time.Second, func() bool {
		return capturedErr != nil
	})

	assert.NotNil(t, capturedErr)
	assert.Contains(t, capturedErr.Error(), "simulated failure")
}

func TestHookLoggingJobFlow(t *testing.T) {

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	hookFactory := func(hookBoolean *bool, str string) func(ctrl orbit.FnControl, err error) error {
		return func(ctrl orbit.FnControl, err error) error {
			logger.Println(str)
			*hookBoolean = true
			return nil
		}
	}

	var (
		onStartHookExecuted   bool
		onResumeHookExecuted  bool
		onPauseHookExecuted   bool
		onStopHookExecuted    bool
		onSuccessHookExecuted bool
		onErrorHookExecuted   bool
		finallyHookExecuted   bool
	)
	p, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{}, monitoring.New())
	assert.NoError(t, err)
	defer p.Kill()

	fn := func(ctrl orbit.FnControl) error {
		for {
			select {
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			case <-ctrl.PauseChan():
				<-ctrl.ResumeChan()
			default:
				logger.Println("[main] syncing data...")
				time.Sleep(300 * time.Millisecond)
			}
		}
	}

	onStartHook := hookFactory(&onStartHookExecuted, "[hook] OnStart: job starting...")
	onSuccessHook := hookFactory(&onSuccessHookExecuted, "[hook] osSuccess: job executed with no error...")
	onErrorHook := hookFactory(&onErrorHookExecuted, "[hook] osError: job executed with an error...")
	onPauseHook := hookFactory(&onPauseHookExecuted, "[hook] osPause: job pausing...")
	onResumeHook := hookFactory(&onResumeHookExecuted, "[hook] osResume: job resuming...")
	onStopHook := hookFactory(&onStopHookExecuted, "[hook] osStop: job stopped...")
	finallyHook := hookFactory(&finallyHookExecuted, "[hook] finally: job finished...")

	jobID := "job-with-hook"
	jobCfg := orbit.Job{
		ID: jobID,
		Fn: fn,
		Hooks: orbit.Hooks{
			OnStart:   orbit.Hook{Fn: onStartHook},
			OnStop:    orbit.Hook{Fn: onStopHook},
			OnError:   orbit.Hook{Fn: onErrorHook},
			OnSuccess: orbit.Hook{Fn: onSuccessHook},
			OnPause:   orbit.Hook{Fn: onPauseHook},
			OnResume:  orbit.Hook{Fn: onResumeHook},
			Finally:   orbit.Hook{Fn: finallyHook},
		},
	}
	assert.NoError(t, p.AddJob(jobCfg))
	assert.NoError(t, p.Run())

	time.Sleep(200 * time.Millisecond)
	assert.NoError(t, p.PauseJob(jobID, 2*time.Second))
	time.Sleep(150 * time.Millisecond)
	assert.True(t, onPauseHookExecuted)
	assert.NoError(t, p.ResumeJob(jobID))
	assert.True(t, onResumeHookExecuted)

	time.Sleep(200 * time.Millisecond)
	assert.NoError(t, p.StopJob(jobID))
	time.Sleep(150 * time.Millisecond)
	assert.True(t, onStopHookExecuted)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, onErrorHookExecuted)
	assert.False(t, onSuccessHookExecuted)
	assert.True(t, finallyHookExecuted)

	logs := output.String()
	assert.Contains(t, logs, "[hook] OnStart: job starting...")
	assert.Contains(t, logs, "[hook] osPause: job pausing...")
	assert.Contains(t, logs, "[hook] osResume: job resuming...")
	assert.Contains(t, logs, "[hook] osStop: job stopped...")
	assert.Contains(t, logs, "[hook] osError: job executed with an error...")
	assert.Contains(t, logs, "[hook] finally: job finished...")
	assert.Contains(t, logs, "[main] syncing data...")
}
