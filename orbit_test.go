package orbit

import (
	"context"
	"errors"
	"fmt"
	errs "orbit/internal/error"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestJobConfig(id string, interval time.Duration, fn func(ctrl FnControl) error) JobConfig {
	return JobConfig{
		ID:       id,
		Interval: IntervalConfig{Time: interval},
		Fn:       fn,
	}
}

func TestSchedulerAPI(t *testing.T) {
	scheduler := New(context.Background())
	var err error
	for i := 0; i < 3; i++ {
		err := err
		jobID := "job-" + strconv.Itoa(i)

		blockChan := make(chan struct{})
		jobCalled := false

		jobFn := func(ctrl FnControl) error {
			jobCalled = true
			ctrl.SaveData(map[string]interface{}{"result": i})
			<-blockChan
			return nil
		}

		p, err := scheduler.CreatePool(PoolConfig{
			MaxWorkers:    1,
			CheckInterval: 10 * time.Millisecond,
		}, nil)

		assert.NoError(t, err)

		err = p.Run()

		assert.NoError(t, err)

		// Add job
		err = scheduler.AddJob(p, newTestJobConfig(jobID, 50*time.Millisecond, jobFn))
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		assert.True(t, jobCalled, "job should be started")

		// Stop job
		err = p.StopJob(jobID)
		assert.NoError(t, err)

		// Remove job
		err = p.RemoveJob(jobID)
		assert.NoError(t, err)

		err = scheduler.AddJob(p, newTestJobConfig(jobID, 50*time.Millisecond, func(ctrl FnControl) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}))
		assert.NoError(t, err)

		time.Sleep(150 * time.Millisecond)
		close(blockChan)

		// Kill pool
		p.Kill()
		time.Sleep(200 * time.Millisecond)

		// Try re-running killed pool
		err = p.Run()
		assert.ErrorIs(t, err, errs.ErrPoolShutdown)

	}
}

func TestJobWithPauseResumeAndState(t *testing.T) {
	scheduler := New(context.Background())

	db := struct {
		rowCnt     int
		uploaded   int
		batchSize  int
		reconnects int
	}{
		rowCnt:    9000,
		uploaded:  0,
		batchSize: 1000,
	}

	onStart := func(ctrl FnControl, err error) error {
		ctrl.SaveData(map[string]interface{}{
			"rowCnt":   db.rowCnt,
			"uploaded": 0,
		})
		return nil
	}

	mainFn := func(ctrl FnControl) error {
		for {
			select {
			case <-ctrl.PauseChan():
				<-ctrl.ResumeChan()
				db.reconnects++
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			default:
				state, err := ctrl.GetData()
				if err != nil {
					return err
				}

				data := state.Data
				uploaded := toInt(data["uploaded"])
				total := toInt(data["rowCnt"])
				time.Sleep(100 * time.Millisecond)
				uploaded += db.batchSize
				fmt.Printf("Uploading: %d / %d\n", uploaded, total)

				ctrl.SaveData(map[string]interface{}{
					"rowCnt":   total,
					"uploaded": uploaded,
				})

				if uploaded >= total {
					return nil
				}
			}
		}
	}

	mon := newDefaultMon()
	pool, err := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)
	assert.NoError(t, err)

	jobID := "stateful-job"
	job := JobConfig{
		ID: jobID,
		Fn: mainFn,
		Hooks: HooksFunc{
			OnStart: Hook{
				Fn:          onStart,
				IgnoreError: true,
			},
		},
		Interval: IntervalConfig{Time: 0},
	}

	err = scheduler.AddJob(pool, job)
	assert.NoError(t, err)

	pool.Run()

	// Ждём, пока job реально начнёт загружать данные
	waitForCondition(t, 2*time.Second, func() bool {
		metrics := mon.GetMetrics()
		stateRaw, ok := metrics[jobID]
		if !ok {
			t.Log("no metrics yet")
			return false
		}
		state, ok := stateRaw.(JobState)
		if !ok {
			t.Log("bad cast to JobState")
			return false
		}
		vRaw, ok := state.Data["uploaded"]
		if !ok {
			t.Log("no uploaded in state")
			return false
		}
		v := toInt(vRaw)
		t.Logf("uploaded = %d", v)
		return v >= 1000
	})

	err = pool.PauseJob(jobID, 100*time.Millisecond)
	assert.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	err = pool.ResumeJob(jobID)
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)

	metrics := mon.GetMetrics()
	stateRaw, ok := metrics[jobID]
	assert.True(t, ok, "Expected metrics for job: %s", jobID)

	state, ok := stateRaw.(JobState)
	assert.True(t, ok, "Expected JobState in metrics")

	assert.Equal(t, 9000, toInt(state.Data["uploaded"]))
	assert.Equal(t, 9000, toInt(state.Data["rowCnt"]))
	assert.True(t, db.reconnects >= 1, "Should reconnect at least once after pause")

	err = pool.StopJob(jobID)
	assert.NoError(t, err)
	err = pool.RemoveJob(jobID)
	assert.NoError(t, err)
}

func TestJobStop(t *testing.T) {
	scheduler := New(context.Background())
	pool, err := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, newDefaultMon())
	assert.NoError(t, err)

	stopped := false

	jobID := "stop-job"
	job := JobConfig{
		ID: jobID,
		Fn: func(ctrl FnControl) error {
			for {
				select {
				case <-ctrl.Context().Done():
					stopped = true
					return nil
				default:
					time.Sleep(10 * time.Millisecond)
				}
			}
		},
		Interval: IntervalConfig{Time: 0},
	}

	assert.NoError(t, scheduler.AddJob(pool, job))
	pool.Run()

	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, pool.StopJob(jobID))

	waitForCondition(t, 300*time.Millisecond, func() bool {
		return stopped
	})
}

func TestJobKill(t *testing.T) {
	scheduler := New(context.Background())
	pool, err := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, newDefaultMon())
	assert.NoError(t, err)

	jobID := "kill-job"
	block := make(chan struct{})

	job := JobConfig{
		ID: jobID,
		Fn: func(ctrl FnControl) error {
			select {
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			case <-block:
				t.Fatal("job should have been killed before unblock")
				return nil
			}
		},
		Interval: IntervalConfig{Time: 0},
	}

	assert.NoError(t, scheduler.AddJob(pool, job))
	pool.Run()

	time.Sleep(50 * time.Millisecond)
	pool.Kill()

	time.Sleep(100 * time.Millisecond)
}

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

	waitForCondition(t, 1*time.Second, func() bool {
		return capturedErr != nil
	})

	assert.NotNil(t, capturedErr)
	assert.Contains(t, capturedErr.Error(), "simulated failure")
}

func TestJobPanic_RecoveryAndStatus(t *testing.T) {
	scheduler := New(context.Background())
	mon := newDefaultMon()

	pool, err := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)
	assert.NoError(t, err)

	job := JobConfig{
		ID: "panic-job",
		Fn: func(ctrl FnControl) error {
			panic("unexpected crash!")
		},
		Interval: IntervalConfig{Time: 0},
	}

	assert.NoError(t, scheduler.AddJob(pool, job))
	pool.Run()

	time.Sleep(100 * time.Millisecond)

	metrics := mon.GetMetrics()
	state := metrics[job.ID].(JobState)

	assert.Equal(t, JobStatus("error"), state.Status)
	assert.Contains(t, state.Error.JobError.Error(), "panic: unexpected crash!")
}

func TestSequentialJobExecutionWithLimitedConcurrency(t *testing.T) {
	scheduler := New(context.Background())

	executed := []string{}
	var mu sync.Mutex

	pool, err := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,                     // только одна джоба одновременно
		CheckInterval: 10 * time.Millisecond, // частая проверка
	}, newDefaultMon())
	assert.NoError(t, err)

	createJob := func(id string) JobConfig {
		return JobConfig{
			ID:       id,
			Name:     fmt.Sprintf("Job %s", id),
			Interval: IntervalConfig{Time: time.Second},
			Fn: func(ctrl FnControl) error {
				mu.Lock()
				executed = append(executed, id)
				mu.Unlock()

				fmt.Printf("[%s] started\n", id)
				time.Sleep(200 * time.Millisecond)
				fmt.Printf("[%s] finished\n", id)
				return nil
			},
		}
	}

	assert.NoError(t, scheduler.AddJob(pool, createJob("job1")))
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, scheduler.AddJob(pool, createJob("job2")))
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, scheduler.AddJob(pool, createJob("job3")))

	pool.Run()

	waitForCondition(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(executed) >= 3
	})

	assert.Equal(t, []string{"job1", "job2", "job3"}, executed)
}
