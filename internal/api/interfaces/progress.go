package interfaces

// ProgressReporter provides progress bar updates for long-running operations.
type ProgressReporter interface {
	IncrementProgress(taskID string, increment, total, priority int, operation, descr, size string)
	RemoveProgressBar(taskID string)
}
