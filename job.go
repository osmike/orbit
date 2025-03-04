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
	ID       string
	Name     string
	Fn       func() error
	Interval time.Duration
	StartAt  time.Time
	EndAt    time.Time
	Retry    Retry
	State    State
	ctx      context.Context
	cancel   context.CancelFunc
	mutex    sync.Mutex
}

type Retry struct {
	Active   bool
	RetryCnt int64
}

type State struct {
	Status        atomic.Value
	ExecutionTime time.Duration
	Data          map[string]string
}

func (j *Job) setStatus(status JobStatus) {
	j.State.Status.Store(status)
}

func (j *Job) getStatus() JobStatus {
	val := j.State.Status.Load()
	if val == nil {
		j.State.Status.Store(Waiting)
		return Waiting
	}
	return val.(JobStatus)
}
