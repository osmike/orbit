package monitoring

import "sync"

type Monitoring struct {
	data *sync.Map
}

func New() *Monitoring {
	return &Monitoring{
		data: &sync.Map{},
	}
}

func (m *Monitoring) SaveMetrics(data map[string]interface{}) {
	for k, v := range data {
		m.data.Store(k, v)
	}
}

func (m *Monitoring) GetMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})
	m.data.Range(func(key, value interface{}) bool {
		metrics[key.(string)] = value
		return true
	})
	return metrics
}
