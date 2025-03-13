package domain

import "time"

type Pool struct {
	IdleTimeout   time.Duration
	MaxWorkers    int
	CheckInterval time.Duration
}
