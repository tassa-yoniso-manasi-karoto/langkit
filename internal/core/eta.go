package core

import (
	"math"
	"sync"
	"time"
)

// ETACalculator provides a more accurate ETA for CPU-bound concurrent tasks
type ETACalculator struct {
	startTime       time.Time
	totalTasks      int64
	completedTasks  int64
	mu              sync.RWMutex
	samplingHistory []samplePoint
	historySize     int
	lastUpdateTime  time.Time
}

type samplePoint struct {
	timestamp     time.Time
	tasksComplete int64
}

// NewETACalculator creates a new ETACalculator
func NewETACalculator(totalTasks int64) *ETACalculator {
	now := time.Now()
	return &ETACalculator{
		startTime:       now,
		totalTasks:      totalTasks,
		completedTasks:  0,
		historySize:     20, // History size to maintain for weighted calculations
		samplingHistory: []samplePoint{{timestamp: now, tasksComplete: 0}},
		lastUpdateTime:  now,
	}
}

// TaskCompleted informs the calculator that a task has been completed
func (e *ETACalculator) TaskCompleted(tasksCompleted int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	now := time.Now()
	e.completedTasks = tasksCompleted
	
	// Only add sample points when enough time has passed (avoid too frequent updates)
	if now.Sub(e.lastUpdateTime) >= 500*time.Millisecond {
		e.lastUpdateTime = now
		e.samplingHistory = append(e.samplingHistory, samplePoint{
			timestamp:     now,
			tasksComplete: tasksCompleted,
		})
		
		// Trim history if it's too large
		if len(e.samplingHistory) > e.historySize {
			e.samplingHistory = e.samplingHistory[1:]
		}
	}
}

// CalculateETA returns the estimated time remaining in seconds
// Returns -1 if no tasks completed yet or if calculations fail
func (e *ETACalculator) CalculateETA() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if e.completedTasks == 0 {
		return -1
	}
	
	if e.completedTasks >= e.totalTasks {
		return 0
	}
	
	// Use multiple strategies and combine them
	eta1 := e.calculateLinearProjection()
	eta2 := e.calculateWeightedAverage()
	eta3 := e.calculateMovingWindowRate()
	
	// Combine the estimates with different weights
	// - Recent progress gets higher weight when we have enough history
	// - Overall average gets higher weight at the beginning
	historyWeight := math.Min(float64(len(e.samplingHistory))/10.0, 1.0)
	
	weightEta1 := 0.4 * (1.0 - historyWeight) // Linear projection (overall average)
	weightEta2 := 0.4 * historyWeight         // Weighted average (recent progress weighted higher)
	weightEta3 := 0.2                         // Moving window rate (very recent progress)
	
	// Combine estimates with weights
	combinedEta := (eta1.Seconds() * weightEta1) + 
	               (eta2.Seconds() * weightEta2) + 
	               (eta3.Seconds() * weightEta3)
	
	return time.Duration(combinedEta * float64(time.Second))
}

// Progress returns the current progress as a percentage (0-100)
func (e *ETACalculator) Progress() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if e.totalTasks == 0 {
		return 100.0
	}
	
	return float64(e.completedTasks) / float64(e.totalTasks) * 100.0
}

// calculateLinearProjection uses the overall average rate to project completion
func (e *ETACalculator) calculateLinearProjection() time.Duration {
	elapsed := time.Since(e.startTime)
	
	// Calculate overall rate (tasks per second)
	overallRate := float64(e.completedTasks) / elapsed.Seconds()
	
	if overallRate <= 0 {
		return -1
	}
	
	// Calculate remaining tasks and time
	remainingTasks := e.totalTasks - e.completedTasks
	remainingTimeSeconds := float64(remainingTasks) / overallRate
	
	return time.Duration(remainingTimeSeconds * float64(time.Second))
}

// calculateWeightedAverage uses a weighted average of rates, favoring recent samples
func (e *ETACalculator) calculateWeightedAverage() time.Duration {
	if len(e.samplingHistory) < 2 {
		return e.calculateLinearProjection()
	}
	
	var totalWeight float64
	var weightedRate float64
	
	// Calculate weighted rates, with more recent samples getting higher weights
	for i := 1; i < len(e.samplingHistory); i++ {
		current := e.samplingHistory[i]
		previous := e.samplingHistory[i-1]
		
		timeInterval := current.timestamp.Sub(previous.timestamp).Seconds()
		tasksDone := current.tasksComplete - previous.tasksComplete
		
		if timeInterval > 0 && tasksDone > 0 {
			rate := float64(tasksDone) / timeInterval
			
			// Weight is higher for more recent samples (exponential weighting)
			weight := math.Pow(1.5, float64(i))
			
			weightedRate += rate * weight
			totalWeight += weight
		}
	}
	
	if totalWeight == 0 || weightedRate == 0 {
		return e.calculateLinearProjection()
	}
	
	// Calculate final weighted rate
	finalRate := weightedRate / totalWeight
	
	// Calculate ETA based on weighted rate
	remainingTasks := e.totalTasks - e.completedTasks
	remainingTimeSeconds := float64(remainingTasks) / finalRate
	
	return time.Duration(remainingTimeSeconds * float64(time.Second))
}

// calculateMovingWindowRate uses only the most recent window of time to calculate rate
func (e *ETACalculator) calculateMovingWindowRate() time.Duration {
	historyLen := len(e.samplingHistory)
	
	// Use only the last 3-5 samples for very recent rate calculation
	windowSize := int(math.Min(5, float64(historyLen)))
	
	if windowSize < 2 {
		return e.calculateLinearProjection()
	}
	
	startIndex := historyLen - windowSize
	startSample := e.samplingHistory[startIndex]
	endSample := e.samplingHistory[historyLen-1]
	
	windowTimeSeconds := endSample.timestamp.Sub(startSample.timestamp).Seconds()
	windowTasksDone := endSample.tasksComplete - startSample.tasksComplete
	
	if windowTimeSeconds <= 0 || windowTasksDone <= 0 {
		return e.calculateLinearProjection()
	}
	
	recentRate := float64(windowTasksDone) / windowTimeSeconds
	remainingTasks := e.totalTasks - e.completedTasks
	remainingTimeSeconds := float64(remainingTasks) / recentRate
	
	return time.Duration(remainingTimeSeconds * float64(time.Second))
}

// GetCompletedTasks returns the current number of completed tasks
func (e *ETACalculator) GetCompletedTasks() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.completedTasks
}

// GetTotalTasks returns the total number of tasks
func (e *ETACalculator) GetTotalTasks() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalTasks
}

// ElapsedTime returns the time elapsed since the beginning of the operation
func (e *ETACalculator) ElapsedTime() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return time.Since(e.startTime)
}