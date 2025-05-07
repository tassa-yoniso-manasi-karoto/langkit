package core

import (
	"sync"
	"time"
)

// ETAProvider defines the interface for ETA calculation implementations
type ETAProvider interface {
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
}


// SimpleETACalculator provides a basic ETA calculator based purely on cross-multiplication
// It implements the ETAProvider interface with a much simpler approach than ETACalculator
type SimpleETACalculator struct {
	startTime      time.Time
	totalTasks     int64
	completedTasks int64
	mu             sync.RWMutex
}

// NewSimpleETACalculator creates a new SimpleETACalculator
func NewSimpleETACalculator(totalTasks int64) *SimpleETACalculator {
	return &SimpleETACalculator{
		startTime:      time.Now(),
		totalTasks:     totalTasks,
		completedTasks: 0,
	}
}

// TaskCompleted informs the calculator that a task has been completed
func (e *SimpleETACalculator) TaskCompleted(tasksCompleted int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
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
		}
	}
	
	// If no tasks completed, return no estimate
	if e.completedTasks == 0 {
		return result
	}
	
	elapsedTotal := time.Since(e.startTime)
	
	// Simple cross-multiplication:
	// time_total = time_elapsed * (total_tasks / completed_tasks)
	// time_remaining = time_total - time_elapsed
	estimate := time.Duration(float64(elapsedTotal) / percentDone - float64(elapsedTotal))
	
	// Apply a minor pessimism factor to avoid being too optimistic
	pessimismFactor := 1.05 // 5% pessimism
	adjustedEstimate := time.Duration(float64(estimate) * pessimismFactor)
	
	// For simple ETAs, create a modest range around the estimate
	rangeMultiplier := 1.10 // 10% range
	lowerBound := time.Duration(float64(adjustedEstimate) / rangeMultiplier)
	upperBound := time.Duration(float64(adjustedEstimate) * rangeMultiplier)
	
	// For simple ETAs, reliability increases with percentage done
	// Start with 0.7 reliability and increase to 0.95 as we approach completion
	reliability := 0.7 + (percentDone * 0.25)
	if reliability > 0.95 {
		reliability = 0.95
	}
	
	// Fill the result
	result = ETAResult{
		Estimate:         adjustedEstimate,
		LowerBound:       lowerBound,
		UpperBound:       upperBound,
		ReliabilityScore: reliability,
		SampleCount:      1, // Just one sample for simple calculation
		PercentDone:      percentDone,
		CrossMultETA:     estimate,
		CrossMultWeight:  1.0, // Always 100% cross-multiplication
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