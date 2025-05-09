package pool_test

import (
	"context"
	"github.com/osmike/orbit/internal/domain"
	errs "github.com/osmike/orbit/internal/error"
	"github.com/osmike/orbit/internal/job"
	"github.com/osmike/orbit/internal/pool"
	"github.com/osmike/orbit/test/monitoring"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestJobExecute(id string, fn domain.Fn) domain.JobDTO {
	return domain.JobDTO{
		ID:       id,
		Interval: domain.Interval{Time: 10 * time.Millisecond},
		Fn:       fn,
	}
}

func newTestPoolExecute(t *testing.T) *pool.Pool {
	t.Helper()
	cfg := domain.Pool{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}
	mon := monitoring.New()

	p, err := pool.New(context.Background(), cfg, mon, job.New)
	assert.NoError(t, err)
	return p
}

func TestExecute_Success(t *testing.T) {
	p := newTestPoolExecute(t)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)
	sem <- struct{}{}
	var err error
	var j domain.Job
	done := make(chan struct{})

	jCfg := newTestJobExecute("exec-success", func(ctrl domain.FnControl) error {
		done <- struct{}{}
		return nil
	})

	err = p.AddJob(jCfg)
	j, err = p.GetJobByID(jCfg.ID)
	assert.NoError(t, err)
	p.Execute(j, sem, wg)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("job did not Execute successfully")
	}

	wg.Wait()

	state := j.GetState()
	assert.Equal(t, domain.Completed, state.Status)
}

func TestExecute_TooEarly(t *testing.T) {
	p := newTestPoolExecute(t)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)
	var j domain.Job
	var err error
	jCfg := domain.JobDTO{
		ID:      "exec-too-early",
		Fn:      func(ctrl domain.FnControl) error { return nil },
		StartAt: time.Now().Add(2 * time.Second),
		Interval: domain.Interval{
			Time: 10 * time.Millisecond,
		},
	}
	err = p.AddJob(jCfg)
	j, err = p.GetJobByID(jCfg.ID)
	assert.NoError(t, err)

	p.Execute(j, sem, wg)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, domain.Waiting, j.GetStatus())
}

func TestExecute_PanicRecovery(t *testing.T) {
	p := newTestPoolExecute(t)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)
	sem <- struct{}{}
	var err error
	var j domain.Job

	jCfg := newTestJobExecute("exec-panic", func(ctrl domain.FnControl) error {
		panic("unexpected panic in job")
	})
	err = p.AddJob(jCfg)
	j, err = p.GetJobByID(jCfg.ID)
	assert.NoError(t, err)
	p.Execute(j, sem, wg)
	wg.Wait()

	assert.Equal(t, domain.Error, j.GetStatus())
	assert.ErrorIs(t, j.GetState().Error.JobError, errs.ErrJobPanicked)
}
