package domain

// Hooks represents lifecycle callback functions that can be optionally provided
// to manage and respond to various job execution events in the scheduler.
type Hooks struct {
	// OnStart is executed right before the job's main function (Fn) begins execution.
	// Returning an error from OnStart prevents the job from running.
	OnStart Hook

	// OnStop is executed when a running job receives a stop signal.
	// It allows for graceful termination, cleanup, or state preservation before stopping the job completely.
	OnStop Hook

	// OnError is executed whenever the job's main function (Fn) encounters an error.
	// This hook provides an opportunity for error handling, logging, notifications, or triggering recovery actions.
	OnError Hook

	// OnSuccess is executed immediately after successful completion of the job's main function (Fn).
	// If OnSuccess returns an error, the job's final status will reflect the failure of this hook.
	OnSuccess Hook

	// OnPause is triggered when a job transitions into a paused state.
	// It can be used for resource management, state tracking, or logging.
	// If OnPause returns an error, the job's final status will reflect the failure of this hook.
	OnPause Hook

	// OnResume is executed when a paused job is resumed.
	// It allows reinitializing resources or state restoration required for the continuation of job execution.
	// If OnResume returns an error, the job's final status will reflect the failure of this hook.
	OnResume Hook

	// Finally is always executed after job completion, regardless of success, error, or any interruptions.
	// Ideal for guaranteed cleanup operations, final resource releases, or logging outcomes.
	// If Finally returns an error, it overrides previous results and marks the job's final status as Error.
	Finally Hook
}

// Hook defines a lifecycle callback that is executed at specific stages of job execution.
//
// Each hook provides optional behavior such as logging, metrics, notifications, etc.,
// and can either allow or block job execution depending on its result.
type Hook struct {
	// Fn is the function to be executed during the lifecycle stage.
	// It receives the current FnControl and an optional error from the previous step (if applicable).
	// It should return an error if the hook fails.
	Fn func(ctrl FnControl, err error) error

	// IgnoreError determines whether hook execution errors should be ignored.
	// If true, the job will proceed even if the hook fails.
	// If false, the job will halt and treat the hook error as fatal.
	IgnoreError bool
}

// HookError captures individual errors encountered in job lifecycle hooks.
//
// Each field corresponds to a specific lifecycle phase where a hook may be executed.
type HookError struct {
	// OnStart captures errors raised by the OnStart hook.
	OnStart error

	// OnStop captures errors raised by the OnStop hook.
	OnStop error

	// OnError captures errors raised by the OnError hook,
	// which is typically executed after a failed job function.
	OnError error

	// OnSuccess captures errors raised by the OnSuccess hook,
	// which is called after the job function completes successfully.
	OnSuccess error

	// OnPause captures errors raised by the OnPause hook,
	// which is triggered when the job is paused manually.
	OnPause error

	// OnResume captures errors raised by the OnResume hook,
	// which is triggered when a paused job is resumed.
	OnResume error

	// Finally captures errors raised by the Finally hook,
	// which always runs after execution, regardless of outcome.
	Finally error
}
