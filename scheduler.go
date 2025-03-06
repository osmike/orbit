package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Scheduler struct {
	cfg    Config
	log    *slog.Logger
	jobs   sync.Map
	ctx    context.Context
	cancel context.CancelFunc
}

type Config struct {
	MaxWorkers  int
	IdleTimeout time.Duration
}

func New(cfg Config, log *slog.Logger, ctx context.Context) *Scheduler {
	ctx, cancel := context.WithCancel(ctx)
	if cfg.MaxWorkers < 1 {
		cfg.MaxWorkers = 100_000
	}
	s := &Scheduler{
		cfg:    cfg,
		log:    log,
		jobs:   sync.Map{},
		ctx:    ctx,
		cancel: cancel,
	}
	go s.runScheduler()
	return s
}
func (s *Scheduler) Add(jobs ...*Job) error {
	for _, job := range jobs {

		if err := s.sanitizeJob(job); err != nil {
			return fmt.Errorf("error adding job - %v, %w, err: %v", job, ErrAddingJob, err)
		}
		s.jobs.Store(job.ID, job)
	}
	return nil
}
