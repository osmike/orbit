package pool

import (
	"context"
	"github.com/stretchr/testify/assert"
	"orbit/internal/domain"
	"orbit/internal/job"
	"orbit/monitoring"
	"sync"
	"testing"
	"time"
)

func newTestJobProcess(t *testing.T, retryFlag bool, id string, status domain.JobStatus, fn domain.Fn, ctx context.Context, mon domain.Monitoring) *job.Job {
	j, err := job.New(domain.JobDTO{
		ID:       id,
		Interval: domain.Interval{Time: 20 * time.Millisecond},
		Fn:       fn,
		Retry: domain.Retry{
			Active: retryFlag,
		},
	}, ctx, mon)
	assert.NoError(t, err)
	j.SetStatus(status)
	return j
}

func newTestPoolProcess() *Pool {
	p := New(context.Background(), domain.Pool{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, monitoring.New())
	return p
}

func TestPool_processWaiting_ExecutesIfDue(t *testing.T) {
	p := newTestPoolProcess()
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)
	done := make(chan struct{}, 1)

	j := newTestJobProcess(t, false, "wait-now", domain.Waiting, func(ctrl domain.FnControl) error {
		done <- struct{}{}
		return nil
	}, p.Ctx, p.Mon)

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
	p := newTestPoolProcess()
	j := newTestJobProcess(t, false, "timeout-job", domain.Running, func(ctrl domain.FnControl) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}, p.Ctx, p.Mon)
	j.SetTimeout(1 * time.Millisecond)

	// simulate running job exceeding timeout
	time.Sleep(10 * time.Millisecond)
	p.processRunning(j)

	assert.Equal(t, domain.Error, j.GetStatus())
	assert.NotNil(t, j.GetState().Error)
}

func TestPool_processCompleted_MarksEnded(t *testing.T) {
	p := newTestPoolProcess()
	j := newTestJobProcess(t, false, "one-shot-job", domain.Running, func(ctrl domain.FnControl) error {
		return nil
	}, p.Ctx, p.Mon)

	// simulate job scheduled in the past
	j.UpdateState(domain.StateDTO{StartAt: time.Now().Add(-time.Hour)})
	p.processCompleted(j)

	assert.Equal(t, domain.Ended, j.GetStatus())
}

func TestPool_processError_RetriesIfPossible(t *testing.T) {
	p := newTestPoolProcess()
	j := newTestJobProcess(t, true, "retry-job", domain.Error, func(ctrl domain.FnControl) error {
		return nil
	}, p.Ctx, p.Mon)

	j.JobDTO.Retry.Count = 1

	p.processError(j)

	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, domain.Completed, j.GetStatus())
}

func TestPool_processError_SkipsIfRetryLimit(t *testing.T) {
	p := newTestPoolProcess()
	j := newTestJobProcess(t, false, "no-retry", domain.Error, func(ctrl domain.FnControl) error {
		return nil
	}, p.Ctx, p.Mon)

	j.JobDTO.Retry.Count = 0

	p.processError(j)

	assert.Equal(t, domain.Error, j.GetStatus())
}
