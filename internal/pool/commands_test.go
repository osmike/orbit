package pool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"go-scheduler/internal/job"
	"go-scheduler/monitoring"

	"github.com/stretchr/testify/assert"
)

func newTestPoolCommands(t *testing.T) *Pool {
	ctx := context.Background()
	cfg := domain.Pool{
		MaxWorkers:    1,
		CheckInterval: 10 * time.Millisecond,
		IdleTimeout:   time.Second,
	}
	p := new(Pool).Init(ctx, cfg, monitoring.New())
	assert.Equal(t, p.MaxWorkers, cfg.MaxWorkers)
	assert.Equal(t, p.CheckInterval, cfg.CheckInterval)
	assert.Equal(t, p.IdleTimeout, cfg.IdleTimeout)
	return p
}

func newTestJobCommands(t *testing.T, id string, status domain.JobStatus, fn domain.Fn, ctx context.Context) *job.Job {
	j, err := job.New(domain.JobDTO{
		ID:       id,
		Name:     id,
		Fn:       fn,
		Interval: domain.Interval{Time: time.Second},
	}, ctx)
	assert.NoError(t, err)
	j.SetStatus(status)
	return j
}

func TestPoolCommands_AddJob_Success(t *testing.T) {
	p := newTestPoolCommands(t)
	j := newTestJobCommands(t, "job-1", domain.Waiting, func(ctrl domain.FnControl) error { return nil }, p.Ctx)

	err := p.AddJob(j)
	assert.NoError(t, err)
	p.Stop()
}

func TestPoolCommands_AddJob_Duplicate(t *testing.T) {
	p := newTestPoolCommands(t)
	j := newTestJobCommands(t, "job-dup", domain.Waiting, func(ctrl domain.FnControl) error { return nil }, p.Ctx)

	assert.NoError(t, p.AddJob(j))
	err := p.AddJob(j)
	assert.ErrorIs(t, err, errs.ErrIDExists)
	p.Stop()
}

func TestPoolCommands_AddJob_InvalidStatus(t *testing.T) {
	p := newTestPoolCommands(t)
	j := newTestJobCommands(t, "job-invalid", domain.Running, func(ctrl domain.FnControl) error { return nil }, p.Ctx)

	err := p.AddJob(j)
	assert.ErrorIs(t, err, errs.ErrJobWrongStatus)
	p.Stop()
}

func TestPoolCommands_RemoveJob_Success(t *testing.T) {
	p := newTestPoolCommands(t)
	j := newTestJobCommands(t, "job-remove", domain.Waiting, func(ctrl domain.FnControl) error { return nil }, p.Ctx)

	assert.NoError(t, p.AddJob(j))
	err := p.RemoveJob("job-remove")
	assert.NoError(t, err)
	p.Stop()
}

func TestPoolCommands_RemoveJob_NotFound(t *testing.T) {
	p := newTestPoolCommands(t)
	err := p.RemoveJob("not-exist")
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
}

//func TestPoolCommands_PauseJob_Success(t *testing.T) {
//	p := newTestPoolCommands(t)
//
//	j := newTestJobCommands(t, "job-pause", domain.Running, func(ctrl domain.FnControl) error {
//		<-ctrl.PauseChan()
//		return nil
//	}, p.Ctx)
//
//	assert.NoError(t, p.AddJob(j))
//
//	go func() {
//		<-j.PauseChan()
//	}()
//
//	err := p.PauseJob("job-pause", 200*time.Millisecond)
//	assert.NoError(t, err)
//	p.Stop()
//}

func TestPoolCommands_PauseJob_NotFound(t *testing.T) {
	p := newTestPoolCommands(t)
	err := p.PauseJob("not-exist", 0)
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
	p.Stop()
}

func TestPoolCommands_ResumeJob_Success(t *testing.T) {
	p := newTestPoolCommands(t)
	resumed := make(chan struct{}, 1)
	var err error
	jID := "job-work-flow"
	j := newTestJobCommands(t, jID, domain.Waiting, func(ctrl domain.FnControl) error {
		for {
			select {
			case <-ctrl.PauseChan():
				<-ctrl.ResumeChan()
				resumed <- struct{}{}
			case <-ctrl.Context().Done():
				return ctrl.Context().Err()
			default:
				fmt.Println("hi")
				time.Sleep(500 * time.Millisecond)
			}
		}
	}, p.Ctx)

	assert.NoError(t, p.AddJob(j))
	go p.Run()
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

	p.Stop()
}

func TestPoolCommands_ResumeJob_NotFound(t *testing.T) {
	p := newTestPoolCommands(t)
	err := p.ResumeJob("not-exist")
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
	p.Stop()
}

func TestPoolCommands_StopJob_Success(t *testing.T) {
	p := newTestPoolCommands(t)

	j := newTestJobCommands(t, "job-stop", domain.Running, func(ctrl domain.FnControl) error {
		<-ctrl.Context().Done()
		return ctrl.Context().Err()
	}, p.Ctx)

	assert.NoError(t, p.AddJob(j))

	err := p.StopJob("job-stop")
	assert.NoError(t, err)
	p.Stop()
}

func TestPoolCommands_StopJob_NotFound(t *testing.T) {
	p := newTestPoolCommands(t)
	err := p.StopJob("not-exist")
	assert.ErrorIs(t, err, errs.ErrJobNotFound)
	p.Stop()
}

func TestPoolCommands_StopPool(t *testing.T) {
	p := newTestPoolCommands(t)
	p.Stop()
	assert.NotNil(t, p.Ctx.Err())
}
