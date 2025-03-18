package pool

import (
	"go-scheduler/internal/domain"
	"sync"
	"time"
)

func (p *Pool) processWaiting(job Job, sem chan struct{}, wg *sync.WaitGroup) {
	if time.Now().After(job.NextRun()) {
		p.Execute(job, sem, wg)
	}
}

func (p *Pool) processRunning(job Job) {
	err := job.ProcessRun()
	if err != nil {
		job.ProcessEnd(domain.Error, err)
	}
}

func (p *Pool) processCompleted(job Job) {
	if job.NextRun().After(time.Now()) {
		job.UpdateState(domain.StateDTO{
			Status: domain.Waiting,
		})
		return
	}
	job.UpdateState(domain.StateDTO{
		Status: domain.Ended,
	})
}

func (p *Pool) processError(job Job) {
	err := job.Retry()
	if err != nil {
		return
	}
	job.UpdateState(domain.StateDTO{
		Status: domain.Waiting,
	})
}
