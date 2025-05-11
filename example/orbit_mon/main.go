// Example: Orbit integration with Prometheus and Grafana
// Exposes custom metrics on /metrics endpoint via promhttp
// Run this together with Prometheus + Grafana for live job monitoring

package main

import (
	"context"
	"fmt"
	"github.com/osmike/orbit"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// PrometheusMonitoring implements orbit.Monitoring and exposes metrics via Prometheus
type PrometheusMonitoring struct {
	successCounter *prometheus.CounterVec
	failureCounter *prometheus.CounterVec
	duration       *prometheus.HistogramVec
	statusGauge    *prometheus.GaugeVec
}

func NewPrometheusMonitoring() *PrometheusMonitoring {
	m := &PrometheusMonitoring{
		successCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "job_success_total",
				Help: "Total number of successful executions",
			},
			[]string{"job_id"},
		),
		failureCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "job_failure_total",
				Help: "Total number of failed executions",
			},
			[]string{"job_id"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "job_duration_seconds",
				Help:    "Execution duration per job",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"job_id"},
		),
		statusGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "job_status",
				Help: "Current job status (0=Waiting, 1=Running, 2=Ended, 3=Error, 4=Paused, 5=Stopped)",
			},
			[]string{"job_id"},
		),
	}

	prometheus.MustRegister(m.successCounter, m.failureCounter, m.duration, m.statusGauge)
	return m
}

func (m *PrometheusMonitoring) SaveMetrics(state orbit.JobState) {
	id := state.JobID
	if state.Status == "completed" || state.Status == "ended" {
		m.successCounter.WithLabelValues(id).Inc()
		m.duration.WithLabelValues(id).Observe(float64(state.ExecutionTime) / 1e9)
	} else if state.Status == "error" {
		m.failureCounter.WithLabelValues(id).Inc()
	}
	statusMap := map[string]float64{
		"waiting":   0,
		"running":   1,
		"ended":     2,
		"error":     3,
		"paused":    4,
		"stopped":   5,
		"completed": 2,
	}
	m.statusGauge.WithLabelValues(id).Set(statusMap[string(state.Status)])
}

func (m *PrometheusMonitoring) GetMetrics() map[string]interface{} {
	return map[string]interface{}{}
}

func main() {
	mon := NewPrometheusMonitoring()
	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		CheckInterval: 500 * time.Millisecond,
	}, mon)
	if err != nil {
		log.Fatal(err)
	}

	job := orbit.Job{
		ID: "grafana_demo_job",
		Fn: func(ctrl orbit.FnControl) error {
			delay := time.Duration(rand.Intn(500)+500) * time.Millisecond
			time.Sleep(delay)
			if rand.Float32() < 0.3 {
				return fmt.Errorf("simulated error")
			}
			return nil
		},
		Interval: orbit.Interval{
			Time: 2 * time.Second,
		},
	}

	if err := pool.AddJob(job); err != nil {
		log.Fatal(err)
	}

	pool.Run()
	log.Println("[Orbit] Scheduler running...")

	http.Handle("/metrics", promhttp.Handler())
	log.Println("[Metrics] Exposed on http://localhost:2112/metrics")
	log.Fatal(http.ListenAndServe(":2112", nil))
}
