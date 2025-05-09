package pool_test

import (
	"context"
	"github.com/osmike/orbit/internal/pool"
	"github.com/osmike/orbit/test/monitoring"
	"testing"
	"time"

	"github.com/osmike/orbit/internal/domain"
	errs "github.com/osmike/orbit/internal/error"
	"github.com/osmike/orbit/internal/job"
	"github.com/stretchr/testify/assert"
)

func newTestPoolCommands(t *testing.T) *pool.Pool {
	ctx := context.Background()
	cfg := domain.Pool{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
		IdleTimeout:   5 * time.Second,
	}
	p, _ := pool.New(ctx, cfg, monitoring.New(), job.New)
	assert.Equal(t, p.MaxWorkers, cfg.MaxWorkers)
	assert.Equal(t, p.CheckInterval, cfg.CheckInterval)
	assert.Equal(t, p.IdleTimeout, cfg.IdleTimeout)
	return p
}

func newTestJobCommands(jobID string, fn domain.Fn) domain.JobDTO {

	return domain.JobDTO{
		ID:       jobID,
		Name:     jobID,
		Fn:       fn,
		Interval: domain.Interval{Time: time.Second},
	}
}

func TestPoolCommands_AddJob_Success(t *testing.T) {
	p := newTestPoolCommands(t)
	defer p.Kill()
	j := newTestJobCommands("job-1", func(ctrl domain.FnControl) error { return nil })

	err := p.AddJob(j)
	assert.NoError(t, err)
}

func TestPoolCommands_AddJob_Duplicate(t *testing.T) {
	p := newTestPoolCommands(t)
	defer p.Kill()
	j := newTestJobCommands("job-dup", func(ctrl domain.FnControl) error { return nil })

	assert.NoError(t, p.AddJob(j))
	err := p.AddJob(j)
	assert.ErrorIs(t, err, errs.ErrIDExists)
}

func TestPoolCommands_RemoveJob_Success(t *testing.T) {
	p := newTestPoolCommands(t)
	defer p.Kill()
	jID := "job-remove"
	j := newTestJobCommands(jID, func(ctrl domain.FnControl) error { return nil })

	assert.NoError(t, p.AddJob(j))
	err := p.RemoveJob(jID)
	assert.NoError(t, err)
	_, err = p.GetJobByID(jID)
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
}

func TestPoolCommands_RemoveJob_NotFound(t *testing.T) {
	p := newTestPoolCommands(t)
	err := p.RemoveJob("not-exist")
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
}

func TestPoolCommands_PauseJob_NotFound(t *testing.T) {
	p := newTestPoolCommands(t)
	defer p.Kill()
	err := p.PauseJob("not-exist", 0)
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
}

func TestPoolCommands_WorkFlowJob_Success(t *testing.T) {
	p := newTestPoolCommands(t)
	defer p.Kill()
	resumed := make(chan struct{}, 1)
	var err error
	jID := "job-work-flow"
	var j domain.Job
	jCfg := newTestJobCommands(jID, func(ctrl domain.FnControl) error {
		for {
			select {
			case <-ctrl.PauseChan():
				<-ctrl.ResumeChan()
				resumed <- struct{}{}
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			default:
				time.Sleep(200 * time.Millisecond)
			}
		}
	})

	assert.NoError(t, p.AddJob(jCfg))
	err = p.Run()
	assert.NoError(t, err)
	j, err = p.GetJobByID(jID)
	assert.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, domain.Running, j.GetStatus())

	err = p.PauseJob(jID, 200*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, domain.Paused, j.GetStatus())

	err = p.ResumeJob(jID)
	time.Sleep(10 * time.Millisecond)
	assert.NoError(t, err)
	select {
	case <-resumed:
		assert.Equal(t, domain.Running, j.GetStatus())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected job to be resumed")
	}

	err = p.StopJob(jID)
	time.Sleep(10 * time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, domain.Stopped, j.GetStatus())
}

func TestPoolCommands_ResumeJob_NotFound(t *testing.T) {
	p := newTestPoolCommands(t)
	defer p.Kill()
	err := p.ResumeJob("not-exist")
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
}

func TestPoolCommands_StopJob_Success(t *testing.T) {
	p := newTestPoolCommands(t)
	defer p.Kill()
	var err error
	var j domain.Job
	jID := "job-stop"
	jCfg := newTestJobCommands(jID, func(ctrl domain.FnControl) error {
		<-ctrl.Context().Done()
		return ctrl.Context().Err()
	})
	assert.NoError(t, p.AddJob(jCfg))
	err = p.Run()
	assert.NoError(t, err)
	err = p.StopJob(jID)
	assert.NoError(t, err)
	j, err = p.GetJobByID(jID)
	assert.NoError(t, err)
	status := j.GetStatus()
	assert.Equal(t, domain.Stopped, status)
}

func TestPoolCommands_StopJob_NotFound(t *testing.T) {
	p := newTestPoolCommands(t)
	defer p.Kill()
	err := p.StopJob("not-exist")
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
}
