package pool

import (
	"context"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"sync"
	"time"
)

type Job interface {
	GetMetadata() domain.JobDTO
	GetStatus() domain.JobStatus
	UpdateStateWithStrict(state domain.StateDTO)
	UpdateState(state domain.StateDTO)
	NextRun() time.Time
	ProcessStart()
	ProcessRun() error
	ProcessEnd(status domain.JobStatus, err error)
	CanExecute() error
	Retry() error
	ExecFunc() error
	Stop()
	Pause() error
	Resume() error
}

type JobData sync.Map

type Pool struct {
	domain.Pool
	jobs   sync.Map
	Ctx    context.Context
	cancel context.CancelFunc
}

func (p *Pool) Init(ctx context.Context, cfg domain.Pool) *Pool {
	pool := &Pool{}
	p.Ctx, p.cancel = context.WithCancel(ctx)
	pool.Pool = cfg
	return pool
}

func (p *Pool) GetJobByID(id string) (Job, error) {
	jobInterface, ok := p.jobs.Load(id)
	if !ok {
		return nil, errs.New(errs.ErrJobNotFound, id)
	}
	return jobInterface.(Job), nil
}

func (p *Pool) Run() {
	ticker := time.NewTicker(p.CheckInterval)
	defer ticker.Stop()
	sem := make(chan struct{}, p.MaxWorkers)
	var wg *sync.WaitGroup
	for {
		select {
		case <-p.Ctx.Done(): // Handle graceful shutdown
			wg.Wait()
			return
		case <-ticker.C: // Periodically process jobs
			p.jobs.Range(func(key, value any) bool {
				job := value.(Job)
				switch job.GetStatus() {
				case domain.Waiting:
					p.processWaiting(job, sem, wg)
				case domain.Running:
					p.processRunning(job)
				case domain.Completed:
					p.processCompleted(job)
				case domain.Error:
					p.processError(job)

				}
				return true
			})
		}
	}

}

func (p *Pool) Stop() {
	p.cancel()
}
