package job

import (
	"go-scheduler/internal/domain"
	"time"
)

func (j *Job) ExecFunc() {
	startTime := time.Now()
	j.ProcessJobStart(startTime)
	ctrl := &FnControl{
		Ctx:        j.ctx,
		PauseChan:  j.pauseCh,
		ResumeChan: j.resumeCh,
		data:       &j.state.Data,
	}
	// Execute the job function and track execution duration.
	if j.Hooks.OnStart != nil {
		if err := j.Hooks.OnStart(ctrl); err != nil {
			j.UpdateState(domain.StateDTO{
				Error:  err,
				Status: domain.Error,
			})
		}
	}
	err := j.Fn(ctrl)
	var executionTime int64
	if err != nil {
		if j.Hooks.OnError != nil {
			j.Hooks.OnError(ctrl, err)
		}
		executionTime = time.Since(startTime).Nanoseconds()
		j.UpdateState(domain.StateDTO{
			Error:         err,
			Status:        domain.Error,
			ExecutionTime: executionTime,
		})
	} else {
		if j.Hooks.OnSuccess != nil {
			err := j.Hooks.OnSuccess(ctrl)
			executionTime = time.Since(startTime).Nanoseconds()
			if err != nil {
				j.UpdateState(domain.StateDTO{
					Error:         err,
					Status:        domain.Error,
					ExecutionTime: executionTime,
				})
			} else {
				j.state.SetSuccessState(j.Retry.ResetOnSuccess, executionTime)
			}
		} else {
			executionTime = time.Since(startTime).Nanoseconds()
			j.state.SetSuccessState(j.Retry.ResetOnSuccess, executionTime)
		}
	}
}
