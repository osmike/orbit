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
//     pause/resume signaling, and metadata storage capabilities.
//
// Returns:
//   - An error if the job execution fails; otherwise, nil upon successful completion.
type Fn func(ctrl FnControl) error

// FnControl provides execution control mechanisms, runtime context,
// pause/resume signaling, and metadata storage for a job function (Fn).
//
// It enables job functions to manage their lifecycle effectively by:
//   - Accessing the execution context to handle cancellations and timeouts.
//   - Responding to pause and resume signals.
//   - Storing and retrieving arbitrary job-specific data.
type FnControl struct {
	// ctx is the execution context for the job, allowing cancellation detection.
	// The context may be canceled due to scheduler shutdown, manual termination,
	// or exceeding the configured job timeout.
	ctx context.Context

	// pauseChan signals the job to pause execution. When the job function receives
	// a message on this channel, it should transition into a paused state.
	pauseChan chan struct{}

	// resumeChan signals a previously paused job to resume execution. When the job
	// function receives a message on this channel, it can safely resume processing.
	resumeChan chan struct{}

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

// Context returns the job's execution context, which can be used to monitor
// cancellation or timeouts due to scheduler shutdown, manual stop, or time limits.
func (ctrl *FnControl) Context() context.Context {
	return ctrl.ctx
}

// PauseChan returns the channel used to signal that the job should pause.
// The job function should monitor this channel and suspend processing when a signal is received.
func (ctrl *FnControl) PauseChan() <-chan struct{} {
	return ctrl.pauseChan
}

// ResumeChan returns the channel used to signal that a paused job should resume execution.
// The job should monitor this channel and continue execution when a signal is received.
func (ctrl *FnControl) ResumeChan() <-chan struct{} {
	out := make(chan struct{})

	go func() {
		select {
		case <-ctrl.resumeChan:
			out <- struct{}{}
		case <-ctrl.ctx.Done():
			close(out)
		}
	}()

	return out
}
