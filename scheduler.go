package go_scheduler

import (
	"context"
	"go-scheduler/internal/domain"
	"go-scheduler/internal/job"
	"go-scheduler/internal/pool"
)

type PoolConfig domain.Pool

type Pool struct {
	*pool.Pool
}

type Scheduler struct {
	ctx context.Context
}

func New(ctx context.Context) *Scheduler {
	return &Scheduler{ctx: ctx}
}

func (s *Scheduler) CreatePool(cfg PoolConfig) Pool {
	p := &Pool{
		Pool: &pool.Pool{},
	}
	p.Init(s.ctx, domain.Pool(cfg))
	return *p
}

func (s *Scheduler) AddJob(pool *pool.Pool, cfg domain.JobDTO) error {
	j, err := job.New(cfg, pool.GetContext())
	if err != nil {
		return err
	}
	return pool.AddJob(j)
}
