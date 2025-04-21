package pool

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"orbit/internal/domain"
	errs "orbit/internal/error"
	"orbit/internal/job"
	"orbit/monitoring"
	"strconv"
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
