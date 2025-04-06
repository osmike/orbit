package pool

import (
	"context"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"go-scheduler/internal/job"
	"go-scheduler/monitoring"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestJob(t *testing.T, id string) *job.Job {
	ctx := context.Background()
	j, err := job.New(domain.JobDTO{
		ID:       id,
		Name:     id,
		Interval: domain.Interval{Time: time.Millisecond * 100},
		Fn: func(ctrl domain.FnControl) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}, ctx)
	assert.NoError(t, err)
	return j
}

func TestPool_InitDefaults(t *testing.T) {
	ctx := context.Background()
	mon := monitoring.New()

	cfg := domain.Pool{}
	p := new(Pool).Init(ctx, cfg, mon)

	assert.Equal(t, domain.DEFAULT_NUM_WORKERS, p.MaxWorkers)
	assert.Equal(t, domain.DEFAULT_CHECK_INTERVAL, p.CheckInterval)
	assert.Equal(t, domain.DEFAULT_IDLE_TIMEOUT, p.IdleTimeout)
	assert.Equal(t, mon, p.mon)
	assert.NotNil(t, p.Ctx)
	assert.NotNil(t, p.cancel)
}

func TestPool_getJobByID_Success(t *testing.T) {
	p := &Pool{}
	j := newTestJob(t, "job-1")
	p.jobs.Store("job-1", j)

	found, err := p.getJobByID("job-1")
	assert.NoError(t, err)
	assert.Equal(t, "job-1", found.GetMetadata().ID)
}

func TestPool_getJobByID_NotFound(t *testing.T) {
	p := &Pool{}
	_, err := p.getJobByID("not-found")
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
}

func TestPool_Run_ExecutesWaitingJobs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := new(Pool).Init(ctx, domain.Pool{
		MaxWorkers:    2,
		CheckInterval: 10 * time.Millisecond,
		IdleTimeout:   time.Second,
	}, monitoring.New())

	j := newTestJob(t, "run-job-1")
	p.jobs.Store("run-job-1", j)

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	p.Run()

	state := j.GetState()
	assert.Equal(t, domain.Completed, state.Status)
	assert.False(t, state.EndAt.IsZero())
	assert.Greater(t, state.ExecutionTime, int64(0))
}
