package orbit

import (
	"context"
	"errors"
	"fmt"
	errs "orbit/internal/error"
	"orbit/test"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestJob(id string, interval time.Duration, fn func(ctrl FnControl) error) Job {
	return Job{
		ID:       id,
		Interval: Interval{Time: interval},
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

		p := scheduler.CreatePool(PoolConfig{
			MaxWorkers:    1,
			CheckInterval: 10 * time.Millisecond,
		}, nil)

		assert.NoError(t, err)

		err = p.Run()

		assert.NoError(t, err)

		// Add job
		err = scheduler.AddJob(p, newTestJob(jobID, 50*time.Millisecond, jobFn))
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		assert.True(t, jobCalled, "job should be started")

		// Stop job
		err = p.StopJob(jobID)
		assert.NoError(t, err)

		// Remove job
		err = p.RemoveJob(jobID)
		assert.NoError(t, err)

		err = scheduler.AddJob(p, newTestJob(jobID, 50*time.Millisecond, func(ctrl FnControl) error {
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
				data := ctrl.GetData()

				uploaded := test.ToInt(data["uploaded"])
				total := test.ToInt(data["rowCnt"])
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
	pool := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)

	jobID := "stateful-job"
	job := Job{
		ID: jobID,
		Fn: mainFn,
		Hooks: Hooks{
			OnStart: Hook{
				Fn:          onStart,
				IgnoreError: true,
			},
		},
		Interval: Interval{Time: 0},
	}

	err := scheduler.AddJob(pool, job)
	assert.NoError(t, err)

	pool.Run()

	test.WaitForCondition(t, 2*time.Second, func() bool {
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
		v := test.ToInt(vRaw)
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

	assert.Equal(t, 9000, test.ToInt(state.Data["uploaded"]))
	assert.Equal(t, 9000, test.ToInt(state.Data["rowCnt"]))
	assert.True(t, db.reconnects >= 1, "Should reconnect at least once after pause")

	err = pool.StopJob(jobID)
	assert.NoError(t, err)

	err = pool.RemoveJob(jobID)
	assert.NoError(t, err)
}

func TestJobStop(t *testing.T) {
	scheduler := New(context.Background())
	pool := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, newDefaultMon())

	stopped := false

	jobID := "stop-job"
	job := Job{
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
		Interval: Interval{Time: 0},
	}

	assert.NoError(t, scheduler.AddJob(pool, job))
	pool.Run()

	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, pool.StopJob(jobID))

	test.WaitForCondition(t, 300*time.Millisecond, func() bool {
		return stopped
	})
}

func TestJobKill(t *testing.T) {
	scheduler := New(context.Background())
	pool := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, newDefaultMon())

	jobID := "kill-job"
	block := make(chan struct{})

	job := Job{
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
		Interval: Interval{Time: 0},
	}

	assert.NoError(t, scheduler.AddJob(pool, job))
	pool.Run()

	time.Sleep(50 * time.Millisecond)
	pool.Kill()

	time.Sleep(100 * time.Millisecond)
}

func TestJobPanic_RecoveryAndStatus(t *testing.T) {
	scheduler := New(context.Background())
	mon := newDefaultMon()

	pool := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)

	job := Job{
		ID: "panic-job",
		Fn: func(ctrl FnControl) error {
			panic("unexpected crash!")
		},
		Interval: Interval{Time: 0},
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

	pool := scheduler.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, newDefaultMon())

	createJob := func(id string) Job {
		return Job{
			ID:       id,
			Name:     fmt.Sprintf("Job %s", id),
			Interval: Interval{Time: time.Second},
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

	pool.Run()

	assert.NoError(t, scheduler.AddJob(pool, createJob("job1")))
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, scheduler.AddJob(pool, createJob("job2")))
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, scheduler.AddJob(pool, createJob("job3")))

	test.WaitForCondition(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(executed) >= 3
	})

	assert.Equal(t, []string{"job1", "job2", "job3"}, executed)
	pool.Kill()
}

func TestRetryMechanismBehavior(t *testing.T) {
	orb := New(context.Background())
	p := orb.CreatePool(PoolConfig{
		MaxWorkers:    3,
		CheckInterval: 10 * time.Millisecond,
	}, newDefaultMon())

	fn := func(ctrl FnControl) error {
		return errors.New("oops")
	}
	intervalCfg := Interval{
		Time: 50 * time.Millisecond,
	}

	jobsArr := []Job{
		{
			ID:       "no-retry",
			Fn:       fn,
			Retry:    Retry{Active: false},
			Interval: intervalCfg,
		},
		{
			ID:       "3-retry",
			Fn:       fn,
			Retry:    Retry{Active: true, Count: 3},
			Interval: intervalCfg,
		},
		{
			ID:       "infinite-retry",
			Fn:       fn,
			Retry:    Retry{Active: true},
			Interval: intervalCfg,
		},
	}

	err := p.Run()
	assert.NoError(t, err)
	defer p.Kill()

	for _, job := range jobsArr {
		err = orb.AddJob(p, job)
		assert.NoError(t, err)
	}

	time.Sleep(600 * time.Millisecond)

	metrics := p.GetMetrics()

	state1 := metrics["no-retry"].(JobState)
	assert.Equal(t, JobStatus("error"), state1.Status)
	assert.Equal(t, 1, state1.Failure, "no-retry should fail exactly once")

	state2 := metrics["3-retry"].(JobState)
	assert.Equal(t, JobStatus("error"), state2.Status)
	assert.Equal(t, 4, state2.Failure, "3-retry should fail once + 3 retries = 4 failures")

	state3 := metrics["infinite-retry"].(JobState)
	assert.GreaterOrEqual(t, state3.Failure, 5, "infinite-retry should have at least 5 failures")
}

func TestCronJobExecutionAndManualStop(t *testing.T) {
	orb := New(context.Background())

	var counter int
	var mu sync.Mutex

	firstRunDone := make(chan struct{})

	pool := orb.CreatePool(PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 100 * time.Millisecond,
	}, newDefaultMon())

	jobID := "cron-increment-job"
	jobCfg := Job{
		ID:   jobID,
		Name: "Increment counter every minute",
		Fn: func(ctrl FnControl) error {
			mu.Lock()
			fmt.Printf("[%s] started\n", jobID)
			counter++
			mu.Unlock()

			select {
			case firstRunDone <- struct{}{}: // Signal first execution
			default:
			}

			return nil
		},
		Interval: Interval{CronExpr: "*/1 * * * *"},
	}

	err := orb.AddJob(pool, jobCfg)
	assert.NoError(t, err)

	err = pool.Run()
	assert.NoError(t, err)

	// Wait up to 70 seconds for the first execution
	select {
	case <-firstRunDone:
		// First run happened
	case <-time.After(70 * time.Second):
		t.Fatal("First cron execution did not happen within expected time")
	}

	// Wait another ~70 seconds for second execution
	time.Sleep(70 * time.Second)

	// Now stop the job
	err = pool.StopJob(jobID)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	metrics := pool.GetMetrics()
	stateRaw, ok := metrics[jobID]
	assert.True(t, ok, "Expected to find job metrics")

	state, ok := stateRaw.(JobState)
	assert.True(t, ok, "Expected JobState type")

	assert.Equal(t, JobStatus("stopped"), state.Status, "Job should be stopped")

	// Counter must be 2 (executed twice)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 2, counter, "Expected counter to be incremented twice")

	pool.Kill()
}

func TestPool_GracefulKill(t *testing.T) {
	ctx := context.Background()
	orb := New(ctx)

	pool := orb.CreatePool(PoolConfig{
		MaxWorkers:    2,
		CheckInterval: 50 * time.Millisecond,
	}, newDefaultMon())

	jobID := "kill-job"
	started := make(chan struct{})

	// Добавляем задачу
	err := orb.AddJob(pool, Job{
		ID:   jobID,
		Name: "Kill Job Example",
		Fn: func(ctrl FnControl) error {
			fmt.Println("[kill-job] started")
			close(started)
			select {
			case <-time.After(5 * time.Second):
				fmt.Println("[kill-job] completed")
				return nil
			case <-ctrl.Context().Done():
				fmt.Println("[kill-job] canceled by kill")
				return ctrl.Context().Err()
			}
		},
		Interval: Interval{Time: 1 * time.Minute}, // чтобы не было автоповтора
	})
	assert.NoError(t, err)

	err = pool.Run()
	assert.NoError(t, err)

	select {
	case <-started:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("Job did not start in time")
	}

	fmt.Println("Killing pool now...")
	pool.Kill()

	time.Sleep(200 * time.Millisecond)

	metrics := pool.GetMetrics()
	stateRaw, ok := metrics[jobID]
	assert.True(t, ok, "Expected to find job metrics")

	state, ok := stateRaw.(JobState)
	assert.True(t, ok, "Expected JobState type")

	assert.Equal(t, JobStatus("stopped"), state.Status, "Job should be stopped after kill")
	assert.Equal(t, "pool shutdown", state.Error.JobError.Error(), "Expected pool shutdown error")
}
