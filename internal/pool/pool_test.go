package pool

import (
	"context"
	"github.com/stretchr/testify/assert"
	"orbit/internal/domain"
	errs "orbit/internal/error"
	"orbit/internal/job"
	"orbit/monitoring"
	"orbit/test"
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
		state, ok := v.(domain.StateDTO)
		assert.True(t, ok, "expected domain.StateDTO for job id %s", k)

		kNum, err := strconv.Atoi(k)
		assert.NoError(t, err, "failed to convert job id to int: job id = %s", k)

		doubledVal, exists := state.Data["doubled"]
		assert.True(t, exists, "missing 'doubled' key in job data: job id = %s, data = %+v", k, state.Data)

		vNum, ok := doubledVal.(int)
		assert.True(t, ok, "expected 'doubled' value to be int: job id = %s, value = %+v", k, doubledVal)

		assert.Equal(t, kNum*2, vNum, "unexpected 'doubled' value: job id = %s, expected = %d, got = %d", k, kNum*2, vNum)

		assert.Greater(t, time.Duration(state.ExecutionTime), 1*time.Second, "execution time must be > 1s: job id = %s, execTime = %d", k, state.ExecutionTime)

		assert.Contains(t, []domain.JobStatus{domain.Completed, domain.Waiting}, state.Status, "unexpected job status: job id = %s, status = %s", k, state.Status)
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

	p := New(context.Background(), cfg, mon)
	for i := 1; i < 1001; i++ {
		fn := func(ctrl domain.FnControl) error {
			for {
				select {
				case <-ctrl.PauseChan():
					<-ctrl.ResumeChan()
				case <-ctrl.Context().Done():
					return ctrl.Context().Err()
				default:
					time.Sleep(2 * time.Second)
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
			Interval: domain.Interval{Time: 3 * time.Second},
			Fn:       fn,
		}
		var j *job.Job
		j, err = job.New(jCfg, p.Ctx, p.Mon)
		assert.NoError(t, err)
		err = p.AddJob(j)
		assert.NoError(t, err)
	}
	err = p.Run()
	assert.NoError(t, err)
	test.WaitForCondition(t, 1*time.Second, func() bool {
		metrics := mon.GetMetrics()

		count := 0
		for _, m := range metrics {
			state, ok := m.(domain.StateDTO)
			if !ok {
				continue
			}
			if state.Status == domain.Running {
				count++
			}
		}
		return count >= 999
	})
	time.Sleep(500 * time.Millisecond)
	CheckMetrics_Running(t, mon.GetMetrics())
	test.WaitForCondition(t, 3*time.Second, func() bool {
		metrics := mon.GetMetrics()
		count := 0
		for _, m := range metrics {
			state, ok := m.(domain.StateDTO)
			if !ok {
				continue
			}
			if state.Status == domain.Completed || state.Status == domain.Waiting {
				count++
			}
		}
		return count >= 999
	})
	CheckMetrics_Completed(t, mon.GetMetrics())

	p.Kill()
	time.Sleep(100 * time.Millisecond)
	err = p.Run()
	assert.ErrorIs(t, err, errs.ErrPoolShutdown)
}
