package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type JobStatus string

const (
	Waiting   JobStatus = "waiting"
	Running   JobStatus = "running"
	Stopped   JobStatus = "stopped"
	Paused    JobStatus = "paused"
	Completed JobStatus = "completed"
	Error     JobStatus = "error"
)

type Fn func(ctrl FnControl) error

type FnControl struct {
	Ctx        context.Context
	PauseChan  chan struct{}
	ResumeChan chan struct{}
	data       *sync.Map
}

func (ctrl *FnControl) SaveUserInfo(data map[string]string) {
	for key, val := range data {
		ctrl.data.Store(key, val)
	}
}

type Job struct {
	ID       string
	Name     string
	Fn       Fn
	Interval time.Duration
	Timeout  time.Duration
	StartAt  time.Time
	EndAt    time.Time
	Retry    Retry
	State    State
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	pauseCh  chan struct{}
	resumeCh chan struct{}
}

type Retry struct {
	Active         bool
	Count          int64
	ResetOnSuccess bool
}

type State struct {
	JobID         string
	StartAt       time.Time
	EndAt         time.Time
	Error         error
	currentRetry  int64
	Status        atomic.Value
	ExecutionTime int64
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

func (j *Job) exec(wg *sync.WaitGroup, sem chan struct{}) {
	wg.Add(1)
	defer wg.Done()
	if !j.canExec() {
		return
	}
	go func() {
		sem <- struct{}{}
		defer func() { <-sem }()
		startTime := time.Now()

		defer func() {
			if r := recover(); r != nil {
				j.setStatus(Error)
				j.State.Error = fmt.Errorf("Job panicked: %v\n", r)
				j.State.ExecutionTime = time.Since(startTime).Nanoseconds()
			}
		}()
		ctrl := FnControl{
			Ctx:        j.ctx,
			PauseChan:  j.pauseCh,
			ResumeChan: j.resumeCh,
			data:       &j.State.Data,
		}
		j.State.StartAt = startTime
		err := j.Fn(ctrl)
		j.State.ExecutionTime = time.Since(startTime).Nanoseconds()

		if err != nil {
			j.setStatus(Error)
			j.State.Error = err
		} else {
			j.setStatus(Completed)
			if j.State.Error != nil {
				if j.Retry.ResetOnSuccess {
					j.State.currentRetry = j.Retry.Count
				}
				j.State.Error = nil
			}
		}
	}()
}

func (j *Job) processRunning() {
	j.State.ExecutionTime = time.Since(j.State.StartAt).Nanoseconds()
	j.State.Error = nil
	if time.Duration(j.State.ExecutionTime) > j.Timeout {
		j.cancel()
		j.setStatus(Error)
		j.State.Error = fmt.Errorf("job timed out after %v", j.Timeout)
		return
	}
}

func (j *Job) processCompleted() {
	j.State.EndAt = time.Now()
	j.tryChangeStatus([]JobStatus{Completed}, Waiting)
}

func (j *Job) processError() {
	j.State.EndAt = time.Now()
	if j.Retry.Active && j.State.currentRetry > 0 {
		j.setStatus(Waiting)
		j.State.currentRetry--
	}
}

func (j *Job) canExec() bool {
	if j.Interval == 0 && !j.State.EndAt.IsZero() {
		j.setStatus(Stopped)
		return false
	}
	if !j.tryChangeStatus([]JobStatus{Waiting}, Running) {
		return false
	}
	if time.Now().Before(j.StartAt) {
		return false
	}
	if time.Now().After(j.EndAt) {
		j.setStatus(Stopped)
		return false
	}
	delay := j.getDelay()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-j.ctx.Done():
			return false
		}
	}
	return true
}

func (j *Job) getDelay() time.Duration {
	delay := j.Interval - time.Duration(j.State.ExecutionTime)
	if delay < 0 {
		delay = 0
	}
	return delay
}

func (j *Job) processStopped() {}
