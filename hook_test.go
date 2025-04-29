package orbit

import (
	"bytes"
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"log"
	"orbit/test"
	"strconv"
	"testing"
	"time"
)

func TestHookOnStartError_RespectsIgnoreFlag(t *testing.T) {
	scheduler := New(context.Background())
	mon := newDefaultMon()

	pool, err := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)
	assert.NoError(t, err)

	called := false

	withHook := func(ignore bool) JobConfig {
		return JobConfig{
			ID: "job-hook-ignore-" + strconv.FormatBool(ignore),
			Fn: func(ctrl FnControl) error {
				called = true
				return nil
			},
			Hooks: HooksFunc{
				OnStart: Hook{
					Fn: func(ctrl FnControl, err error) error {
						return errors.New("onStart failed")
					},
					IgnoreError: ignore,
				},
			},
			Interval: IntervalConfig{Time: 0},
		}
	}

	jobOK := withHook(true)
	jobFail := withHook(false)

	assert.NoError(t, scheduler.AddJob(pool, jobOK))
	assert.NoError(t, scheduler.AddJob(pool, jobFail))

	pool.Run()
	time.Sleep(100 * time.Millisecond)

	assert.True(t, called, "Job with IgnoreError=true should have executed")

	metrics := mon.GetMetrics()
	failState := metrics[jobFail.ID].(JobState)
	assert.Equal(t, JobStatus("error"), failState.Status)
	assert.Contains(t, failState.Error.OnStart.Error(), "onStart failed")
}

func TestHookOnError_IsCalled(t *testing.T) {
	scheduler := New(context.Background())
	mon := newDefaultMon()

	pool, err := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)
	assert.NoError(t, err)

	var capturedErr error

	job := JobConfig{
		ID: "job-with-error-hook",
		Fn: func(ctrl FnControl) error {
			return errors.New("simulated failure")
		},
		Hooks: HooksFunc{
			OnError: Hook{
				Fn: func(ctrl FnControl, err error) error {
					capturedErr = err
					return nil
				},
			},
		},
		Interval: IntervalConfig{Time: 0},
	}

	assert.NoError(t, scheduler.AddJob(pool, job))
	pool.Run()

	test.WaitForCondition(t, 1*time.Second, func() bool {
		return capturedErr != nil
	})

	assert.NotNil(t, capturedErr)
	assert.Contains(t, capturedErr.Error(), "simulated failure")
}

func TestHookLoggingJobFlow(t *testing.T) {
	orb := New(context.Background())

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	hookFactory := func(hookBoolean *bool, str string) func(ctrl FnControl, err error) error {
		return func(ctrl FnControl, err error) error {
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
	p, err := orb.CreatePool(PoolConfig{}, newDefaultMon())
	defer p.Kill()
	assert.NoError(t, err, "creating pool with empty config")

	fn := func(ctrl FnControl) error {
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
	jobCfg := JobConfig{
		ID: jobID,
		Fn: fn,
		Hooks: HooksFunc{
			OnStart:   Hook{Fn: onStartHook},
			OnStop:    Hook{Fn: onStopHook},
			OnError:   Hook{Fn: onErrorHook},
			OnSuccess: Hook{Fn: onSuccessHook},
			OnPause:   Hook{Fn: onPauseHook},
			OnResume:  Hook{Fn: onResumeHook},
			Finally:   Hook{Fn: finallyHook},
		},
	}
	assert.NoError(t, orb.AddJob(p, jobCfg))
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
