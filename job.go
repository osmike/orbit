package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type JobStatus string

const (
	Waiting   JobStatus = "waiting"
	Running   JobStatus = "running"
	Stopped   JobStatus = "stopped"
	Completed JobStatus = "completed"
	Error     JobStatus = "error"
)

type Job struct {
	ID        string
	Name      string
	Fn        func() error
	Interval  time.Duration
	StartAt   time.Time
	EndAt     time.Time
	Retry     Retry
	State     State
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	pauseChan chan struct{}
	execChan  chan struct{}
}

type Retry struct {
	Active   bool
	RetryCnt int64
}

type State struct {
	Status        atomic.Value
	ExecutionTime atomic.Int64
	Data          sync.Map
}

func (j *Job) setStatus(status JobStatus) {
	j.State.Status.Store(status)
}

func (j *Job) getStatus() JobStatus {
	val := j.State.Status.Load()
	if val == nil {
		return Waiting
	}
	return val.(JobStatus)

}

func (j *Job) tryChangeStatus(allowed []JobStatus, newStatus JobStatus) bool {
	current := j.getStatus()
	for _, status := range allowed {
		if current == status {
			return j.State.Status.CompareAndSwap(current, newStatus)
		}
	}
	return false
}
