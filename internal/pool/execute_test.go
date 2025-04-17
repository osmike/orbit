package pool

import (
	"context"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"go-scheduler/internal/job"
	"go-scheduler/monitoring"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestJobExecute(t *testing.T, poolID string, id string, fn domain.Fn, status domain.JobStatus, ctx context.Context) *job.Job {
	j, err := job.New(poolID, domain.JobDTO{
		ID:       id,
		Interval: domain.Interval{Time: 10 * time.Millisecond},
		Fn:       fn,
	}, ctx)
	assert.NoError(t, err)
	j.SetStatus(status)
	return j
}

func newTestPoolExecute(t *testing.T) *Pool {
	t.Helper()

	p := &Pool{}
	cfg := domain.Pool{
		ID:            "new-pool",
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}
	mon := monitoring.New()

	p, _ = New(context.Background(), cfg, mon)
	return p
}

func TestExecute_Success(t *testing.T) {
	p := newTestPoolExecute(t)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)
	sem <- struct{}{}

	done := make(chan struct{})

	j := newTestJobExecute(t, p.ID, "exec-success", func(ctrl domain.FnControl) error {
		done <- struct{}{}
		return nil
	}, domain.Waiting, p.Ctx)

	p.execute(j, sem, wg)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("job did not execute successfully")
	}

	wg.Wait()

	state := j.GetState()
	assert.Equal(t, domain.Completed, state.Status)
}

func TestExecute_TooEarly(t *testing.T) {
	p := newTestPoolExecute(t)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)

	j := newTestJobExecute(t, p.ID, "exec-too-early", func(ctrl domain.FnControl) error {
		return nil
	}, domain.Waiting, p.Ctx)
	j.StartAt = time.Now().Add(2 * time.Second)

	p.execute(j, sem, wg)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, domain.Waiting, j.GetStatus())
}

func TestExecute_InvalidStatus(t *testing.T) {
	p := newTestPoolExecute(t)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)
	sem <- struct{}{}
	j := newTestJobExecute(t, p.ID, "exec-wrong-status", func(ctrl domain.FnControl) error {
		return nil
	}, domain.Completed, p.Ctx)

	p.execute(j, sem, wg)
	wg.Wait()

	assert.Equal(t, domain.Error, j.GetStatus())
	assert.ErrorIs(t, j.GetState().Error.JobError, errs.ErrJobWrongStatus)
}

func TestExecute_PanicRecovery(t *testing.T) {
	p := newTestPoolExecute(t)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)
	sem <- struct{}{}

	j := newTestJobExecute(t, p.ID, "exec-panic", func(ctrl domain.FnControl) error {
		panic("unexpected panic in job")
	}, domain.Waiting, p.Ctx)

	p.execute(j, sem, wg)
	wg.Wait()

	assert.Equal(t, domain.Error, j.GetStatus())
	assert.ErrorIs(t, j.GetState().Error.JobError, errs.ErrJobPanicked)
}
