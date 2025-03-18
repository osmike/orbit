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
	ProcessStart(start time.Time)
	ProcessRun() error
	ProcessEnd(start time.Time, status domain.JobStatus, err error)
	CanExecute() error
	ExecFunc() error
	Stop()
}

type JobData sync.Map

type Pool struct {
	domain.Pool
	History sync.Map
	Jobs    sync.Map
	ctx     context.Context
	cancel  context.CancelFunc
}

func (p *Pool) Init(ctx context.Context, cfg domain.Pool) *Pool {
	pool := &Pool{}
	p.ctx, p.cancel = context.WithCancel(ctx)
	pool.Pool = cfg
	return pool
}

func (p *Pool) GetJobByID(id string) (Job, error) {
	jobInterface, ok := p.Jobs.Load(id)
	if !ok {
		return nil, errs.New(errs.ErrJobNotFound, id)
	}
	return jobInterface.(Job), nil
}

func (p *Pool) AddJob(job Job) error {
	meta := job.GetMetadata()
	if _, ok := p.Jobs.Load(meta.ID); ok {
		return errs.New(errs.ErrIDExists, meta.ID)
	}
	if job.GetStatus() != domain.Waiting {
		return errs.New(errs.ErrJobWrongStatus, meta.ID)
	}
	p.Jobs.Store(meta.ID, job)
	return nil
}

func (p *Pool) RemoveJob(id string) error {
	job, err := p.GetJobByID(id)
	if err != nil {
		return err
	}
	job.Stop()
	p.Jobs.Delete(id)
	return nil
}

func (p *Pool) Run() {
	ticker := time.NewTicker(p.CheckInterval)
	defer ticker.Stop()
	sem := make(chan struct{}, p.MaxWorkers)
	var wg *sync.WaitGroup
	for {
		select {
		case <-p.ctx.Done(): // Handle graceful shutdown
			wg.Wait()
			return
		case <-ticker.C: // Periodically process jobs
			p.Jobs.Range(func(key, value any) bool {
				job := value.(Job)
				switch job.GetStatus() {
				case domain.Waiting:
					if time.Now().After(job.NextRun()) {
						p.Execute(job, sem, wg)
					}
				case domain.Running:
				case domain.Paused:
				case domain.Stopped:
				case domain.Completed:
				case domain.Ended:

				}
				return true
			})
		}
	}

}
