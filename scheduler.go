package go_scheduler

import (
	"context"
	"go-scheduler/internal/domain"
	"go-scheduler/internal/job"
	defaultMonitoring "go-scheduler/internal/monitoring"
	"go-scheduler/internal/pool"
)

type PoolConfig struct {
	*domain.Pool
}

type Pool struct {
	*pool.Pool
}

type Monitoring interface {
	domain.Monitoring
}

type Scheduler struct {
	ctx context.Context
}

func New(ctx context.Context) *Scheduler {
	return &Scheduler{ctx: ctx}
}

func (s *Scheduler) CreatePool(cfg PoolConfig, mon Monitoring) *Pool {
	p := &Pool{
		Pool: &pool.Pool{},
	}
	if mon == nil {
		mon = defaultMonitoring.New()
	}
	p.Init(s.ctx, *cfg.Pool, mon)
	return p
}

func (s *Scheduler) AddJob(pool *Pool, cfg domain.JobDTO) error {
	j, err := job.New(cfg, pool.Ctx)
	if err != nil {
		return err
	}
	return pool.AddJob(j)
}
