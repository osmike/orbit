package pool

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
	"go-scheduler/internal/job"
	"go-scheduler/monitoring"
	"strconv"
	"sync"
	"testing"
	"time"
)

func CheckMetrics_Running(t *testing.T, data map[string]interface{}) {
	for k, v := range data {
		state, _ := v.(domain.StateDTO)
		_, err := strconv.Atoi(k)
		assert.NoError(t, err, "convert id to int. job id - %s, error - %v", k, err)
		assert.True(t, time.Duration(state.ExecutionTime) > 500*time.Millisecond, "exec time must be more than 500ms. job id - %s, executing time - %d", k, state.ExecutionTime)
		assert.Equal(t, domain.Running, state.Status, "job status must be Running. job id - %s, status - %s", k, state.Status)
	}
}

func CheckMetrics_Completed(t *testing.T, data map[string]interface{}) {
	for k, v := range data {
		state, _ := v.(domain.StateDTO)
		k_num, err := strconv.Atoi(k)
		assert.NoError(t, err, "convert id to int. job id - %s, error - %v", k, err)
		doubledVal, ok := state.Data["doubled"]
		assert.True(t, ok, "must be key doubled in state's data. job id - %s, data - %v", k, state.Data)
		v_num, ok := doubledVal.(int)
		assert.True(t, ok, "convert value of data[doubled] to int. job id - %s, value of data[doubled] - %d", k, doubledVal)
		assert.True(t, (v_num/2) == k_num, "must be equal: id and value of data[doubled], job id - %s, value of data[doubled] - %d", k, doubledVal)
		assert.True(t, time.Duration(state.ExecutionTime) > 1*time.Second, "exec time must be more than 1s. job id - %s, executing time - %S", k, state.ExecutionTime)
		assert.Equal(t, domain.Completed, state.Status, "job status must be Completed. job id - %s, status - %s", k, state.Status)
	}
}

func CheckMetrics_Stopped(t *testing.T, data *sync.Map) {
	data.Range(func(k, v interface{}) bool {
		var j *job.Job
		j, _ = v.(*job.Job)
		state := j.GetState()
		assert.Equal(t, domain.Stopped, state.Status, "job status must be Stopped. job id - %s, status - %s", k, state.Status)
		assert.ErrorIs(t, state.Error.JobError, errs.ErrPoolShutdown, "state error must be \"pool shutdown\". job id - %s, err - %s", k, state.Error.JobError)
		assert.Equal(t, 2, state.Success, "successfully executed jobs must be 2. job id - %s, success count - %s", k, state.Success)
		return true
	})
}

func CheckIfMetrics_ContainsCorrectData(data map[string]interface{}) bool {
	for k, v := range data {
		state, _ := v.(domain.StateDTO)
		k_num, err := strconv.Atoi(k)
		if err != nil {
			return false
		}
		doubledVal, ok := state.Data["doubled"]
		if !ok {
			return false
		}
		v_num, ok := doubledVal.(int)
		if !ok {
			return false
		}
		if (v_num / 2) != k_num {
			return false
		}
		if time.Duration(state.ExecutionTime) < 1*time.Second {
			return false
		}
	}
	return true
}

func PrintJobsInPool(p *Pool) {
	p.jobs.Range(func(k, v interface{}) bool {
		var key string
		var j *job.Job
		key, _ = k.(string)
		j, _ = v.(*job.Job)
		state := j.GetState()
		fmt.Printf("key - %s\n", key)
		fmt.Printf("job id - %s\n", j.ID)
		fmt.Printf("job start at - %s\n", j.StartAt)
		fmt.Printf("job end at - %s\n", j.EndAt)
		fmt.Printf("job timeout - %s\n", j.Timeout)
		fmt.Printf("job state id - %s\n", state.JobID)
		fmt.Printf("job state start at - %v\n", state.StartAt)
		fmt.Printf("job state end at - %v\n", state.EndAt)
		fmt.Printf("job state status - %s\n", state.Status)
		fmt.Printf("job state exec time - %v\n", state.ExecutionTime)
		fmt.Printf("job state data - %v\n", state.Data)
		fmt.Printf("job error - %v\n", state.Error.JobError)
		fmt.Printf("job success count - %v\n", state.Success)
		return true
	})
}

func PrintJobsInMonitoring(mon domain.Monitoring) {
	monData := mon.GetMetrics()
	for k, v := range monData {
		state, _ := v.(domain.StateDTO)
		fmt.Printf("key - %s\n", k)
		fmt.Printf("job state start at - %v\n", state.StartAt)
		fmt.Printf("job state end at - %v\n", state.EndAt)
		fmt.Printf("job state status - %s\n", state.Status)
		fmt.Printf("job state exec time - %v\n", state.ExecutionTime)
		fmt.Printf("job next run - %v\n", state.NextRun)
		fmt.Printf("job state data - %v\n", state.Data)
		fmt.Printf("job error - %v\n", state.Error.JobError)
		fmt.Printf("job success count - %v\n", state.Success)
	}
}

func TestPool_Run(t *testing.T) {
	cfg := domain.Pool{
		MaxWorkers:    1001,
		CheckInterval: 20 * time.Millisecond,
		IdleTimeout:   5 * time.Second,
	}
	mon := monitoring.New()
	var err error

	p, _ := New(context.Background(), cfg, mon)
	for i := 1; i < 1000; i++ {
		fn := func(ctrl domain.FnControl) error {
			for {
				select {
				case <-ctrl.PauseChan():
					<-ctrl.ResumeChan()
				case <-ctrl.Context().Done():
					fmt.Println("job here")
					return ctrl.Context().Err()
				default:
					time.Sleep(1 * time.Second)
					data := map[string]interface{}{
						"doubled": i * 2,
					}
					ctrl.SaveData(data)
					return nil
				}
			}
		}
		jCfg := domain.JobDTO{
			ID:       strconv.Itoa(i),
			Interval: domain.Interval{Time: 2 * time.Second},
			Fn:       fn,
		}
		var j *job.Job
		j, err = job.New(jCfg, p.Ctx, p.Mon)
		assert.NoError(t, err)
		err = p.AddJob(j)
		assert.NoError(t, err)
	}
	p.Run()
	time.Sleep(550 * time.Millisecond)
	CheckMetrics_Running(t, mon.GetMetrics())
	time.Sleep(530 * time.Millisecond)
	CheckMetrics_Completed(t, mon.GetMetrics())
	time.Sleep(2000 * time.Millisecond)

	p.Kill()
	time.Sleep(100 * time.Millisecond)
	err = p.Run()
	assert.ErrorIs(t, err, errs.ErrPoolShutdown)
}
