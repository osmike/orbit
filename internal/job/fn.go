package job

import (
	"context"
	"sync"
)

// Fn defines the function signature that every job must implement.
// This function receives a FnControl instance, which allows interaction with the scheduler's execution flow.
// The function should return an error if the execution fails.
type Fn func(ctrl FnControl) error

// FnControl provides control mechanisms and contextual information to a running job.
// It allows the job to manage its execution by handling pause/resume signals and storing runtime metadata.
type FnControl struct {
	// Ctx is the execution context associated with the job.
	// It allows the job to detect when it is canceled due to timeout, scheduler shutdown, or manual termination.
	Ctx context.Context

	// PauseChan is a channel used to signal that the job should pause its execution.
	// When a signal is received on this channel, the job is expected to enter a paused state and stop further processing.
	PauseChan chan struct{}

	// ResumeChan is a channel used to signal that a paused job should resume execution.
	// When a signal is received on this channel, the job can continue running from where it left off.
	ResumeChan chan struct{}

	// data is a map that stores job-specific metadata.
	// This allows the job to persist and retrieve contextual information dynamically during execution.
	data *sync.Map
}

// SaveUserInfo allows a running job to store custom key-value metadata.
// This can be used for logging, debugging, or sharing execution state across multiple runs of the same job.
func (ctrl *FnControl) SaveUserInfo(data map[string]string) {
	for key, val := range data {
		ctrl.data.Store(key, val)
	}
}
