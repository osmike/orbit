package job

import (
	"context"
)

// Fn defines the signature of a job's main execution function.
//
// This function is invoked by the scheduler at runtime and represents
// the core logic of a scheduled task.
//
// Parameters:
//   - ctrl: FnControl instance providing access to execution context, pause/resume
//     signaling, and runtime metadata storage.
//
// Returns:
//   - An error if the job execution fails, or nil on successful completion.
type Fn func(ctrl FnControl) error

// FnControl provides the job's execution function (Fn) with control mechanisms,
// lifecycle coordination, and runtime metadata handling.
//
// It exposes capabilities for:
//   - Context management (cancellation, timeout).
//   - Pause/resume signaling support.
//   - Saving runtime key-value data for inspection or lifecycle hooks.
type FnControl struct {
	ctx        context.Context               // Execution context (cancellable by timeout or external stop)
	pauseChan  chan struct{}                 // Signal to pause job execution
	resumeChan chan struct{}                 // Signal to resume a paused job
	saveData   func(map[string]interface{})  // Internal callback for saving metadata to job state
	getData    func() map[string]interface{} // Internal callback used to retrieve a safe copy of the job’s execution state.
}

// SaveData stores custom key-value data generated during job execution.
//
// This metadata becomes part of the job's execution state and is retained
// across lifecycle hooks and monitoring systems.
//
// Parameters:
//   - data: Map of user-defined runtime data to persist.
func (ctrl *FnControl) SaveData(data map[string]interface{}) {
	ctrl.saveData(data)
}

// GetData returns a copy of the job's current saved runtime data.
//
// This method allows the job to access the latest key-value pairs previously stored via SaveData.
// It can be used inside the main function (Fn) or any lifecycle hook (OnStart, OnSuccess, etc.).
//
// Returns:
//   - map[string]interface{}: A snapshot of the user-defined runtime data.
func (ctrl *FnControl) GetData() map[string]interface{} {
	return ctrl.getData()
}

// Context returns the execution context associated with this job.
//
// The context is used to monitor cancellation or timeout events
// triggered by scheduler shutdown or job configuration limits.
//
// Returns:
//   - context.Context instance for cancellation awareness.
func (ctrl *FnControl) Context() context.Context {
	return ctrl.ctx
}

// PauseChan exposes the channel used to notify a job that it should pause.
//
// Job functions should monitor this channel and enter a paused state
// upon receiving a signal (e.g., via select-case).
//
// Returns:
//   - <-chan struct{}: read-only pause signal channel.
func (ctrl *FnControl) PauseChan() <-chan struct{} {
	return ctrl.pauseChan
}

// ResumeChan provides a channel that signals when a paused job should resume.
//
// This wrapper around the internal resume channel ensures that if the context
// is canceled while the job is paused, the resume channel will also be closed
// to prevent goroutine leaks.
//
// Returns:
//   - <-chan struct{}: read-only channel for resume signaling.
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
