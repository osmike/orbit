
# 🚀 Orbit Scheduler – Task Scheduling Made Easy!

[![Go](https://img.shields.io/badge/Made%20with-Go-blue)](https://golang.org)
[![Coverage](https://img.shields.io/badge/Coverage-87%25-brightgreen)](https://github.com/osmike/orbit)
![MIT License](https://img.shields.io/badge/license-MIT-green.svg)

Orbit is a powerful yet intuitive job scheduler written entirely in Go. Effortlessly schedule, run, monitor, and manage your tasks, ensuring reliability and efficiency.

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
    orb := orbit.New(ctx)

    pool, _ := orb.CreatePool(ctx, orbit.PoolConfig{
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

    orb.AddJob(pool, jobCfg)
	// Run starts the main controlling goroutine for the pool.
	// It continuously manages job scheduling, execution, and lifecycle events.
    pool.Run()

    select {} // Keep running indefinitely
}
```

---

## 🛠 Features

- **🎮 Live Control**: Pause, Resume, or Stop jobs dynamically — as easily as managing a video or audio track.
  - **📅 Advanced Job Example**: Cron-Based Execution with State & Control
    You can define advanced jobs that run on a cron schedule,
    maintain execution state between intervals, and support pause/resume operations mid-execution.
    Here's a job that runs every Friday at 8 PM, pulls row count from a database, and uploads data in batches:
```go
onStartFn := func(ctrl orb.FnControl) error {
    state, err := ctrl.GetData()
    if err != nil {
        return err
    }

    rowCnt := db.GetRowCount()
    ctrl.SaveData(map[string]interface{}{
        "rowCnt":  rowCnt,
        "uploaded": 0,
    })

    fmt.Printf("Job %s started, row count: %d\n", state.JobID, rowCnt)
    return nil
}

mainFn := func(ctrl orb.FnControl) error {
    for {
        select {
            case <-ctrl.PauseChan():
                fmt.Println("Paused... waiting for resume")
            <-ctrl.ResumeChan()
                fmt.Println("Resumed, reconnecting to DB...")
                reconnectToDB()        
            case <-ctrl.Context().Done():
                return ctrl.Context().Err()
            
            default:
                state, err := ctrl.GetData()
                if err != nil {
                    return err
                }
    
                data := state.Data
                uploaded := data["uploaded"].(int)
                total := data["rowCnt"].(int)
    
                uploaded += db.BatchInsert(1000)
                ctrl.SaveData(map[string]interface{}{
                    "rowCnt":  total,
                    "uploaded": uploaded,
                })
    
                if uploaded >= total {
                    fmt.Println("✅ All data uploaded successfully.")
                    return nil
                }
        }
    }
}    
```
    Then configure the job like this:
```go
jobCfg := orbit.JobConfig{
    ID:       "weekly-upload",
    Name:     "Weekly DB Upload",
    Fn:       mainFn,
    OnStart:  onStartFn,
    Cron:     "0 20 * * 5", // Every Friday at 20:00 (8 PM)
}
```
    And add it to a running pool:
```go
orb.AddJob(pool, jobCfg)
```
   **🧠 Key Concepts**
    - **Persistent Job State**: Use `ctrl.SaveData()` and `ctrl.GetData()` to persist values (like counters, status flags, etc.) between runs or across iterations inside a job.
    - **Pause & Resume**: You can call `PauseJob(id string)` and `ResumeJob(id string)` at runtime from your application logic. The job can listen to `ctrl.PauseChan()` and `ctrl.ResumeChan()` to handle reconnections or resume where it left off.
    - **Graceful Shutdown**: Jobs respect ctrl.Context().Done() to terminate cleanly.
- **Concurrency Control**: Limit how many jobs run simultaneously.
- **Lifecycle Hooks**: Customize behavior with hooks (`OnStart`, `OnSuccess`, `OnError`, etc.).
- **Retry Mechanism**: Automatically retry failed tasks with configurable strategy.
- **Graceful Shutdown**: Ensures all jobs terminate safely and persist state.

---

## 📊 Monitoring & Metrics

Out-of-the-box job execution metrics include:

- **⏱ Start Time**
- **⏱ End Time**
- **⏱ Execution Time**
- **✅ Success Counts**
- **❌ Failure Counts**
- **📌 Custom User Metrics**


Integrate with popular monitoring tools or use the built-in storage for immediate insights.

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

## ⚖️ License

[MIT License](LICENSE)

---
🚀 **Ready to schedule smarter?** [Get Orbit now!](https://github.com/osmike/orbit)

