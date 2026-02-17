package eta

import (
	"math"
	"sort"
	"sync"
	"time"
)

const (
	// Sliding window keeps samples from at most this duration
	windowMaxAge = 2 * time.Minute
	// Minimum samples to retain even if they're older than windowMaxAge
	windowMinSamples = 10
	// EMA smoothing factor for rate (adapts in ~3 updates)
	emaAlpha = 0.3
)

// sample is a lightweight (timestamp, progress) pair.
type sample struct {
	timestamp     time.Time
	tasksComplete int64
}

// ETACalculator provides ETA estimation for CPU-bound concurrent tasks
// using a sliding-window rate with conservative selection and EMA smoothing.
type ETACalculator struct {
	startTime      time.Time
	totalTasks     int64
	completedTasks int64
	mu             sync.Mutex // Mutex, not RWMutex: CalculateETAWithConfidence mutates state

	samples      []sample
	smoothedRate float64   // EMA-smoothed effective rate (items/second)
	lastReported ETAResult // last emitted result for smoothing continuity
}

// NewETACalculator creates a new ETACalculator
func NewETACalculator(totalTasks int64) *ETACalculator {
	now := time.Now()
	return &ETACalculator{
		startTime:    now,
		totalTasks:   totalTasks,
		samples:      []sample{{timestamp: now, tasksComplete: 0}},
		smoothedRate: 0,
		lastReported: ETAResult{Estimate: -1, LowerBound: -1, UpperBound: -1},
	}
}

// TaskCompleted records progress. tasksCompleted is an absolute count.
func (e *ETACalculator) TaskCompleted(tasksCompleted int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if tasksCompleted <= e.completedTasks {
		return
	}
	e.completedTasks = tasksCompleted

	now := time.Now()
	e.samples = append(e.samples, sample{timestamp: now, tasksComplete: tasksCompleted})
	e.pruneSamples(now)
}

// pruneSamples enforces the sliding window: keep samples from the last
// windowMaxAge, but always retain at least windowMinSamples entries.
func (e *ETACalculator) pruneSamples(now time.Time) {
	cutoff := now.Add(-windowMaxAge)
	cutIdx := 0
	for i, s := range e.samples {
		if !s.timestamp.Before(cutoff) {
			cutIdx = i
			break
		}
	}
	// Ensure we keep at least windowMinSamples
	if len(e.samples)-cutIdx < windowMinSamples && len(e.samples) > windowMinSamples {
		cutIdx = len(e.samples) - windowMinSamples
	}
	if cutIdx > 0 {
		e.samples = e.samples[cutIdx:]
	}
}

// CalculateETA returns the point estimate of time remaining.
func (e *ETACalculator) CalculateETA() time.Duration {
	return e.CalculateETAWithConfidence().Estimate
}

// CalculateETAWithConfidence returns a detailed ETAResult.
func (e *ETACalculator) CalculateETAWithConfidence() ETAResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	percentDone := float64(0)
	if e.totalTasks > 0 {
		percentDone = float64(e.completedTasks) / float64(e.totalTasks)
	}

	noEstimate := ETAResult{
		Estimate:    -1,
		LowerBound:  -1,
		UpperBound:  -1,
		PercentDone: percentDone,
		Algorithm:   AlgorithmAdvanced,
	}

	if e.completedTasks >= e.totalTasks {
		return ETAResult{
			Estimate:    0,
			LowerBound:  0,
			UpperBound:  0,
			PercentDone: 1.0,
			Algorithm:   AlgorithmAdvanced,
		}
	}

	elapsed := time.Since(e.startTime)
	if len(e.samples) < 2 || e.completedTasks == 0 || elapsed < SimpleETAMinimumElapsed {
		if e.lastReported.Estimate >= 0 {
			return e.lastReported
		}
		return noEstimate
	}

	remaining := float64(e.totalTasks - e.completedTasks)

	// --- 1. Global rate (cross-multiplication anchor) ---
	globalRate := float64(e.completedTasks) / elapsed.Seconds()

	// --- 2. Window rate (head-tail delta smooths bursts automatically) ---
	first := e.samples[0]
	last := e.samples[len(e.samples)-1]
	windowElapsed := last.timestamp.Sub(first.timestamp).Seconds()
	windowTasks := float64(last.tasksComplete - first.tasksComplete)

	windowRate := 0.0
	if windowElapsed > 0.1 { // need at least 100ms of window
		windowRate = windowTasks / windowElapsed
	}

	// --- 3. Conservative rate selection ---
	effectiveRate := globalRate
	if windowRate > 0 {
		if windowRate < globalRate {
			// Slowing down: trust the window immediately
			effectiveRate = windowRate
		} else {
			// Speeding up: be skeptical, blend conservatively
			effectiveRate = (globalRate * 0.7) + (windowRate * 0.3)
		}
	}
	if effectiveRate <= 0 {
		return noEstimate
	}

	// --- 4. EMA smoothing on rate ---
	if e.smoothedRate <= 0 {
		e.smoothedRate = effectiveRate // seed on first real calculation
	} else {
		e.smoothedRate = emaAlpha*effectiveRate + (1-emaAlpha)*e.smoothedRate
	}

	// --- 5. Derive duration from smoothed rate ---
	estimateSec := remaining / e.smoothedRate
	estimate := time.Duration(estimateSec * float64(time.Second))

	// --- 6. Compute variability (CV) from windowed per-pair rates ---
	variability := e.computeVariability()

	// --- 7. Uncertainty cone ---
	// Base: 30% at 0% done, narrows to 5% at 100%
	uncertainty := 0.30 - (percentDone * 0.25)
	if uncertainty < 0.05 {
		uncertainty = 0.05
	}
	// Penalty for few samples (early in run). Scale from +20% at 2 samples
	// to 0% at 20 samples. This is sample-count driven, not wall-clock driven,
	// because the Advanced calculator is used for high-throughput item-bars
	// where many samples arrive in just a few seconds.
	nSamples := len(e.samples)
	if nSamples < 20 {
		uncertainty += 0.20 * (1.0 - float64(nSamples)/20.0)
	}
	// Penalty for high variability
	if variability > 0.3 {
		extra := (variability - 0.3) * 0.5 // up to ~15% extra for CV=0.6
		if extra > 0.15 {
			extra = 0.15
		}
		uncertainty += extra
	}
	// Global cap
	if uncertainty > 0.50 {
		uncertainty = 0.50
	}

	lowerBound := time.Duration(float64(estimate) * (1.0 - uncertainty))
	upperBound := time.Duration(float64(estimate) * (1.0 + uncertainty))

	// Ensure minimum 1-second bounds
	if lowerBound < time.Second && estimate > time.Second {
		lowerBound = time.Second
	}

	// --- 8. Reliability: derived from actual interval width ---
	relWidth := float64(upperBound-lowerBound) / float64(estimate)
	reliability := 1.0 - relWidth
	if reliability < 0.5 {
		reliability = 0.5
	}

	// Count effective rate samples (consecutive pairs in window)
	sampleCount := len(e.samples) - 1
	if sampleCount < 0 {
		sampleCount = 0
	}

	result := ETAResult{
		Estimate:         estimate,
		LowerBound:       lowerBound,
		UpperBound:       upperBound,
		ReliabilityScore: reliability,
		SampleCount:      sampleCount,
		PercentDone:      percentDone,
		Algorithm:        AlgorithmAdvanced,
		Variability:      variability,
		CumulativeRate:   globalRate,
	}

	e.lastReported = result
	return result
}

// computeVariability returns the coefficient of variation (stddev/mean) of
// per-pair rates in the current sample window, with top/bottom 10% trimmed.
func (e *ETACalculator) computeVariability() float64 {
	if len(e.samples) < 3 {
		return 0
	}

	// Collect per-pair rates
	rates := make([]float64, 0, len(e.samples)-1)
	for i := 1; i < len(e.samples); i++ {
		dt := e.samples[i].timestamp.Sub(e.samples[i-1].timestamp).Seconds()
		dTasks := float64(e.samples[i].tasksComplete - e.samples[i-1].tasksComplete)
		if dt > 0.001 && dTasks > 0 { // skip near-zero intervals
			rates = append(rates, dTasks/dt)
		}
	}
	if len(rates) < 3 {
		return 0
	}

	// Trim top/bottom 10%
	sort.Float64s(rates)
	trimCount := len(rates) / 10
	if trimCount < 1 && len(rates) > 4 {
		trimCount = 1
	}
	trimmed := rates[trimCount : len(rates)-trimCount]
	if len(trimmed) < 2 {
		trimmed = rates // fall back to untrimmed if too few
	}

	// Mean
	sum := 0.0
	for _, r := range trimmed {
		sum += r
	}
	mean := sum / float64(len(trimmed))
	if mean <= 0 {
		return 0
	}

	// Stddev
	sumSq := 0.0
	for _, r := range trimmed {
		d := r - mean
		sumSq += d * d
	}
	stddev := math.Sqrt(sumSq / float64(len(trimmed)))

	return stddev / mean
}

// Progress returns the current progress as a percentage (0-100)
func (e *ETACalculator) Progress() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.totalTasks == 0 {
		return 100.0
	}
	return float64(e.completedTasks) / float64(e.totalTasks) * 100.0
}

// GetCompletedTasks returns the current number of completed tasks
func (e *ETACalculator) GetCompletedTasks() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.completedTasks
}

// GetTotalTasks returns the total number of tasks
func (e *ETACalculator) GetTotalTasks() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.totalTasks
}

// ElapsedTime returns the time elapsed since the beginning of the operation
func (e *ETACalculator) ElapsedTime() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return time.Since(e.startTime)
}

// UpdateTotalTasks updates the total task count without resetting rate statistics.
// This preserves all sample history and smoothed rate, since processing rates
// remain valid even when the total changes (e.g. discovering already-done items).
func (e *ETACalculator) UpdateTotalTasks(newTotalTasks int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.totalTasks = newTotalTasks
}

// GetAlgorithmType returns the type of algorithm used by this calculator
func (e *ETACalculator) GetAlgorithmType() AlgorithmType {
	return AlgorithmAdvanced
}
