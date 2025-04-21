package domain

// Monitoring defines an interface for collecting, storing, and retrieving metrics related to job execution.
//
// Implementations of this interface can persist metrics in various ways, such as:
// - In-memory storage for simple debugging and development purposes.
// - Real-time logging for operational monitoring.
// - External systems like dashboards, time-series databases, or analytics platforms.
type Monitoring interface {
	// SaveMetrics stores execution metrics derived from a job's execution state (StateDTO).
	//
	// Implementations typically capture metrics like execution timestamps, duration,
	// final status, encountered errors, and user-defined runtime metadata.
	//
	// Parameters:
	//   - dto: StateDTO instance containing the job's execution details and metadata.
	SaveMetrics(dto StateDTO)
}
