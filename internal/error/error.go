package error

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
	ErrTooManyJobs = errors.New("too many jobs")
)

var (
	ErrInvalidCronExpression = errors.New("invalid cron expression")
)

var (
	ErrJobPanicked        = errors.New("job panicked")
	ErrJobTimout          = errors.New("job timed out")
	ErrJobExecution       = errors.New("job exec before start time")
	ErrJobExecWrongStatus = errors.New("job execution with wrong status")
	ErrJobExecTooEarly    = errors.New("job execution too early")
	ErrJobExecAfterEnd    = errors.New("job execution after end time")
)

func New(err error, str string) error {
	return fmt.Errorf("%w: %s", err, str)
}
