package mock

import (
	"go-scheduler/internal/domain"
	"time"
)

type Job struct {
	ID     string
	Status domain.JobStatus
}

func NewJob(id string, status domain.JobStatus) *Job {
	return &Job{ID: id, Status: status}
}

func (j *Job) GetMetadata() domain.JobDTO                                    { return domain.JobDTO{ID: j.ID} }
func (j *Job) GetStatus() domain.JobStatus                                   { return j.Status }
func (j *Job) UpdateState(state domain.StateDTO)                             {}
func (j *Job) NextRun() time.Time                                            { return time.Now() }
func (j *Job) ProcessStart(mon domain.Monitoring)                            {}
func (j *Job) ProcessRun(mon domain.Monitoring) error                        { return nil }
func (j *Job) ProcessEnd(s domain.JobStatus, e error, mon domain.Monitoring) {}
func (j *Job) CanExecute() error                                             { return nil }
func (j *Job) Retry() error                                                  { return nil }
func (j *Job) Execute() error                                                { return nil }
func (j *Job) Stop()                                                         {}
func (j *Job) Pause(timeout time.Duration) error                             { return nil }
func (j *Job) Resume() error                                                 { return nil }
