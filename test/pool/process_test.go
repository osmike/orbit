package pool_test

import (
	"context"
	"github.com/osmike/orbit/internal/domain"
	"github.com/osmike/orbit/internal/job"
	"github.com/osmike/orbit/internal/pool"
	"github.com/osmike/orbit/test/monitoring"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func newTestJobProcess(retryFlag bool, id string, fn domain.Fn) domain.JobDTO {

	return domain.JobDTO{
		ID:       id,
		Interval: domain.Interval{Time: 20 * time.Millisecond},
		Fn:       fn,
		Retry: domain.Retry{
			Active: retryFlag,
		},
	}
}

func newTestPoolProcess() *pool.Pool {
	p, err := pool.New(context.Background(), domain.Pool{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
	}, monitoring.New(), job.New)
	if err != nil {
		panic(err)
	}
	return p
}

func TestPool_processWaiting_ExecutesIfDue(t *testing.T) {
	p := newTestPoolProcess()
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 1)
	done := make(chan struct{}, 1)
	var err error
	var j domain.Job

	jCfg := newTestJobProcess(false, "wait-now", func(ctrl domain.FnControl) error {
		done <- struct{}{}
		return nil
	})

	err = p.AddJob(jCfg)
	assert.NoError(t, err)
	j, err = p.GetJobByID(jCfg.ID)

	go p.ProcessWaiting(j, sem, wg)

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("job was not executed from waiting state")
	}

	wg.Wait()
	assert.Equal(t, domain.Completed, j.GetStatus())
}

func TestPool_processRunning_MarksErrorOnTimeout(t *testing.T) {
	var err error
	var j domain.Job
	p := newTestPoolProcess()
	jCfg := newTestJobProcess(false, "timeout-job", func(ctrl domain.FnControl) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	err = p.AddJob(jCfg)
	j, err = p.GetJobByID(jCfg.ID)
	assert.NoError(t, err)
	j.SetStatus(domain.Running)

	j.SetTimeout(1 * time.Millisecond)

	// simulate running job exceeding timeout
	time.Sleep(10 * time.Millisecond)
	p.ProcessRunning(j)

	assert.Equal(t, domain.Error, j.GetStatus())
	assert.NotNil(t, j.GetState().Error)
}

func TestPool_processCompleted_MarksEnded(t *testing.T) {
	var err error
	var j domain.Job
	p := newTestPoolProcess()
	jCfg := newTestJobProcess(false, "one-shot-job", func(ctrl domain.FnControl) error {
		return nil
	})
	err = p.AddJob(jCfg)
	j, err = p.GetJobByID(jCfg.ID)
	assert.NoError(t, err)
	j.SetStatus(domain.Running)

	// simulate job scheduled in the past
	j.UpdateState(domain.StateDTO{StartAt: time.Now().Add(-time.Hour)})
	p.ProcessCompleted(j)

	assert.Equal(t, domain.Ended, j.GetStatus())
}

func TestPool_processError_RetriesIfPossible(t *testing.T) {
	var err error
	var j domain.Job
	p := newTestPoolProcess()
	jCfg := newTestJobProcess(true, "retry-job", func(ctrl domain.FnControl) error {
		return nil
	})
	err = p.AddJob(jCfg)
	j, err = p.GetJobByID(jCfg.ID)
	assert.NoError(t, err)
	j.SetStatus(domain.Error)

	jCfg.Retry.Count = 1

	p.ProcessError(j)

	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, domain.Completed, j.GetStatus())
}

func TestPool_processError_SkipsIfRetryLimit(t *testing.T) {
	var err error
	var j domain.Job
	p := newTestPoolProcess()
	jCfg := newTestJobProcess(false, "no-retry", func(ctrl domain.FnControl) error {
		return nil
	})
	jCfg.Retry.Count = 0
	err = p.AddJob(jCfg)
	j, err = p.GetJobByID(jCfg.ID)
	assert.NoError(t, err)
	j.SetStatus(domain.Error)

	p.ProcessError(j)

	assert.Equal(t, domain.Error, j.GetStatus())
}
