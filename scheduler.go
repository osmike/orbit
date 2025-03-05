package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Scheduler struct {
	cfg     Config
	log     *slog.Logger
	Jobs    []*Job
	jobChan chan *Job
	ctx     context.Context
	cancel  context.CancelFunc
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
		cfg:     cfg,
		log:     log,
		Jobs:    make([]*Job, 0),
		jobChan: make(chan *Job, 100),
		ctx:     ctx,
		cancel:  cancel,
	}
	go s.runScheduler()
	return s
}
func (s *Scheduler) Add(jobs ...*Job) error {
	for _, job := range jobs {

		if err := s.sanitizeJob(job); err != nil {
			return fmt.Errorf("error adding job - %v, %w, err: %v", job, ErrAddingJob, err)
		}
		s.Jobs = append(s.Jobs, job)
		select {
		case s.jobChan <- job:
			s.log.Info("Added job", "id", job.ID)
		default:
			s.log.Warn("Job queue is full, skipping job", "id", job.ID)
			continue
		}
	}
	return nil
}
