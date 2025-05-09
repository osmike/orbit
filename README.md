

# 🚀 Orbit Scheduler – Task Scheduling Made Easy!

[![Go](https://img.shields.io/badge/Made%20with-Go-blue)](https://golang.org)
[![Coverage](https://img.shields.io/badge/Coverage-87%25-brightgreen)](https://github.com/osmike/orbit)
![MIT License](https://img.shields.io/badge/license-MIT-green.svg)

Orbit is a powerful yet intuitive job scheduler written entirely in Go. Effortlessly schedule, run, monitor, and manage your tasks, ensuring reliability and efficiency.

Orbit is a **lightweight** and **dependency-free** Go library — no external modules required!

---

## ✨ Why Orbit?

- **🔧 Simple API**: Quickly set up scheduled jobs using clean and intuitive methods.
- **⚡ High Performance**: Leverages Go's concurrency model to run thousands of tasks effortlessly.
- **📈 Built-In Monitoring**: Track job execution in real-time with built-in monitoring hooks.
- **🎯 Flexible Scheduling**: Supports both interval-based and cron-based schedules.
- **🧠 Intelligent Control**: Pause, resume, and stop jobs on the fly — interactively control any task like media playback.
- **🔒 Safe & Reliable**: Panic recovery and error isolation ensure your scheduler never crashes.


---

## 📦 Installation

Simply install with:

```bash
go get github.com/osmike/orbit
```

---

## 🚦 Quick Start

Here's how easy it is to get started:

```go
package main

import (
  "context"
  "fmt"
  "github.com/osmike/orbit"
  "time"
)

func main() {
  ctx := context.Background()

  pool, _ := orbit.CreatePool(ctx, orbit.PoolConfig{
    // MaxWorkers sets the maximum number of concurrent workers allowed to execute jobs simultaneously.
    // Higher values can improve throughput for CPU-bound or I/O-bound tasks, but might consume more system resources.
    // Default value:
    MaxWorkers: 1000,
    // IdleTimeout specifies the duration after which a job that remains idle
    // (not executed or scheduled for immediate execution) will be marked as inactive.
    // This helps optimize resource usage and prevents accumulation of stale tasks.
    // Default value:
    IdleTimeout: 100 * time.Hour,
    // CheckInterval defines how frequently the pool checks for jobs that are ready for execution or require status updates.
    // Short intervals result in more responsive job execution at the expense of slightly increased CPU utilization.
    CheckInterval: 100 * time.Millisecond,
  }, nil)

  jobCfg := orbit.JobConfig{
    ID:   "hello-world",
    Name: "Print Hello World",
    Fn: func(ctrl orbit.FnControl) error {
      fmt.Println("Hello, World!")
      return nil
    },
    Interval: orbit.IntervalConfig{Time: 5 * time.Second},
  }

  pool.AddJob(jobCfg)
  // Run starts the main controlling goroutine for the pool.
  // It continuously manages job scheduling, execution, and lifecycle events.
  pool.Run()

  select {} // Keep running indefinitely
}
```

---

## 🛠 Features

### 🎮 **Live Control**: Pause, Resume, or Stop jobs dynamically — as easily as managing a video or audio track.
#### **📅 Advanced Job Example**: Cron-Based Execution with State & Control
This example demonstrates how to:

- initialize job state on start (`OnStart`)
- perform incremental batch processing
- support pause/resume interaction
- run on a cron schedule

```go
// onStartFn runs before the main job function.
// It initializes the state with the total number of rows to process.
jobID := "weekly-upload"

onStartFn := func(ctrl orb.FnControl) error {
    rowCnt := db.GetRowCount() // Get total rows to process from DB

    // Save the job's initial state
    ctrl.SaveData(map[string]interface{}{
        "rowCnt":  rowCnt,
        "uploaded": 0, // progress tracker
    })

    fmt.Printf("Job %s started, row count: %d\n", jobID, rowCnt)
    return nil
}


// mainFn handles the batch upload in chunks of 1000 rows.
// It supports pause/resume, stateful progress, and clean shutdown.
mainFn := func(ctrl orb.FnControl) error {
    for {
        select {
        case <-ctrl.PauseChan():
            fmt.Println("Paused... waiting for resume")

            <-ctrl.ResumeChan()
            fmt.Println("Resumed, reconnecting to DB...")
            reconnectToDB()

        case <-ctrl.Context().Done():
            return ctrl.Context().Err() // exit cleanly if context is cancelled

        default:
            // Read current data
            data := ctrl.GetData()

            uploaded := data["uploaded"].(int)
            total := data["rowCnt"].(int)

            // Simulate batch processing
            uploaded += db.BatchInsert(1000)

            // Persist updated data in job state
            ctrl.SaveData(map[string]interface{}{
                "rowCnt":  total,
                "uploaded": uploaded,
            })

            // Stop the job when all data is uploaded
            if uploaded >= total {
                fmt.Println("✅ All data uploaded successfully.")
                return nil
            }
        }
    }
}
  
```
***📌 This pattern is ideal for large ETL-like jobs, syncing data from external systems, or anything that may be paused and resumed on demand.***

Then configure the job like this:
```go
jobCfg := orbit.JobConfig{
    ID:       jobID,
    Name:     "Weekly DB Upload",
    Fn:       mainFn,
    OnStart:  onStartFn,
    Cron:     "0 20 * * 5", // Every Friday at 20:00 (8 PM)
}
```
And add it to a running pool:
```go
pool.AddJob(jobCfg)
```

#### **🧠 Key Concepts**
- **Persistent Job State**: Use `ctrl.SaveData()` and `ctrl.GetData()` to persist job's data between runs or across iterations inside a job.
- **Pause & Resume**: You can call `PauseJob(id string)` and `ResumeJob(id string)` at runtimefrom your application logic. The job can listen to `ctrl.PauseChan()` and `ctrl.ResumeChan()`to handle reconnections or resume where it left off.
- **Graceful Shutdown**: Jobs respect ctrl.Context().Done() to terminate cleanly.

#### 🕹 Runtime Control Example: Pause & Resume the Job
You can pause and resume a running job dynamically using the pool's control methods:
```go
// Pause the job with a timeout of 10 seconds.
// This timeout ensures two things:
//
// 1. If the job **never reads** from ctrl.PauseChan(), it will be auto-resumed after 10s.
// 2. It gives the job enough time to reach the point where it starts listening
//    for the pause signal (ctrl.PauseChan()) before it auto-resumes.
//
// This protects your system from accidental "stuck" states
// and provides a safe buffer for graceful transitions.
err := pool.PauseJob("weekly-upload", 10*time.Second)
if err != nil {
    log.Println("Pause failed:", err)
}

// Resume the job early (before the timeout ends)
err = pool.ResumeJob("weekly-upload")
if err != nil {
    log.Println("Resume failed:", err)
}
```
**🔎 Why the Timeout Matters?**

In Orbit, a job becomes paused only when it explicitly reads from ctrl.PauseChan().
But some jobs might:
- never reach that line
- reach it too late (e.g., stuck in DB logic or long loops)

The timeout solves both cases by:
- automatically resuming the job if it does not respond to pause
- allowing you time to coordinate other actions (like logging, alerts, or data flush) before pause is actually picked up

---

### **Concurrency Control**: Limit how many jobs run simultaneously.
### 🧪 Example: Concurrency Control in Action

You can control how many jobs run in parallel using the `MaxWorkers` option in your pool configuration.

#### 🔁 Scenario

Let's say you want only **one job at a time** to run. You add 3 jobs with the same interval, but with slight delays in when they're registered:

```go

pool, _ := orbit.CreatePool(context.Background(), context.Background(), orbit.PoolConfig{
    MaxWorkers:    1,                       // allow only 1 job at a time
    CheckInterval: 50 * time.Millisecond,  // quick job scanning
}, nil)

createJob := func(id string, delay time.Duration) orbit.JobConfig {
    return orbit.JobConfig{
        ID:       id,
        Name:     fmt.Sprintf("Job %s", id),
        Interval: orbit.IntervalConfig{Time: time.Second}, // runs every 1s
        Fn: func(ctrl orbit.FnControl) error {
            fmt.Printf("[%s] Started at %v\n", id, time.Now())
            time.Sleep(time.Second)
            fmt.Printf("[%s] Finished at %v\n", id, time.Now())
            return nil
        },
    }
}

pool.AddJob(createJob("job1", 0))
time.Sleep(100 * time.Millisecond)
pool.AddJob(createJob("job2", 100*time.Millisecond))
time.Sleep(100 * time.Millisecond)
pool.AddJob(createJob("job3", 200*time.Millisecond))

pool.Run()
select {}
```

#### 🔍 Output (approximate):

```
[job1] Started at 00:00:00
[job1] Finished at 00:00:01
[job2] Started at 00:00:01
[job2] Finished at 00:00:02
[job3] Started at 00:00:02
[job3] Finished at 00:00:03
```

#### 📌 Takeaway

Even though all jobs have the same interval (1s), they are executed **sequentially** because `MaxWorkers = 1`. The order of execution is determined by the **order in which jobs were added** to the pool.

---

### **Lifecycle Hooks**: Customize behavior with hooks (`OnStart`, `OnSuccess`, `OnError`, etc.).
Orbit gives you full control over the job lifecycle by providing hooks for key events. Hooks allow you to log, trace, or augment the behavior of jobs without touching the core logic.
<div align="center">

```mermaid
stateDiagram-v2

    state Running {
        [*] --> OnStart
        OnStart --> Executing : run Fn()
        Executing --> OnSuccess : no error
        Executing --> OnError : error occurred
        OnSuccess --> Finally
        OnError --> Finally
        Finally --> [*]
    }
```
</div>

#### Supported Hooks:
- `OnStart` – triggered when the job starts
- `OnPause` / `OnResume` – triggered on manual pause/resume
- `OnStop` – triggered on manual stop
- `OnSuccess` – after successful execution
- `OnError` – when an error occurs during job execution
- `Finally` – always executed at the end, regardless of result

### 🔍 Example: Logging Job Lifecycle Events
This example demonstrates a full lifecycle job that logs execution through each phase:
```go
var logger = log.New(os.Stdout, "", log.LstdFlags)

job := orbit.JobConfig{
    ID:   "hooked-job",
    Name: "Lifecycle Logging Demo",
    Interval: orbit.IntervalConfig{Time: 1 * time.Second},
    Fn: func(ctrl orbit.FnControl) error {
        logger.Println("[main] syncing data...")
        time.Sleep(300 * time.Millisecond)
        return errors.New("sync failed") // force error for demo
    },
    Hooks: orbit.HooksFunc{
        OnStart: orbit.Hook{
            Fn: func(ctrl orbit.FnControl, err error) error {
                logger.Println("[hook] OnStart: job starting...")
                return nil
            },
        },
        OnPause: orbit.Hook{
            Fn: func(ctrl orbit.FnControl, err error) error {
                logger.Println("[hook] OnPause: job paused")
                return nil
            },
        },
        OnResume: orbit.Hook{
            Fn: func(ctrl orbit.FnControl, err error) error {
                logger.Println("[hook] OnResume: job resumed")
                return nil
            },
        },
        OnStop: orbit.Hook{
            Fn: func(ctrl orbit.FnControl, err error) error {
                logger.Println("[hook] OnStop: job stopped")
                return nil
            },
        },
        OnSuccess: orbit.Hook{
            Fn: func(ctrl orbit.FnControl, err error) error {
                logger.Println("[hook] OnSuccess: job succeeded")
                return nil
            },
        },
        OnError: orbit.Hook{
            Fn: func(ctrl orbit.FnControl, err error) error {
                logger.Printf("[hook] OnError: job failed: %v\n", err)
                return nil
            },
        },
        Finally: orbit.Hook{
            Fn: func(ctrl orbit.FnControl, err error) error {
                logger.Println("[hook] Finally: job finished")
                return nil
            },
        },
    },
}
```
#### ✅ Sample Output:
```
[hook] OnStart: job starting...
[main] syncing data...
[hook] OnError: job failed: sync failed
[hook] Finally: job finished
```

#### 🧠 Best Practices
- Use hooks to **log**, **monitor**, or **recover state** cleanly.
- Return errors inside hooks if you want the execution to halt (unless `IgnoreError` is set).
- Combine with `ctrl.SaveData()` to track job-level metrics.


---



### ♻️**Retry Mechanism**: Automatically retry failed tasks with configurable strategy.
Orbit allows you to easily configure retry behavior for your jobs.
You can define whether a job should retry on failure, how many times, or even retry infinitely!

### 🧪 Example: Retry Scenarios
Let's configure three jobs with different retry strategies:
```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/osmike/orbit"
)

func main() {

    pool, _ := orbit.CreatePool(context.Background(), orbit.PoolConfig{
        MaxWorkers:    3,
        CheckInterval: 10 * time.Millisecond,
    }, nil)

    failFn := func(ctrl orbit.FnControl) error {
        return errors.New("oops, failed!")
    }

    // Define three jobs:
    jobs := []orbit.JobConfig{
        {
            ID: "no-retry",
            Fn: failFn,
            Retry: orbit.RetryConfig{
                Active: false, // No retries — fail once and stop
            },
            Interval: orbit.IntervalConfig{Time: 50 * time.Millisecond},
        },
        {
            ID: "three-retry",
            Fn: failFn,
            Retry: orbit.RetryConfig{
                Active: true,
                Count:  3, // Retry up to 3 times after initial failure
            },
            Interval: orbit.IntervalConfig{Time: 50 * time.Millisecond},
        },
        {
            ID: "infinite-retry",
            Fn: failFn,
            Retry: orbit.RetryConfig{
                Active: true,
                Count:  0, // 0 means infinite retries!
            },
            Interval: orbit.IntervalConfig{Time: 50 * time.Millisecond},
        },
    }

    pool.Run()

    // Add jobs to the pool
    for _, job := range jobs {
        _ = pool.AddJob(job)
    }

    // Let them run for a while
    time.Sleep(600 * time.Millisecond)

    // Retrieve metrics
    metrics := pool.GetMetrics()

    for id, m := range metrics {
        fmt.Printf("Job %s: failures = %d, status = %s\n", id, m.(orbit.JobState).Failure, m.(orbit.JobState).Status)
    }
}

```

| Job ID           | Behavior                         | Failure Count  | Final Status |
|:-----------------|:---------------------------------|:---------------|:-------------|
| `no-retry`        | Fails once, no retries           | 1               | error        |
| `three-retry`     | Fails + retries 3 times          | 4 (1+3)         | error        |
| `infinite-retry`  | Keeps retrying indefinitely      | ≥5 (and growing) | running or error depending on timing |

### 📌 Takeaway

- **`Retry.Active: false`** — the job fails once and **is automatically removed** from the pool.
- **`Retry.Count > 0`** — the job retries up to `Count` times; after that, it **is removed** from the pool.
- **`Retry.Count == 0`** — the job retries infinitely until it is manually stopped.
- `Retry.ResetOnSuccess: true` —
  After a successful execution, the retry counter is reset, allowing fresh retries if future failures occur.
- **Automatic job cleanup**: Orbit ensures that jobs which are no longer retryable do not clog the pool — they are **automatically deleted** after exhausting retries or immediately if retries are disabled.

⚡ **Tip:** Always monitor your retry settings carefully to balance fault-tolerance and resource usage.

⚡ **Important**
- Retries are scheduled naturally: After a failure, the job returns to the Waiting state and follows the normal CheckInterval + Interval or Cron scheduling for the next run.
- Jobs that exhaust retries or have no retries configured are cleanly removed to prevent resource leaks.

---
### 🛑 Graceful Shutdown: Safely terminate all jobs with Kill()
Orbit provides a reliable way to gracefully shutdown the entire pool and stop all running or waiting jobs safely via the `Kill()` method.

When you call `pool.Kill()`:
- All jobs are canceled through the shared `pool.Context()`.
- Each job updates its metrics with
  - `Status: stopped`
  - `Error: errs.ErrPoolShutdown`
- All jobs are removed from the pool.
- The pool is marked as killed and cannot be restarted.

### 🧪 Example: Graceful Kill
Let's create a simple pool with one long-running job and gracefully shut it down:
```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/osmike/orbit"
)

func main() {

    pool, _ := orbit.CreatePool(context.Background(), orbit.PoolConfig{
        MaxWorkers:    1,
        CheckInterval: 50 * time.Millisecond,
    }, nil)

    longJob := orbit.JobConfig{
        ID: "long-running-job",
        Fn: func(ctrl orbit.FnControl) error {
            fmt.Println("Job started")
            <-ctrl.Context().Done()
            fmt.Println("Job canceled:", ctrl.Context().Err())
            return ctrl.Context().Err()
        },
        Interval: orbit.IntervalConfig{Time: 1 * time.Hour}, // Never intended to complete
    }

    _ = pool.AddJob(longJob)

    pool.Run()

    time.Sleep(100 * time.Millisecond) // Give some time to start

    pool.Kill() // Gracefully shutdown

    time.Sleep(100 * time.Millisecond) // Wait a little for cleanup

    // Trying to run the pool again will fail
    if err := pool.Run(); err != nil {
        fmt.Println("Cannot restart killed pool:", err)
    }
}
```
| Step                   | What Happens                                          |
|-------------------------|--------------------------------------------------------|
| `Kill()` called         | Pool context is canceled; all jobs receive Done() signal |
| Jobs react to cancellation | Jobs detect `ctx.Done()` and exit if implemented correctly |
| Metrics update          | `Status = stopped`, `Error = ErrPoolShutdown` recorded in metrics |
| Pool cleanup            | Jobs are removed from internal storage |
| Pool is killed          | `Run()` will return an error if called again |
### 📌 Takeaway
- `Kill()` does not forcefully kill goroutines — it signals cancellation via context.
- Jobs should respect `ctx.Done()` to exit gracefully.
- After Kill(), the pool is permanently shut down.
- Metrics will reflect the final stopped state with the shutdown reason.
#### ⚡ Tip: Always design your jobs to listen to ctrl.Context().Done() if you want graceful shutdown behavior!
### ⚡ Quick Recap
```
p.Kill() 
→ ctx.Cancel() → jobs detect Done() 
→ update metrics (stopped + shutdown error) 
→ remove jobs 
→ pool is permanently dead
```

---
## 📊 Monitoring & Metrics

Orbit automatically tracks the state of each job in real time.

Collected metrics include:
| **Metric**                      | **Description**                               |
|-------------------------|--------------------------------------------------------------|
🕰 **Start Time** | When the job starts execution
🕰 **End Time** | When the job finishes execution
🏃 **Execution Time** | Total execution duration
✅ **Success Counts** | Number of successful executions
❌ **Failure Counts** | Number of failed executions
📌 **Custom User Metrics** | User-defined metadata (`SaveData`)
🚦 **Status** | Current job status (Waiting, Running, Completed, Error, Paused, Stopped, Ended)

## ⚙️ How monitoring works
Monitoring updates happen automatically at every significant lifecycle event and periodically while a job is running:

| Event	                                   | What gets saved                                                                           |
|------------------------------------------|-------------------------------------------------------------------------------------------|
| Job creation                             | 	Status set to `Waiting`, initial state saved                                             |
| Before job start (**Waiting state**)     | 	Orbit checks every `CheckInterval` (default 100ms) if `StartAt` time has arrived         |
| Job start                                | 	Status changed to `Running`, `StartAt` recorded, `NextRun` scheduled, `Data` wiped clean |
| During job execution (**Running state**) | 	Every `CheckInterval`, Orbit updates `ExecutionTime` and saves metrics                   |
| Execution error                          | 	Status set to `Error`, execution error saved                                             |
| Successful completion                    | 	Status set to `Completed`                                                                |
| Retry after failure                      | 	Status goes `Error` → `Completed` → `Waiting` automatically                              |
| Exhausted retries                        | 	Status set to `Ended`, job removed from the pool                                         |
| Pause/Resume                             | 	Status updated to `Paused` and back to `Running`                                         |
| Pool shutdown                            | 	All jobs forcibly set to `Stopped`, error `ErrPoolShutdown` attached                     |

✅ On every state change or execution time update, SaveMetrics() is called automatically.

## 🧩 Custom Monitoring
> **Orbit uses a Monitoring interface!**

You can implement your own Monitoring system easily (for example, sending metrics to Prometheus, DataDog, logs, etc.).

Just pass your custom implementation when creating a Pool:
```go
myMon := NewCustomMonitoring()

pool, _ := orb.CreatePool(orbit.PoolConfig{
    MaxWorkers: 10,
}, myMon)
```
If you don't provide a custom monitor, Orbit uses a built-in in-memory monitoring system by default.

To fetch metrics, simply call:
```go
metrics := pool.GetMetrics()
```
## 📜 Full Job Lifecycle
<div align="center">

```mermaid
flowchart TD
    classDef waiting fill:#ffeeba,stroke:#f0ad4e,color:#003366,stroke-width:2px
    classDef running fill:#c3e6cb,stroke:#28a745,color:#000000,stroke-width:2px
    classDef completed fill:#bee5eb,stroke:#17a2b8,color:#003366,stroke-width:2px
    classDef error fill:#f5c6cb,stroke:#dc3545,color:#000000,stroke-width:2px
    classDef ended fill:#d6d8db,stroke:#6c757d,color:#000000,stroke-width:2px
    classDef stopped fill:#e2e3e5,stroke:#6c757d,color:#003366,stroke-width:2px

    A[Job created<br/>SaveMetrics] --> B[Status: Waiting]
    B -->|CheckInterval| C{StartAt reached?}
    C -- No --> B
    C -- Yes --> D[Status: Running<br/>Wipe Data<br/>SaveMetrics]
    D -->|CheckInterval| E{Execution continues}
    E -- No error --> F[Status: Completed<br/>SaveMetrics]
    F -->|NextRun scheduled| B
    E -- Error --> G[Status: Error<br/>SaveMetrics]
    G -- Retry available --> F
    G -- Retries exhausted --> H[Status: Ended<br/>SaveMetrics<br/>Remove job]
    D -- Timeout or shutdown --> I[Status: Stopped<br/>SaveMetrics]

    class B waiting
    class D running
    class F completed
    class G error
    class H ended
    class I stopped

```

</div>

## 📌 Key Points
- Orbit checks jobs every `CheckInterval` (default 100ms).
- **ExecutionTime** is updated **periodically** while running — not just at the end!
- **Data is wiped** clean before every new execution (`Data = {}` at start).
- **Monitoring is fully pluggable** — just implement the `Monitoring` interface!

## 🧩 Example: Custom Monitoring with Prometheus
Orbit allows you to define your own custom Monitoring implementations.

You can seamlessly integrate Orbit with external observability systems like Prometheus, DataDog, New Relic, or any other monitoring backend.

Here’s a real-world example of integrating Orbit with Prometheus using its official Go client:

```go
package mymonitoring

import (
    "orbit"
    "github.com/prometheus/client_golang/prometheus"
)

// PrometheusMonitoring implements the Monitoring interface and exports Orbit job metrics to Prometheus.
type PrometheusMonitoring struct {
    jobExecutionTime *prometheus.GaugeVec
    jobSuccessCount  *prometheus.CounterVec
    jobFailureCount  *prometheus.CounterVec
}

// NewPrometheusMonitoring initializes Prometheus metrics and registers them.
func NewPrometheusMonitoring() *PrometheusMonitoring {
    pm := &PrometheusMonitoring{
        jobExecutionTime: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "orbit_job_execution_time_seconds",
                Help: "Execution time of a job in seconds",
            },
            []string{"job_id"},
        ),
        jobSuccessCount: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "orbit_job_success_total",
                Help: "Total number of successful job completions",
            },
            []string{"job_id"},
        ),
        jobFailureCount: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "orbit_job_failure_total",
                Help: "Total number of failed job executions",
            },
            []string{"job_id"},
        ),
    }

    // Register metrics
    prometheus.MustRegister(pm.jobExecutionTime)
    prometheus.MustRegister(pm.jobSuccessCount)
    prometheus.MustRegister(pm.jobFailureCount)

    return pm
}

// SaveMetrics collects and exports metrics from the given JobState.
func (pm *PrometheusMonitoring) SaveMetrics(dto orbit.JobState) {
    jobID := dto.JobID

    pm.jobExecutionTime.WithLabelValues(jobID).Set(float64(dto.ExecutionTime) / 1e9) // nanoseconds -> seconds
    pm.jobSuccessCount.WithLabelValues(jobID).Add(float64(dto.Success))
    pm.jobFailureCount.WithLabelValues(jobID).Add(float64(dto.Failure))
}

// GetMetrics (optional) returns dummy information since Prometheus pulls metrics externally.
func (pm *PrometheusMonitoring) GetMetrics() map[string]interface{} {
    return map[string]interface{}{
        "info": "Metrics are available at the Prometheus endpoint",
    }
}
```
### 🔥 Key points:
- SaveMetrics is automatically called every time a job's state changes (Start, Completion, Error, Retry, Pause, Resume, etc).
- You can track execution times, success/failure counters, status, and even custom runtime metadata.
- Prometheus scrapes the registered metrics automatically via the /metrics HTTP endpoint.
### 📌 Important
- Monitoring is fully pluggable via the Monitoring interface.
- You can provide your own implementation for Prometheus, DataDog, New Relic, InfluxDB, or even log them manually.
- If no custom Monitoring is passed when creating a Pool, Orbit uses its default lightweight in-memory Monitoring.
---

## 🗂 Project Structure

```
orbit/
├── internal/
│   ├── domain/     # Core domain definitions
│   ├── error/      # Custom error handling
│   ├── job/        # Job execution and lifecycle management
│   └── pool/       # Job pool management
├── mon.go      # Implementation of default monitoring storage
└── orbit.go    # Main scheduler API entry point
```

---
## 📚 Public API Reference

### 1. Constants

| Constant               | 	Description                                           |
|------------------------|--------------------------------------------------------|
| DEFAULT_NUM_WORKERS    | 	Default number of concurrent workers per pool (1000). |
| DEFAULT_CHECK_INTERVAL | 	Default interval to check job states (100ms).         |
| DEFAULT_IDLE_TIMEOUT   | 	Default timeout for considering a job idle (100h).    |
| DEFAULT_PAUSE_TIMEOUT  | 	Default timeout for pause acknowledgment (1s).        |
| MAX_END_AT             | 	Maximum end time for job execution (year 9999).       |

### 2. Types
## `JobStatus` string

Represents a job’s lifecycle state:
|Status | Description |
| --- | --- |
|`waiting` | The job is waiting to be executed.
|`running` | The job is currently executing. **Pool** monitors a job currently in the **"Running"** state, checking for execution timeouts or runtime errors. If the job exceeds its configured timeout, it is marked as **Error**, triggering its finalization and metric recording.
|`completed` | The job has finished execution successfully. When completed, pool checks if the job has future scheduled executions. If another execution is pending, the job state is reset to **"Waiting"**. Otherwise, the job is marked as **"Ended"**, indicating no further executions are planned.
|`paused` | The job is temporarily paused. Pause method attempts to pause the currently running job by sending a non-blocking signal to `pauseCh` (retrive via `FnControl.PauseChan() <-chan struct{}`). Once the pause signal is sent, a timeout watcher is started in a separate goroutine. If the job fails to acknowledge the pause (by reading from `pauseCh`) within the specified timeout, the job's status is automatically reverted back to Running.
|`stopped` |  The job has been explicitly stopped and will not run unless restarted. The method triggers immediate job cancellation via its execution context. When job is resumed via `pool.ResumeJob(id string)` its triggers recreation of job context usig pool context.
|`ended` | The job reached its defined end condition (e.g., end time or retry limit). When ended, job is removing from the pool and this triggers graceful cleanup with canceling context and closing channels
|`error` | The job encountered an error during execution. If retries are available, the job is rescheduled by setting status to Completed. If retries are exhausted or disabled, the job is finalized (Ended) and removed from the pool. (Note: In this case, execution errors are not stored in the final state.)

## `JobState` struct

Represents the runtime state of a job.
| Field  |  Type  |	Description |
| --- | --- | --- |
| JobID | `string` |	Unique identifier of the job associated with this state. |
| StartAt | `time.Time` |	Timestamp when the job execution started. |
| EndAt | `time.Time` |	Timestamp when the job execution ended (zero if still running). |
| Error | `StateError` |	Captures any execution errors and lifecycle hook errors. |
| Status | `JobStatus` |	Current lifecycle status (waiting, running, completed, error, etc.). |
| ExecutionTime | `int64` |	Duration of the job's execution (in nanoseconds). |
| Data | `map[string]interface{}` |	User-defined runtime key-value data captured during execution via FnControl.SaveData(). |
| Success | `int`	| Number of times the job has completed successfully. |
| Failure | `int`	| Number of times the job has failed. |
| NextRun | `time.Time` |	Scheduled time for the next execution (zero for one-time jobs). |

## `StateError` struct

Captures all types of errors that can occur during job execution.

| Field     | Type     | 	Description                                                              |
|-----------|----------|---------------------------------------------------------------------------|
| JobError  | `error`  | Error returned by the main job function (Fn).                             |
| HookError | `struct` | containing errors from lifecycle hooks (OnStart, OnError, Finally, etc.). |

Method:
- `IsEmpty() bool`:
  Returns true if both `JobError` and all `HookError` fields are `nil` — meaning no errors occurred during job execution or hooks.

## `HookError` struct

| Field     | Types   | 	Description                |
|-----------|---------|-----------------------------|
| OnStart   | `error` | 	Error from OnStart hook.   |
| OnStop    | `error` | 	Error from OnStop hook.    |
| OnError   | `error` | 	Error from OnError hook.   |
| OnSuccess | `error` | 	Error from OnSuccess hook. |
| OnPause   | `error` | 	Error from OnPause hook.   |
| OnResume  | `error` | 	Error from OnResume hook.  |
| Finally   | `error` | 	Error from Finally hook.   |

## `Job` struct

Configuration for creating a new job.

| Field    | Type                         | 	Description                                                              |
|----------|------------------------------|---------------------------------------------------------------------------|
| ID       | `string`                     | 	Required. Unique identifier of the job.                                  |
| Name     | `string`                     | 	Optional human-readable name (defaults to ID if empty).                  |
| Fn       | `func(ctrl FnControl) error` | 	Main execution function of the job, accepting a FnControl.               |
| Interval | `Interval`                   | 	Defines when and how often the job should run (either interval or cron). |
| Timeout  | `time.Duration`              | 	Maximum allowed time for job execution before cancellation.              |
| StartAt  | `time.Time`                  | 	Earliest time when the job can start (defaults to time.Now() if unset).  |
| EndAt    | `time.Time`                  | 	Latest time when the job can run (defaults to MAX_END_AT if unset).      |
| Retry    | `Retry`                      | 	Retry behavior configuration on execution failure.                       |
| Hooks    | `Hooks`                      | 	Optional lifecycle hooks for additional job logic at specific stages.    |

## `Retry` struct

Defines retry behavior for failed jobs.

| Field          | Type   | 	Description                                                                                                                    |
|----------------|--------|---------------------------------------------------------------------------------------------------------------------------------|
| Active         | `bool` | 	Enables or disables retry logic. If `Active: false` → **No** retries.                                                          |
| Count          | `int`  | 	Number of retry attempts allowed. `0` means infinite retries. If `Count > 0` → Retry up to Count times, then finalize the job. |
| ResetOnSuccess | `bool` | 	If true, resets retry counter after a successful execution.                                                                    |

## `Interval`

Defines job scheduling options.

|Field	|Type |Description|
| -- | -- | -- |
|Time | `time.Duration` |	Fixed duration between executions (e.g., every 5 minutes). |
| CronExpr | `string` |	Cron expression (e.g., `"0 0 * * *"` for midnight daily).|

Only one of `Time` or `CronExpr` should be set; setting both results in an `error`.

## `Hooks`

Lifecycle hooks that can run custom logic at different points during a job's life.

| Field     | Type   | 	Description                                                            |
|-----------|--------|-------------------------------------------------------------------------|
| OnStart   | `Hook` | 	Triggered right before the job starts.                                 |
| OnStop    | `Hook` | 	Triggered when the job is stopped.                                     |
| OnError   | `Hook` | 	Triggered if the main job function returns an error.                   |
| OnSuccess | `Hook` | 	Triggered if the job completes successfully.                           |
| OnPause   | `Hook` | 	Triggered when the job is paused.                                      |
| OnResume  | `Hook` | 	Triggered when the job is resumed.                                     |
| Finally   | `Hook` | 	Always triggered after job finishes, regardless of success or failure. |

## `Hook`
| Field       | Type                                    | 	Description                                                                                                                                                                                                     |
|-------------|-----------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Fn          | `func(ctrl FnControl, err error) error` | `Fn` is the function to be executed during the lifecycle stage. It receives the current `FnControl` and an optional `error` for handling error in `OnError` hook. It should return an `error` if the hook fails. |
| IgnoreError | `bool`                                  | `IgnoreError` determines whether hook execution errors should be ignored. If true, the job will proceed even if the hook fails. If false, the job will halt and treat the hook error as fatal.                   |

## `FnControl`

An interface providing control and metadata access inside job logic.

| Method                             | 	Description                                               |
|------------------------------------|------------------------------------------------------------|
| `SaveData(map[string]interface{})` | Save custom runtime data (merged into `JobState.Data`).    |
| `GetData() map[string]interface{}` | 	Retrieve a copy of the saved runtime data.                |
| `Context() context.Context`        | 	Execution context to monitor for cancellation or timeout. |
| `PauseChan() <-chan struct{}`      | 	Channel signaling the job to pause.                       |
| `ResumeChan() <-chan struct{}`     | 	Channel signaling the job to resume after pause.          |

## `PoolConfig`
Defines configuration settings for initializing a scheduler execution pool.

| Field         | 	Type            | 	Description                                                                                     |
|---------------|------------------|--------------------------------------------------------------------------------------------------|
| MaxWorkers    | 	`int`           | 	Maximum number of concurrent workers allowed. If `0`, defaults to `1000`.                       |
| CheckInterval | 	`time.Duration` | 	How frequently the pool checks jobs for execution eligibility. If `0`, defaults to `100ms`.     |
| IdleTimeout   | 	`time.Duration` | 	Maximum time a job can remain idle before being marked as inactive. If `0`, defaults to `100h`. |

This configuration is passed to scheduler.CreatePool(...) and determines how the pool handles parallel execution and job polling frequency.

## `Pool`
Represents a job execution engine. Manages job scheduling, concurrency, and lifecycle control.

| Method                                             | 	Description                                                                                          |
|----------------------------------------------------|-------------------------------------------------------------------------------------------------------|
| `AddJob(job Job) error`	                           | Adds a job to the pool. Fails if the `ID` already exists or status is not `Waiting`.                  |
| `RemoveJob(id string) error`	                      | Removes a job from the pool by `ID`. Does not stop the job explicitly.                                |
| `PauseJob(id string, timeout time.Duration) error` | Sends a pause signal to a running job. Timeout defines how long the job has to acknowledge the pause. |
| `ResumeJob(id string) error`	                      | Resumes a job from Paused or Stopped state. Recreates internal context from pool context.             |
| `StopJob(id string) error`	                        | Cancels job execution and updates status to Stopped.                                                  |
| `Run() error`	                                     | Starts the pool scheduler loop. Checks job states every CheckInterval and enforces MaxWorkers.        |
| `Kill()`	                                          | Immediately stops all jobs, cancels context, and marks the pool as shut down permanently.             |
| `GetMetrics() map[string]interface{}`	             | Returns collected metrics from the provided Monitoring implementation.                                |

## `Orbit`
Main orchestrator for pool and job lifecycle.

| Method                                             | 	Description                                                              |
|----------------------------------------------------|---------------------------------------------------------------------------|
| `CreatePool(ctx context.Context, cfg PoolConfig, mon Monitoring) *Pool` | 	Initializes a pool with the given configuration and monitoring strategy. |

___
## ⚖️ License

[MIT License](LICENSE)

---
🚀 **Ready to schedule smarter?** [Get Orbit now!](https://github.com/osmike/orbit)


---
