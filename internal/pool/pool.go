package pool

import (
	"context"
	"go-scheduler/internal/domain"
	"time"
)

type Job interface {
	GetMetadata() domain.JobDTO
	GetStatus() domain.JobStatus
	SetStatus(status domain.JobStatus)
	TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool
	UpdateStateWithStrict(state domain.StateDTO)
	UpdateState(state domain.StateDTO)
	GetDelay() time.Duration
	NextRun() time.Time
	CanExecute() error
	ExecFunc()
}

type Pool struct {
	ctx    context.Context
	cancel context.CancelFunc
}
