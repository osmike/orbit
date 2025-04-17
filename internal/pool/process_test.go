package pool

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go-scheduler/internal/domain"
	"go-scheduler/internal/job"
	"go-scheduler/monitoring"
	"sync"
	"testing"
	"time"
)

func newTestJobProcess(t *testing.T, poolID string, id string, status domain.JobStatus, fn domain.Fn, ctx context.Context) *job.Job {
	j, err := job.New(poolID, domain.JobDTO{
		ID:       id,
		Interval: domain.Interval{Time: 20 * time.Millisecond},
		Fn:       fn,
	}, ctx)
	assert.NoError(t, err)
	j.SetStatus(status)
	return j
}

func newTestPoolProcess(t *testing.T) *Pool {
	p, _ := New(context.Background(), domain.Pool{
		ID:            "new-pool",
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, monitoring.New())
	return p
}

func TestPool_processWaiting_ExecutesIfDue(t *testing.T) {
	p := newTestPoolProcess(t)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)
	done := make(chan struct{}, 1)

	j := newTestJobProcess(t, p.ID, "wait-now", domain.Waiting, func(ctrl domain.FnControl) error {
		done <- struct{}{}
		return nil
	}, p.Ctx)

	go p.processWaiting(j, sem, wg)

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("job was not executed from waiting state")
	}

	wg.Wait()
	assert.Equal(t, domain.Completed, j.GetStatus())
}

func TestPool_processRunning_MarksErrorOnTimeout(t *testing.T) {
	p := newTestPoolProcess(t)
	j := newTestJobProcess(t, p.ID, "timeout-job", domain.Running, func(ctrl domain.FnControl) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}, p.Ctx)
	j.SetTimeout(1 * time.Millisecond)

	// simulate running job exceeding timeout
	time.Sleep(10 * time.Millisecond)
	p.processRunning(j)

	assert.Equal(t, domain.Error, j.GetStatus())
	assert.NotNil(t, j.GetState().Error)
}

func TestPool_processCompleted_MarksEnded(t *testing.T) {
	p := newTestPoolProcess(t)
	j := newTestJobProcess(t, p.ID, "one-shot-job", domain.Running, func(ctrl domain.FnControl) error {
		return nil
	}, p.Ctx)

	// simulate job scheduled in the past
	j.UpdateState(domain.StateDTO{StartAt: time.Now().Add(-time.Hour)})
	p.processCompleted(j)

	assert.Equal(t, domain.Ended, j.GetStatus())
}

func TestPool_processError_RetriesIfPossible(t *testing.T) {
	p := newTestPoolProcess(t)
	j := newTestJobProcess(t, p.ID, "retry-job", domain.Error, func(ctrl domain.FnControl) error {
		return nil
	}, p.Ctx)

	j.JobDTO.Retry.Count = 1

	p.processError(j)

	assert.Equal(t, domain.Waiting, j.GetStatus())
}

func TestPool_processError_SkipsIfRetryLimit(t *testing.T) {
	p := newTestPoolProcess(t)
	j := newTestJobProcess(t, p.ID, "no-retry", domain.Error, func(ctrl domain.FnControl) error {
		return nil
	}, p.Ctx)

	j.JobDTO.Retry.Count = 0

	p.processError(j)

	assert.Equal(t, domain.Error, j.GetStatus())
}
