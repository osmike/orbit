package scheduler

import "time"

const (
	DEFAULT_NUM_WORKERS    = 1000
	DEFAULT_CHECK_INTERVAL = 100 * time.Millisecond
	DEFAULT_IDLE_TIMEOUT   = 100 * time.Hour
)

var (
	MAX_END_AT = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
)
