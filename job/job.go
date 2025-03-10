package job

import (
	"context"
	"fmt"
	"go-scheduler/domain"
	errs "go-scheduler/error"
	"sync"
	"time"
)

// Job represents a scheduled task that can be executed by the scheduler.
type Job struct {
	domain.JobDTO

	// Ctx is the execution context of the job, allowing cancellation and timeout control.
	ctx context.Context

	// Cancel is the function used to cancel the job's execution.
	cancel context.CancelFunc

	// State contains the runtime information of the job, such as execution time and status.
	state *state

	// Mu is a mutex used to synchronize access to the job's state.
	mu sync.Mutex

	// pauseCh is a channel used to pause the execution of the job.
	// When a signal is received, the job should enter the Paused state.
	pauseCh chan struct{}

	// resumeCh is a channel used to resume execution of a paused job.
	// When a signal is received, the job should transition back to Running.
	resumeCh chan struct{}

	cron *CronSchedule
}

func New(jobDTO domain.JobDTO, ctx context.Context) (*Job, error) {
	job := &Job{
		JobDTO: jobDTO,
	}

	if job.ID == "" {
		return nil, errs.New(errs.ErrEmptyID, fmt.Sprintf("job name - %s", job.Name))
	}

	if job.Fn == nil {
		return nil, errs.New(errs.ErrEmptyFunction, job.ID)
	}
	if job.Name == "" {
		job.Name = job.ID
	}
	if job.Schedule.CronExpr != "" && job.Schedule.Interval > 0 {
		return nil, errs.New(errs.ErrMixedScheduleType, job.ID)
	}
	if job.Schedule.CronExpr != "" {
		var err error
		job.cron, err = ParseCron(job.Schedule.CronExpr)
		if err != nil {
			return nil, errs.New(errs.ErrInvalidCronExpression, fmt.Sprintf("error - %v, id: %s", err, job.ID))
		}
	}
	if job.StartAt.IsZero() {
		job.StartAt = time.Now()
	}
	if job.EndAt.IsZero() {
		job.EndAt = domain.MAX_END_AT // Predefined constant for the maximum job execution period
	}
	if job.EndAt.Before(job.StartAt) {
		return nil, errs.New(errs.ErrWrongTime, fmt.Sprintf("ending time cannot be before starting time, id: %s", job.ID))
	}
	job.state.Init(job.ID)
	job.ctx, job.cancel = context.WithCancel(ctx)
	job.pauseCh = make(chan struct{})
	job.resumeCh = make(chan struct{})

	return job, nil
}

func (j *Job) GetStatus() domain.JobStatus {
	return j.state.GetStatus()
}

func (j *Job) SetStatus(status domain.JobStatus) {
	j.state.SetStatus(status)
}

func (j *Job) TrySetStatus(allowed []domain.JobStatus, status domain.JobStatus) bool {
	return j.state.TrySetStatus(allowed, status)
}

func (j *Job) UpdateStateWithStrict(state domain.StateDTO) {
	j.state.Update(state, true)
}

func (j *Job) UpdateState(state domain.StateDTO) {
	j.state.Update(state, false)
}
