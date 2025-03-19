package scheduler

import (
	"sync"
)

type DefaultMonitor struct {
	Data sync.Map
}

func NewDefaultMonitor() *DefaultMonitor {
	return &DefaultMonitor{}
}

func (m *DefaultMonitor) AddJob(job *JobMetadata) {
	m.Data.Store(job.ID, job)
}

func (m *DefaultMonitor) GetJobByID(id string) *JobMetadata {
	if job, ok := m.Data.Load(id); ok {
		return job.(*JobMetadata)
	}
	return nil
}

func (m *DefaultMonitor) GetJobs() []*JobMetadata {
	var jobs []*JobMetadata
	m.Data.Range(func(_, value interface{}) bool {
		jobs = append(jobs, value.(*JobMetadata))
		return true
	})
	return jobs
}

func (m *DefaultMonitor) UpdateState(id string, state *State) {
	if job, ok := m.Data.Load(id); ok {
		state.Data.Range(func(key, value interface{}) bool {
			job.(*JobMetadata).State.Data.Store(key, value)
			return true
		})
		job.(*JobMetadata).State.EndAt = state.EndAt
		job.(*JobMetadata).State.Error = state.Error
		job.(*JobMetadata).State.ExecutionTime = state.ExecutionTime
		job.(*JobMetadata).State.Status.Store(state.Status.Load())
	}
}
