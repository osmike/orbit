// Example: Orbit + zap structured logging integration
// Demonstrates how to use a production-grade logger in job execution and hooks

package main

import (
	"context"
	"errors"
	"github.com/osmike/orbit"
	"go.uber.org/zap"
	"math/rand"
	"time"
)

var logger *zap.Logger

func main() {
	// Initialize zap logger (development config for human-readable output)
	logger, _ = zap.NewDevelopment()
	defer logger.Sync()

	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		CheckInterval: 500 * time.Millisecond,
	}, nil)
	if err != nil {
		logger.Fatal("Failed to create Orbit pool", zap.Error(err))
	}

	job := orbit.Job{
		ID: "loggable_job",
		Fn: runWithLogging,
		Hooks: orbit.Hooks{
			OnStart:   orbit.Hook{Fn: logStart},
			OnSuccess: orbit.Hook{Fn: logSuccess},
			OnError:   orbit.Hook{Fn: logError},
			Finally:   orbit.Hook{Fn: logFinally},
		},
		Interval: orbit.Interval{Time: 2 * time.Second},
	}

	if err := pool.AddJob(job); err != nil {
		logger.Fatal("Failed to add job", zap.Error(err))
	}

	logger.Info("Scheduler started")
	pool.Run()

	time.Sleep(15 * time.Second)
	pool.Kill()
	logger.Info("Scheduler stopped")
}

func runWithLogging(ctrl orbit.FnControl) error {
	jobID := ctrl.GetData()["job_id"]
	logger.Info("Job execution started", zap.Any("job", jobID))
	time.Sleep(time.Duration(rand.Intn(800)+200) * time.Millisecond)
	if rand.Float32() < 0.3 {
		return errors.New("simulated job failure")
	}
	return nil
}

func logStart(ctrl orbit.FnControl, _ error) error {
	logger.Info("OnStart hook executed", zap.String("hook", "OnStart"))
	ctrl.SaveData(map[string]interface{}{"job_id": "loggable_job"})
	return nil
}

func logSuccess(ctrl orbit.FnControl, _ error) error {
	logger.Info("Job executed successfully", zap.String("hook", "OnSuccess"))
	return nil
}

func logError(ctrl orbit.FnControl, execErr error) error {
	logger.Error("Job failed", zap.String("hook", "OnError"), zap.Error(execErr))
	return nil
}

func logFinally(ctrl orbit.FnControl, _ error) error {
	logger.Info("Finally hook executed", zap.String("hook", "Finally"))
	return nil
}
