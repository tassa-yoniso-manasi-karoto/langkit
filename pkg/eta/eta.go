package eta

import "time"

// AlgorithmType specifies the type of ETA calculation algorithm used.
type AlgorithmType int

const (
	AlgorithmUnknown AlgorithmType = iota
	AlgorithmSimple
	AlgorithmAdvanced
)

// String returns a string representation of the algorithm type
func (at AlgorithmType) String() string {
	switch at {
	case AlgorithmSimple:
		return "Simple"
	case AlgorithmAdvanced:
		return "Advanced"
	default:
		return "Unknown"
	}
}

// Time thresholds for ETA calculation and display
const (
	SimpleETAMinimumElapsed  = 2 * time.Second // Minimum elapsed time to show SimpleETACalculator estimate
	SimpleETAPessimismFactor = 1.05            // 5% pessimism for SimpleETACalculator
	SimpleETARangeMultiplier = 1.10            // 10% range for SimpleETACalculator
	MinBulkProgressElapsed   = 5 * time.Second // Minimum elapsed time to show ETA for bulk updates
	SimpleETAMinimumProgress = 0.15            // Minimum progress (15%) before showing ETA
)

// ETAResult represents an ETA calculation with estimate ranges
type ETAResult struct {
	Estimate         time.Duration // Point estimate
	LowerBound       time.Duration // Lower estimate bound
	UpperBound       time.Duration // Upper estimate bound
	ReliabilityScore float64       // Reliability indicator (0.0-1.0), calibrated to interval width
	SampleCount      int           // Number of effective rate samples used
	PercentDone      float64       // Percentage of tasks completed (0.0-1.0)
	Algorithm        AlgorithmType // The algorithm type that produced this result
	Variability      float64       // Coefficient of variation of processing rates
	CumulativeRate   float64       // Global rate: total items / total time (items/second)
}

// Provider defines the interface for ETA calculation implementations
type Provider interface {
	// TaskCompleted informs the calculator that a task has been completed
	TaskCompleted(tasksCompleted int64)

	// CalculateETA returns the estimated time remaining
	CalculateETA() time.Duration

	// CalculateETAWithConfidence returns detailed ETA information
	CalculateETAWithConfidence() ETAResult

	// Progress returns the current progress as a percentage (0-100)
	Progress() float64

	// GetCompletedTasks returns the current number of completed tasks
	GetCompletedTasks() int64

	// GetTotalTasks returns the total number of tasks
	GetTotalTasks() int64

	// ElapsedTime returns the time elapsed since the beginning of the operation
	ElapsedTime() time.Duration

	// UpdateTotalTasks updates the total task count without resetting rate statistics
	UpdateTotalTasks(newTotalTasks int64)

	// GetAlgorithmType returns the type of algorithm used by this calculator
	GetAlgorithmType() AlgorithmType
}
