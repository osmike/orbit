package orbit_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/osmike/orbit"
	errs "github.com/osmike/orbit/internal/error"
	"github.com/osmike/orbit/test"
	"github.com/osmike/orbit/test/monitoring"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestJob(id string, interval time.Duration, fn func(ctrl orbit.FnControl) error) orbit.Job {
	return orbit.Job{
		ID:       id,
		Interval: orbit.Interval{Time: interval},
		Fn:       fn,
	}
}

func TestSchedulerAPI(t *testing.T) {
	var err error
	for i := 0; i < 3; i++ {
		err := err
		jobID := "job-" + strconv.Itoa(i)

		blockChan := make(chan struct{})
		jobCalled := false

		jobFn := func(ctrl orbit.FnControl) error {
			jobCalled = true
			ctrl.SaveData(map[string]interface{}{"result": i})
			<-blockChan
			return nil
		}

		p, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
			MaxWorkers:    1,
			CheckInterval: 10 * time.Millisecond,
		}, nil)

		assert.NoError(t, err)

		err = p.Run()

		assert.NoError(t, err)

		// Add job
		err = p.AddJob(newTestJob(jobID, 50*time.Millisecond, jobFn))
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		assert.True(t, jobCalled, "job should be started")

		// Stop job
		err = p.StopJob(jobID)
		assert.NoError(t, err)

		// Remove job
		err = p.RemoveJob(jobID)
		assert.NoError(t, err)

		err = p.AddJob(newTestJob(jobID, 50*time.Millisecond, func(ctrl orbit.FnControl) error {
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

	onStart := func(ctrl orbit.FnControl, err error) error {
		ctrl.SaveData(map[string]interface{}{
			"rowCnt":   db.rowCnt,
			"uploaded": 0,
		})
		return nil
	}

	mainFn := func(ctrl orbit.FnControl) error {
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

	mon := monitoring.New()
	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)
	assert.NoError(t, err)
	jobID := "stateful-job"
	job := orbit.Job{
		ID: jobID,
		Fn: mainFn,
		Hooks: orbit.Hooks{
			OnStart: orbit.Hook{
				Fn:          onStart,
				IgnoreError: true,
			},
		},
		Interval: orbit.Interval{Time: 0},
	}

	err = pool.AddJob(job)
	assert.NoError(t, err)

	pool.Run()

	test.WaitForCondition(t, 2*time.Second, func() bool {
		metrics := mon.GetMetrics()
		stateRaw, ok := metrics[jobID]
		if !ok {
			t.Log("no metrics yet")
			return false
		}
		state, ok := stateRaw.(orbit.JobState)
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

	state, ok := stateRaw.(orbit.JobState)
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
	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, monitoring.New())
	assert.NoError(t, err)
	stopped := false

	jobID := "stop-job"
	job := orbit.Job{
		ID: jobID,
		Fn: func(ctrl orbit.FnControl) error {
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
		Interval: orbit.Interval{Time: 0},
	}

	assert.NoError(t, pool.AddJob(job))
	pool.Run()

	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, pool.StopJob(jobID))

	test.WaitForCondition(t, 300*time.Millisecond, func() bool {
		return stopped
	})
}

func TestJobKill(t *testing.T) {
	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, monitoring.New())

	assert.NoError(t, err)

	jobID := "kill-job"
	block := make(chan struct{})

	job := orbit.Job{
		ID: jobID,
		Fn: func(ctrl orbit.FnControl) error {
			select {
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			case <-block:
				t.Fatal("job should have been killed before unblock")
				return nil
			}
		},
		Interval: orbit.Interval{Time: 0},
	}

	assert.NoError(t, pool.AddJob(job))
	pool.Run()

	time.Sleep(50 * time.Millisecond)
	pool.Kill()

	time.Sleep(100 * time.Millisecond)
}

func TestJobPanic_RecoveryAndStatus(t *testing.T) {
	mon := monitoring.New()

	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, mon)
	assert.NoError(t, err)
	job := orbit.Job{
		ID: "panic-job",
		Fn: func(ctrl orbit.FnControl) error {
			panic("unexpected crash!")
		},
		Interval: orbit.Interval{Time: 0},
	}

	assert.NoError(t, pool.AddJob(job))
	pool.Run()

	time.Sleep(100 * time.Millisecond)

	metrics := mon.GetMetrics()
	state := metrics[job.ID].(orbit.JobState)

	assert.Equal(t, orbit.JobStatus("error"), state.Status)
	assert.Contains(t, state.Error.JobError.Error(), "panic: unexpected crash!")
}

func TestSequentialJobExecutionWithLimitedConcurrency(t *testing.T) {

	executed := []string{}
	var mu sync.Mutex

	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, monitoring.New())
	assert.NoError(t, err)
	createJob := func(id string) orbit.Job {
		return orbit.Job{
			ID:       id,
			Name:     fmt.Sprintf("Job %s", id),
			Interval: orbit.Interval{Time: time.Second},
			Fn: func(ctrl orbit.FnControl) error {
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

	assert.NoError(t, pool.AddJob(createJob("job1")))
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, pool.AddJob(createJob("job2")))
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, pool.AddJob(createJob("job3")))

	test.WaitForCondition(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(executed) >= 3
	})

	assert.Equal(t, []string{"job1", "job2", "job3"}, executed)
	pool.Kill()
}

func TestRetryMechanismBehavior(t *testing.T) {
	p, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers:    3,
		CheckInterval: 10 * time.Millisecond,
	}, monitoring.New())
	assert.NoError(t, err)
	fn := func(ctrl orbit.FnControl) error {
		return errors.New("oops")
	}
	intervalCfg := orbit.Interval{
		Time: 50 * time.Millisecond,
	}

	jobsArr := []orbit.Job{
		{
			ID:       "no-retry",
			Fn:       fn,
			Retry:    orbit.Retry{Active: false},
			Interval: intervalCfg,
		},
		{
			ID:       "3-retry",
			Fn:       fn,
			Retry:    orbit.Retry{Active: true, Count: 3},
			Interval: intervalCfg,
		},
		{
			ID:       "infinite-retry",
			Fn:       fn,
			Retry:    orbit.Retry{Active: true},
			Interval: intervalCfg,
		},
	}

	err = p.Run()
	assert.NoError(t, err)
	defer p.Kill()

	for _, job := range jobsArr {
		err = p.AddJob(job)
		assert.NoError(t, err)
	}

	time.Sleep(600 * time.Millisecond)

	metrics := p.GetMetrics()

	state1 := metrics["no-retry"].(orbit.JobState)
	assert.Equal(t, orbit.JobStatus("error"), state1.Status)
	assert.Equal(t, 1, state1.Failure, "no-retry should fail exactly once")

	state2 := metrics["3-retry"].(orbit.JobState)
	assert.Equal(t, orbit.JobStatus("error"), state2.Status)
	assert.Equal(t, 4, state2.Failure, "3-retry should fail once + 3 retries = 4 failures")

	state3 := metrics["infinite-retry"].(orbit.JobState)
	assert.GreaterOrEqual(t, state3.Failure, 5, "infinite-retry should have at least 5 failures")
}

func TestCronJobExecutionAndManualStop(t *testing.T) {

	var counter int
	var mu sync.Mutex

	firstRunDone := make(chan struct{})

	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers:    1,
		CheckInterval: 100 * time.Millisecond,
	}, monitoring.New())
	assert.NoError(t, err)
	jobID := "cron-increment-job"
	jobCfg := orbit.Job{
		ID:   jobID,
		Name: "Increment counter every minute",
		Fn: func(ctrl orbit.FnControl) error {
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
		Interval: orbit.Interval{CronExpr: "*/1 * * * *"},
	}

	err = pool.AddJob(jobCfg)
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

	state, ok := stateRaw.(orbit.JobState)
	assert.True(t, ok, "Expected JobState type")

	assert.Equal(t, orbit.JobStatus("stopped"), state.Status, "Job should be stopped")

	// Counter must be 2 (executed twice)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 2, counter, "Expected counter to be incremented twice")

	pool.Kill()
}

func TestPool_GracefulKill(t *testing.T) {
	ctx := context.Background()
	pool, err := orbit.CreatePool(ctx, orbit.PoolConfig{
		MaxWorkers:    2,
		CheckInterval: 50 * time.Millisecond,
	}, monitoring.New())
	assert.NoError(t, err)
	jobID := "kill-job"
	started := make(chan struct{})

	// Добавляем задачу
	err = pool.AddJob(orbit.Job{
		ID:   jobID,
		Name: "Kill Job Example",
		Fn: func(ctrl orbit.FnControl) error {
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
		Interval: orbit.Interval{Time: 1 * time.Minute}, // чтобы не было автоповтора
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

	state, ok := stateRaw.(orbit.JobState)
	assert.True(t, ok, "Expected JobState type")

	assert.Equal(t, orbit.JobStatus("stopped"), state.Status, "Job should be stopped after kill")
	assert.Equal(t, "pool shutdown", state.Error.JobError.Error(), "Expected pool shutdown error")
}
