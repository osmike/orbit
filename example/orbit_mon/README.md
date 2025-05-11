# Grafana Monitoring Example for Orbit

This example demonstrates how to integrate the Orbit scheduler with Prometheus and Grafana for real-time job monitoring and alerting.

## 📦 Features

- Tracks job success/failure counts
- Measures execution duration in seconds
- Exposes job status as numeric gauge
- Fully compatible with Prometheus + Grafana stack
---
## 📁 Files
- main.go – Runs an Orbit job that emits Prometheus metrics

## 🔧 Setup Instructions

### 1. Run the example
```
go run main.go
```

It will:
- Start a recurring job that randomly succeeds or fails
- Expose metrics on http://localhost:2112/metrics

### 2. Prometheus Configuration

Create a file named `prometheus.yml`:
```yml
global:
  scrape_interval: 5s

scrape_configs:
  - job_name: 'orbit_example'
    static_configs:
      - targets: ['localhost:2112']
```


Then run Prometheus:

```
prometheus --config.file=prometheus.yml
```

### 3. Grafana Dashboard (Suggested Panels)

Import a new dashboard and add these panels:

#### ✅ Job Success Count
```
sum(increase(job_success_total[5m])) by (job_id)
```
#### ❌ Job Failure Count
```
sum(increase(job_failure_total[5m])) by (job_id)
```
#### ⏱️ Execution Duration (Histogram)
```
histogram_quantile(0.95, sum(rate(job_duration_seconds_bucket[5m])) by (le, job_id))
```
#### 📊 Job Status
```
job_status{job_id="grafana_demo_job"}
```
Display as: Stat panel with value mappings:
```
0 = Waiting
1 = Running
2 = Completed
3 = Error
4 = Paused
5 = Stopped
```
---
## 📈 Example Output

After running a few minutes, your dashboard will show:
- Success/failure spikes
- Duration distribution
- Live job state indicator
---
## 💡 Tips
- You can add alert rules in Prometheus for job error spikes
- You can expose additional metrics by extending PrometheusMonitoring
- All metric labels are per job_id to support multi-job environments
---
Made with ❤️ and Orbit

