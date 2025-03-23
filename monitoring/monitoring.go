package monitoring

import (
	"go-scheduler/internal/domain"
	"sync"
)

// Monitoring provides an in-memory, thread-safe implementation of the domain.Monitoring interface.
//
// It stores and retrieves execution metrics for scheduled jobs using a concurrent-safe map (`sync.Map`).
// This basic implementation is suitable for internal debugging, testing, and simple runtime analytics.
// For production scenarios, it is advisable to extend or replace this implementation with more advanced solutions.
type Monitoring struct {
	data *sync.Map // Thread-safe storage keyed by JobID, storing job execution states.
}

// New creates and initializes a new Monitoring instance.
//
// Returns:
//   - Pointer to an initialized Monitoring instance ready for metric storage and retrieval.
func New() *Monitoring {
	return &Monitoring{
		data: &sync.Map{},
	}
}

// SaveMetrics stores execution metrics from the provided StateDTO into the monitoring storage.
//
// Metrics are indexed by the job's unique identifier (JobID), allowing efficient retrieval.
//
// Parameters:
//   - dto: domain.StateDTO containing execution details of the job.
func (m *Monitoring) SaveMetrics(dto domain.StateDTO) {
	m.data.Store(dto.JobID, dto)
}

// GetMetrics retrieves all stored job execution metrics.
//
// Returns:
//   - A map with JobID as keys and domain.StateDTO values representing
//     the captured metrics and state details of each job execution.
func (m *Monitoring) GetMetrics() map[string]interface{} {
	result := make(map[string]interface{})
	m.data.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(domain.StateDTO)
		return true
	})
	return result
}
