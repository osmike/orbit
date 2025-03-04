package scheduler

import "errors"

var (
	ErrEmptyID       = errors.New("empty ID")
	ErrEmptyFunction = errors.New("function is empty")
	ErrWrongTime     = errors.New("wrong time")
	ErrAddingJob     = errors.New("error adding job")
	ErrJobNotFound   = errors.New("job not found")
)
