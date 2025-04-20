package orbit

import (
	"context"
	errs "go-scheduler/internal/error"
	"strconv"
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
