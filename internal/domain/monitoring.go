package domain

type Monitoring interface {
	SaveMetrics(data map[string]interface{})
	GetMetrics() map[string]interface{}
}
