package domain

// Hooks represents lifecycle callback functions that can be optionally provided
// to manage and respond to various job execution events in the scheduler.
type Hooks struct {
	// OnStart is executed right before the job's main function (Fn) begins execution.
	// Returning an error from OnStart prevents the job from running.
	OnStart func(ctrl FnControl) error

	// OnStop is executed when a running job receives a stop signal.
	// It allows for graceful termination, cleanup, or state preservation before stopping the job completely.
	OnStop func(ctrl FnControl) error

	// OnError is executed whenever the job's main function (Fn) encounters an error.
	// This hook provides an opportunity for error handling, logging, notifications, or triggering recovery actions.
	OnError func(ctrl FnControl, err error)

	// OnSuccess is executed immediately after successful completion of the job's main function (Fn).
	// If OnSuccess returns an error, the job's final status will reflect the failure of this hook.
	OnSuccess func(ctrl FnControl) error

	// OnPause is triggered when a job transitions into a paused state.
	// It can be used for resource management, state tracking, or logging.
	// If OnPause returns an error, the job's final status will reflect the failure of this hook.
	OnPause func(ctrl FnControl) error

	// OnResume is executed when a paused job is resumed.
	// It allows reinitializing resources or state restoration required for the continuation of job execution.
	// If OnResume returns an error, the job's final status will reflect the failure of this hook.
	OnResume func(ctrl FnControl) error

	// Finally is always executed after job completion, regardless of success, error, or any interruptions.
	// Ideal for guaranteed cleanup operations, final resource releases, or logging outcomes.
	// If Finally returns an error, it overrides previous results and marks the job's final status as Error.
	Finally func(ctrl FnControl) error
}
