// Example: Orbit + SQLite integration with pause/resume and stateful task execution.
// This example demonstrates how to:
//   - Initialize a job with hooks and runtime state
//   - Interact with a SQLite database across executions
//   - Pause and resume a long-running task mid-execution
//   - Track job metrics and internal state

package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/osmike/orbit"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const dbName = "data.db"

func main() {
	jID := "orbit_db_task"

	// Create a job with a task function and an OnStart hook.
	job := orbit.Job{
		ID: jID,
		Fn: task,
		Hooks: orbit.Hooks{
			OnStart: orbit.Hook{
				Fn:          onStartFn,
				IgnoreError: true,
			},
		},
		Interval: orbit.Interval{
			Time: 1 * time.Second,
		},
	}

	pool, err := orbit.CreatePool(context.Background(), orbit.PoolConfig{}, nil)
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}

	if err := pool.AddJob(job); err != nil {
		log.Fatalf("Failed to add job: %v", err)
	}

	fmt.Println("[Orbit] Scheduler started")
	pool.Run()

	// Pause and resume demonstration
	time.Sleep(1 * time.Second)
	fmt.Println("[Orbit] Pausing job...")
	pool.PauseJob(jID, 100*time.Millisecond)

	time.Sleep(6 * time.Second)
	fmt.Println("[Orbit] Resuming job...")
	pool.ResumeJob(jID)

	time.Sleep(7 * time.Second)
	fmt.Printf("[Orbit] Metrics: %+v\n", pool.GetMetrics())

	pool.Kill()
	fmt.Println("[Orbit] Pool terminated gracefully")
}

func onStartFn(ctrl orbit.FnControl, _ error) (resErr error) {
	fmt.Println("[Hook:OnStart] Initializing SQLite database...")

	_, err := os.Stat(dbName)
	needInit := os.IsNotExist(err)

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		return fmt.Errorf("unable to open database: %w", err)
	}

	if needInit {
		resErr = fmt.Errorf("database not found — initializing from scratch")
	} else {
		row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='t_test'`)
		var name string
		err := row.Scan(&name)
		if err == nil && name == "t_test" {
			row = db.QueryRow(`SELECT COUNT(*) FROM t_test`)
			var count int
			err = row.Scan(&count)
			if err == nil && count > 0 {
				ctrl.SaveData(map[string]interface{}{
					"init": map[string]interface{}{
						"count":     count,
						"tableName": "t_test",
						"db":        db,
					},
				})
				fmt.Printf("[Hook:OnStart] Found existing table 't_test' with %d rows\n", count)
				return nil
			}
		}
	}

	fmt.Println("[Hook:OnStart] Creating table and inserting demo data (1000 rows)...")
	_, err = db.Exec(`
		DROP TABLE IF EXISTS t_test;
		CREATE TABLE t_test (
			id INTEGER PRIMARY KEY,
			name TEXT,
			value REAL
		);
	`)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	stmt, err := tx.Prepare("INSERT INTO t_test (name, value) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for i := 1; i <= 1000; i++ {
		_, err := stmt.Exec(fmt.Sprintf("Record %d", i), float64(i)/100.0)
		if err != nil {
			return fmt.Errorf("insert: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	ctrl.SaveData(map[string]interface{}{
		"init": map[string]interface{}{
			"tableName": "t_test",
			"count":     1000,
			"db":        db,
		},
	})
	fmt.Println("[Hook:OnStart] Database initialized and seeded successfully")
	return nil
}

func task(ctrl orbit.FnControl) error {
	fmt.Println("[Task] Execution started")

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	data := ctrl.GetData()
	initData, ok := data["init"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("init data not found")
	}

	table, ok := initData["tableName"].(string)
	if !ok {
		return fmt.Errorf("table name not found")
	}
	count, ok := initData["count"].(int)
	if !ok {
		return fmt.Errorf("row count not found")
	}

	const pageSize = 50
	offset := 0

	for {
		select {
		case <-ctrl.Context().Done():
			fmt.Println("[Task] Execution cancelled")
			return nil
		case <-ctrl.PauseChan():
			fmt.Println("[Task] Paused")
			<-ctrl.ResumeChan()
			fmt.Println("[Task] Resumed")
		default:
			time.Sleep(300 * time.Millisecond)

			query := fmt.Sprintf(`
				SELECT id, name, value
				FROM %s
				ORDER BY id
				LIMIT ? OFFSET ?`, table,
			)
			rows, err := db.Query(query, pageSize, offset)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}

			for rows.Next() {
				var id int
				var name string
				var value float64
				if err := rows.Scan(&id, &name, &value); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				fmt.Printf("[Task] Row: ID=%d, Name='%s', Value=%.2f\n", id, name, value)
			}
			rows.Close()

			offset += pageSize
			if offset >= count {
				fmt.Println("[Task] All records processed")
				return nil
			}
		}
	}
}
