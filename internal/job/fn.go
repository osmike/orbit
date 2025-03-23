package job

import (
	"context"
	"sync"
)

// Fn defines the signature of functions executed by the scheduler.
// Each scheduled job must implement this function.
//
// Parameters:
//   - ctrl: Provides the job function with execution control, context management,
//     and metadata storage capabilities.
//
// Returns:
//   - An error if the job execution fails; otherwise, nil upon successful completion.
type Fn func(ctrl FnControl) error

// FnControl provides execution control mechanisms, runtime context,
// and metadata storage for a job function (Fn).
//
// It enables job functions to manage their lifecycle effectively by:
//   - Accessing the execution context to handle cancellations and timeouts.
//   - Responding to pause and resume signals.
//   - Storing and retrieving arbitrary job-specific data.
type FnControl struct {
	// Ctx is the execution context for the job, allowing cancellation detection.
	// The context may be canceled due to scheduler shutdown, manual termination,
	// or exceeding the configured job timeout.
	Ctx context.Context

	// PauseChan signals the job to pause execution. When the job function receives
	// a message on this channel, it should transition into a paused state.
	PauseChan chan struct{}

	// ResumeChan signals a previously paused job to resume execution. When the job
	// function receives a message on this channel, it can safely resume processing.
	ResumeChan chan struct{}

	// data stores custom key-value metadata generated or used by the job during execution.
	// This is useful for persisting execution state, debugging, logging, or reporting metrics.
	data *sync.Map
}

// SaveData stores custom runtime metadata for the job execution.
//
// The stored data is accessible throughout the job's lifecycle and can be
// leveraged by lifecycle hooks (OnStart, OnSuccess, etc.) or for post-execution analysis.
//
// Parameters:
//   - data: Key-value pairs representing custom metadata or execution state information.
func (ctrl *FnControl) SaveData(data map[string]interface{}) {
	for k, v := range data {
		ctrl.data.Store(k, v)
	}
}
