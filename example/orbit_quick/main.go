package main

import (
	"context"
	"fmt"
	"github.com/osmike/orbit"
	"time"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

func main() {
	pool, _ := orbit.CreatePool(context.Background(), orbit.PoolConfig{
		MaxWorkers: 10,
	}, nil)
	job := orbit.Job{
		ID: "job",
		Fn: func(ctrl orbit.FnControl) error {
			fmt.Printf("Hello from orbit\n")
			return nil
		},
		Interval: orbit.Interval{
			Time: 5 * time.Second,
		},
	}
	err := pool.AddJob(job)
	if err != nil {
		panic(err)
	}
	pool.Run()
	for range 5 {
		time.Sleep(5 * time.Second)
		fmt.Printf("metrics: %v", pool.GetMetrics())
	}
	pool.Kill()
}
