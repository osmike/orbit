package job

import "sync"

func (j *Job) execute(wg *sync.WaitGroup, sem chan struct{}) {
	defer wg.Done()
	wg.Add(1)

}
