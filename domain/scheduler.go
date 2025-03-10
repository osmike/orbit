package domain

import "time"

type Schedule struct {
	Interval time.Duration
	CronExpr string
}
