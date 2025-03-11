package pool

import (
	"go-scheduler/internal/domain"
	"time"
)

type Job interface {
	GetStatus() domain.JobStatus
	SetStatus(status domain.JobStatus)
	TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool
	UpdateStateWithStrict(state domain.StateDTO)
	UpdateState(state domain.StateDTO)
	GetDelay() time.Duration
}

func (p *Pool) Execute(job *)
