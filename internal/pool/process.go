package pool

import (
	"sync"
	"time"
)

func (p *Pool) processWaiting(job Job, sem chan struct{}, wg *sync.WaitGroup) {
	if time.Now().After(job.NextRun()) {
		p.Execute(job, sem, wg)
	}
}

func (p *Pool) processRunning(job Job) {

}
