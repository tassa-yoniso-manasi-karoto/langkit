package eta

import (
	"sync"
	"time"
)

// SimpleETACalculator provides a basic ETA calculator based purely on cross-multiplication
// It implements the ETAProvider interface with a much simpler approach than ETACalculator
type SimpleETACalculator struct {
	startTime       time.Time
	totalTasks      int64
	completedTasks  int64
	initialProgress int64   // Tasks already completed at start
	mu              sync.RWMutex
}

// NewSimpleETACalculator creates a new SimpleETACalculator
func NewSimpleETACalculator(totalTasks int64) *SimpleETACalculator {
	return &SimpleETACalculator{
		startTime:       time.Now(),
		totalTasks:      totalTasks,
		completedTasks:  0,
		initialProgress: 0,
	}
}

// TaskCompleted informs the calculator that a task has been completed
func (e *SimpleETACalculator) TaskCompleted(tasksCompleted int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// If this is the first update and we're starting with progress already done,
	// record that as our initial progress
	if e.completedTasks == 0 && tasksCompleted > 0 {
		e.initialProgress = tasksCompleted
	}

	e.completedTasks = tasksCompleted
}

// CalculateETA returns an estimated time remaining using cross-multiplication
func (e *SimpleETACalculator) CalculateETA() time.Duration {
	result := e.CalculateETAWithConfidence()
	return result.Estimate
}

// CalculateETAWithConfidence returns an ETAResult with simple cross-multiplication
func (e *SimpleETACalculator) CalculateETAWithConfidence() ETAResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	percentDone := float64(0)
	if e.totalTasks > 0 {
		percentDone = float64(e.completedTasks) / float64(e.totalTasks)
	}

	// Default result for error cases
	result := ETAResult{
		Estimate:         -1,
		LowerBound:       -1,
		UpperBound:       -1,
		ReliabilityScore: 0,
		SampleCount:      0,
		PercentDone:      percentDone,
		Algorithm:        AlgorithmSimple,
	}

	// If task is already complete, return zero ETA
	if e.completedTasks >= e.totalTasks {
		return ETAResult{
			Estimate:         0,
			LowerBound:       0,
			UpperBound:       0,
			ReliabilityScore: 1.0,
			SampleCount:      0,
			PercentDone:      1.0,
			Algorithm:        AlgorithmSimple,
		}
	}

	// Calculate tasks completed during this session
	tasksDoneThisSession := e.completedTasks - e.initialProgress

	// Minimum session tasks: relative to total size instead of a fixed constant.
	// For a 10-file run need ~2 done, for a 4-file run need 1, caps at 5.
	minTasks := int64(e.totalTasks / 4)
	if minTasks < 1 {
		minTasks = 1
	}
	if minTasks > 5 {
		minTasks = 5
	}

	// If not enough tasks completed in this session OR elapsed time is too short OR progress is too low, return no estimate
	if tasksDoneThisSession < minTasks ||
		time.Since(e.startTime) < SimpleETAMinimumElapsed ||
		percentDone < SimpleETAMinimumProgress {
		return result
	}

	// Current remaining tasks
	remainingTasks := e.totalTasks - e.completedTasks

	// Time spent on this session
	elapsedTime := time.Since(e.startTime)

	// Re-implement using standard cross-multiplication:
	// time_total_for_remaining = (time_elapsed / tasks_done_this_session) * remaining_tasks
	// Simple cross-multiplication for remaining time
	estimate := time.Duration(float64(elapsedTime) * float64(remainingTasks) / float64(tasksDoneThisSession))

	// Apply a minor pessimism factor to avoid being too optimistic
	adjustedEstimate := time.Duration(float64(estimate) * SimpleETAPessimismFactor)

	// For simple ETAs, create a modest range around the estimate
	lowerBound := time.Duration(float64(adjustedEstimate) / SimpleETARangeMultiplier)
	upperBound := time.Duration(float64(adjustedEstimate) * SimpleETARangeMultiplier)

	result = ETAResult{
		Estimate:         adjustedEstimate,
		LowerBound:       lowerBound,
		UpperBound:       upperBound,
		ReliabilityScore: 0.0,
		SampleCount:      1,
		PercentDone:      percentDone,
		Algorithm:        AlgorithmSimple,
	}

	return result
}

// Progress returns the current progress as a percentage (0-100)
func (e *SimpleETACalculator) Progress() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if e.totalTasks == 0 {
		return 100.0
	}
	
	return float64(e.completedTasks) / float64(e.totalTasks) * 100.0
}

// GetCompletedTasks returns the current number of completed tasks
func (e *SimpleETACalculator) GetCompletedTasks() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.completedTasks
}

// GetTotalTasks returns the total number of tasks
func (e *SimpleETACalculator) GetTotalTasks() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalTasks
}

// ElapsedTime returns the time elapsed since the beginning of the operation
func (e *SimpleETACalculator) ElapsedTime() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return time.Since(e.startTime)
}

// UpdateTotalTasks updates the total task count
func (e *SimpleETACalculator) UpdateTotalTasks(newTotalTasks int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// No need to update if the count is the same
	if newTotalTasks == e.totalTasks {
		return
	}
	
	// Simply update the total task count
	e.totalTasks = newTotalTasks
}

// GetAlgorithmType returns the type of algorithm used by this calculator
func (e *SimpleETACalculator) GetAlgorithmType() AlgorithmType {
	return AlgorithmSimple
}