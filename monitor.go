package scheduler

import "time"

type Monitor interface {
	SaveMetrics(data JobMetrics) error
	GetMetrics() JobMetrics
}

type MonitorSettings struct {
	CustomMetrics    map[string]string
	MonitorTime      bool
	MonitorError     bool
	MonitorJobStatus bool
}

type MonitorConfig struct {
	Settings MonitorSettings
	Monitor  Monitor
}

type JobMetrics struct {
	ExecutionTime time.Duration
	ErrorCount    int
	Status        string
}

type MonitorClient struct {
	Cfg MonitorConfig
}

func NewMonitorClient(cfg MonitorConfig) *MonitorClient {
	return &MonitorClient{Cfg: cfg}
}

func (m *MonitorClient) start(MonitorSettings) error {
	return nil
}
