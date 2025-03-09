package scheduler

import (
	"errors"
	"fmt"
)

var (
	ErrEmptyID           = errors.New("empty ID")
	ErrIDExists          = errors.New("job ID not unique")
	ErrEmptyFunction     = errors.New("function is empty")
	ErrWrongTime         = errors.New("wrong time")
	ErrMixedScheduleType = errors.New("job schedule is only supported with one type of interval")
	ErrAddingJob         = errors.New("error adding job")
	ErrJobNotFound       = errors.New("job not found")
)

var (
	ErrInvalidCronExpression = errors.New("invalid cron expression")
)

var (
	ErrJobPanicked = errors.New("job panicked")
	ErrJobTimout   = errors.New("job timed out")
)

func newErr(err error, str string) error {
	return fmt.Errorf("%w: %s", err, str)
}
