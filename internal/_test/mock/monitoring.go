package mock

import (
	"go-scheduler/internal/domain"
)

type Monitoring struct {
	Metrics map[string]domain.StateDTO
}

func NewMonitoring() *Monitoring {
	return &Monitoring{
		Metrics: make(map[string]domain.StateDTO),
	}
}

func (m *Monitoring) SaveMetrics(state domain.StateDTO) {
	m.Metrics[state.JobID] = state
}

func (m *Monitoring) GetMetrics() map[string]domain.StateDTO {
	return m.Metrics
}
