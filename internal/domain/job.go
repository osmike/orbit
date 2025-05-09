package domain

import (
	"context"
	"time"
)

// Job represents an executable task managed by the scheduler.
// It encapsulates lifecycle management methods, status updates, execution logic, and retry mechanisms.
//
// Each Job implementation must provide thread-safe access to internal state and logic,
// ensuring safe concurrent interactions within the Pool environment.
type Job interface {
	// GetMetadata returns the job's configuration metadata.
	GetMetadata() JobDTO

	// SetTimeout sets the max allowed job execution time; 0 disables timeout tracking.
	SetTimeout(timeout time.Duration)

	// GetStatus returns the current execution status of the job.
	GetStatus() JobStatus

	// SetStatus forcefully sets the job's current status.
	SetStatus(status JobStatus)

	TrySetStatus(allowed []JobStatus, status JobStatus) bool
	// UpdateState partially updates the job's state with non-zero fields from the provided DTO.
	UpdateState(state StateDTO)

	GetState() *StateDTO

	// GetNextRun calculates and returns the next scheduled execution time for the job.
	GetNextRun() time.Time

	// ProcessStart marks the job's state as "Running", initializing execution metrics and metadata.
	ProcessStart()

	// ProcessRun monitors the job during execution, checking for timeout conditions.
	// Returns ErrJobTimeout if the job exceeds its configured timeout.
	ProcessRun() error

	// ProcessEnd finalizes the job state after execution completes, recording metrics and handling errors.
	ProcessEnd(status JobStatus, err error)

	// ProcessError performs retry logic and updates the job state in case of failure.
	ProcessError() error

	// CanExecute checks if the job meets conditions required to start execution immediately.
	// Returns an error indicating why execution is not allowed, or nil if eligible.
	CanExecute() error

	// Execute performs the job's main execution logic, handling internal lifecycle hooks and error handling.
	Execute() error

	// Stop forcibly stops job execution and updates its state accordingly.
	Stop() error

	// Pause attempts to pause job execution with a specified timeout.
	Pause(timeout time.Duration) error

	// Resume resumes execution of a paused job, if applicable.
	Resume(ctx context.Context) error

	// LockJob acquires exclusive execution access to the job if it is available.
	LockJob() bool

	// UnlockJob releases exclusive execution access to the job.
	UnlockJob()
}
