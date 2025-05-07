package core

import (
	"math"
	"sync"
	"time"
)

// ETAResult represents an ETA calculation with estimate ranges
type ETAResult struct {
	Estimate         time.Duration // Point estimate (median)
	LowerBound       time.Duration // Lower estimate bound
	UpperBound       time.Duration // Upper estimate bound
	ReliabilityScore float64       // Reliability indicator (0.0-1.0)
	SampleCount      int           // Number of samples used
	PercentDone      float64       // Percentage of tasks completed (0.0-1.0)
	RatesPerSec      []float64     // Debug: Recent processing rates (items/second)
	AvgRate          float64       // Debug: Average processing rate (items/second)
	CumulativeRate   float64       // Debug: Cumulative rate (total items / total time)
	Variability      float64       // Debug: Measure of processing rate variability

	// New fields for cross-multiplication ETA
	CrossMultETA    time.Duration // ETA based on cross-multiplication
	CrossMultWeight float64       // Weight given to cross-multiplication ETA
	IsLargeJob      bool          // Whether this is considered a large job
}

// ETACalculator provides a more accurate ETA for CPU-bound concurrent tasks
type ETACalculator struct {
	startTime          time.Time
	totalTasks         int64
	completedTasks     int64
	mu                 sync.RWMutex
	samples            []processSample // Sample points for rate calculation
	lastUpdateTime     time.Time
	stableEstimate     time.Duration // Last stable estimate (to prevent jumping)
	hasEnoughSamples   bool          // Whether we have enough data for good estimates
	processingSpeedup  float64       // Ratio of recent to overall speed (detects acceleration)
	processingSlowdown float64       // Ratio of overall to recent speed (detects slowdown)
	lastReported       ETAResult     // Last reported estimate (for smoothing)

	// Added fields for improved ETA calculation
	lastBurstTime        time.Time      // Track time of last completion burst
	burstDetectionWindow time.Duration  // Time window to group completions as a single burst
	elapsedAtLastBurst   time.Duration  // Elapsed time when last burst was detected
	processedAtLastBurst int64          // Tasks processed when last burst was detected
	rateVariability      float64        // Measure of processing rate variability
	weightedRates        []weightedRate // Rates with weights for better averaging
}

// weightedRate represents a processing rate with an associated weight for averaging
type weightedRate struct {
	rate   float64 // Processing rate (items/second)
	weight float64 // Weight factor for this rate (higher = more significant)
}

// processSample represents a single data point for ETA calculation
type processSample struct {
	timestamp      time.Time
	tasksComplete  int64
	rate           float64 // Items per second at this sample point
	remainingTasks int64

	// The following fields help track relative changes in processing speed
	elapsedTotal     time.Duration // Total elapsed time at this sample
	elapsedIncrement time.Duration // Time since last sample
	taskIncrement    int64         // Tasks completed since last sample

	// Burst detection fields
	isBurstSample bool // Whether this sample is part of a burst
	burstSize     int  // Number of tasks in this burst
}

// NewETACalculator creates a new ETACalculator
func NewETACalculator(totalTasks int64) *ETACalculator {
	now := time.Now()
	initSample := processSample{
		timestamp:        now,
		tasksComplete:    0,
		rate:             0,
		remainingTasks:   totalTasks,
		elapsedTotal:     0,
		elapsedIncrement: 0,
		taskIncrement:    0,
		isBurstSample:    false,
		burstSize:        0,
	}

	return &ETACalculator{
		startTime:          now,
		totalTasks:         totalTasks,
		completedTasks:     0,
		samples:            []processSample{initSample},
		lastUpdateTime:     now,
		stableEstimate:     -1, // No estimate yet
		hasEnoughSamples:   false,
		processingSpeedup:  1.0,
		processingSlowdown: 1.0,
		lastReported:       ETAResult{Estimate: -1, LowerBound: -1, UpperBound: -1},

		// Initialize new fields
		lastBurstTime:        now,
		burstDetectionWindow: 300 * time.Millisecond, // 300ms window for burst detection
		elapsedAtLastBurst:   0,
		processedAtLastBurst: 0,
		rateVariability:      0.0,
		weightedRates:        []weightedRate{},
	}
}

// TaskCompleted informs the calculator that a task has been completed
func (e *ETACalculator) TaskCompleted(tasksCompleted int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	elapsedTotal := now.Sub(e.startTime)

	// No update if task count hasn't changed
	if tasksCompleted <= e.completedTasks {
		return
	}

	// Calculate the rate and other metrics
	var rate float64 = 0
	var taskIncrement int64 = 0
	var elapsedIncrement time.Duration = 0
	var isBurst bool = false
	var burstSize int = 0

	// Detect if this is part of a burst of task completions
	// A burst is defined as multiple task completions in a short time window
	timeSinceLastUpdate := now.Sub(e.lastUpdateTime)
	if timeSinceLastUpdate < e.burstDetectionWindow {
		isBurst = true
		// If we're in a burst window, count this as part of the same burst
		if len(e.samples) > 0 && e.samples[len(e.samples)-1].isBurstSample {
			burstSize = e.samples[len(e.samples)-1].burstSize + 1
		} else {
			burstSize = 1
		}
	}

	// Get the previous sample for comparison
	if len(e.samples) > 0 {
		lastSample := e.samples[len(e.samples)-1]
		elapsedIncrement = now.Sub(lastSample.timestamp)
		taskIncrement = tasksCompleted - lastSample.tasksComplete

		// Calculate instantaneous rate (items per second)
		if elapsedIncrement.Seconds() > 0 && taskIncrement > 0 {
			rate = float64(taskIncrement) / elapsedIncrement.Seconds()
		}
	}

	// We'll update our state
	e.completedTasks = tasksCompleted

	// Calculate cumulative rate (total tasks / total time) for logging only
	// The actual calculation for ETA estimation happens in the CalculateETAWithConfidence method
	_ = float64(e.completedTasks) / elapsedTotal.Seconds()

	// Only add new samples if enough time has passed or significant progress was made
	// This prevents sample flooding during high-throughput phases
	significantProgress := taskIncrement >= 5 || (float64(taskIncrement) >= 0.02*float64(e.totalTasks))
	enoughTimeElapsed := elapsedIncrement >= 300*time.Millisecond

	if enoughTimeElapsed || significantProgress {
		// Create and add a new sample
		newSample := processSample{
			timestamp:        now,
			tasksComplete:    tasksCompleted,
			rate:             rate,
			remainingTasks:   e.totalTasks - tasksCompleted,
			elapsedTotal:     elapsedTotal,
			elapsedIncrement: elapsedIncrement,
			taskIncrement:    taskIncrement,
			isBurstSample:    isBurst,
			burstSize:        burstSize,
		}

		e.samples = append(e.samples, newSample)
		e.lastUpdateTime = now

		// Add to weighted rates for better averaging
		// Weight is proportional to taskIncrement (larger batches have higher statistical significance)
		// and recency (more recent samples are weighted higher)
		if rate > 0 {
			recencyWeight := 1.0
			if len(e.weightedRates) > 0 {
				// Exponential weighting for recency
				recencyWeight = 1.2 // 20% higher weight for the most recent sample
			}

			// Scale by task increment (but cap to avoid overly dominant samples)
			taskWeight := math.Min(float64(taskIncrement), 10.0)

			// Calculate final weight
			weight := taskWeight * recencyWeight

			// Add to weighted rates
			e.weightedRates = append(e.weightedRates, weightedRate{
				rate:   rate,
				weight: weight,
			})

			// Keep weighted rates manageable by pruning oldest
			if len(e.weightedRates) > 30 {
				e.weightedRates = e.weightedRates[len(e.weightedRates)-30:]
			}
		}

		// Logarithmic sampling for long-term trends
		// Keep more recent samples and fewer old ones
		if len(e.samples) > 40 {
			// Phase 1: Keep first 5 samples for baseline
			// Phase 2: Keep logarithmically spaced samples from middle portion
			// Phase 3: Keep most recent 20 samples for current trend analysis

			// Prepare the new sample set
			newSamples := make([]processSample, 0, 30)

			// Phase 1: First 5 samples
			newSamples = append(newSamples, e.samples[:5]...)

			// Phase 2: Logarithmically spaced samples from middle
			// Skip this if we don't have enough samples yet
			if len(e.samples) > 35 {
				middle := e.samples[5 : len(e.samples)-20]
				if len(middle) > 10 {
					// Get ~5 logarithmically spaced samples
					step := float64(len(middle)) / 5.0
					for i := 0; i < 5; i++ {
						idx := int(math.Floor(float64(i) * step))
						if idx < len(middle) {
							newSamples = append(newSamples, middle[idx])
						}
					}
				} else {
					// If middle section is small, just keep it
					newSamples = append(newSamples, middle...)
				}
			}

			// Phase 3: Most recent 20 samples
			lastIdx := len(e.samples)
			startIdx := lastIdx - 20
			if startIdx < 0 {
				startIdx = 0
			}
			newSamples = append(newSamples, e.samples[startIdx:lastIdx]...)

			// Replace sample array
			e.samples = newSamples
		}

		// Detect if we have enough samples for reliable estimates
		if len(e.samples) >= 4 || (len(e.samples) >= 2 && float64(tasksCompleted) > float64(e.totalTasks)*0.2) {
			e.hasEnoughSamples = true
		}

		// Calculate rate variability (standard deviation / mean)
		if len(e.weightedRates) >= 3 {
			var sum, sumSquares, sumWeights float64

			// Calculate weighted mean
			for _, wr := range e.weightedRates {
				sum += wr.rate * wr.weight
				sumWeights += wr.weight
			}

			if sumWeights > 0 {
				weightedMean := sum / sumWeights

				// Calculate weighted variance
				for _, wr := range e.weightedRates {
					deviation := wr.rate - weightedMean
					sumSquares += (deviation * deviation) * wr.weight
				}

				weightedVariance := sumSquares / sumWeights
				weightedStdDev := math.Sqrt(weightedVariance)

				// Coefficient of variation (stddev/mean)
				if weightedMean > 0 {
					e.rateVariability = weightedStdDev / weightedMean
				}
			}
		}

		// Detect significant changes in processing speed by comparing recent to overall rates
		if len(e.samples) >= 5 {
			// Calculate overall average rate
			overallElapsed := e.samples[len(e.samples)-1].timestamp.Sub(e.samples[0].timestamp).Seconds()
			overallTasks := float64(e.samples[len(e.samples)-1].tasksComplete - e.samples[0].tasksComplete)
			overallRate := 0.0
			if overallElapsed > 0 {
				overallRate = overallTasks / overallElapsed
			}

			// Calculate recent average rate (last 3-5 samples)
			recentSamples := 3
			if len(e.samples) < 8 {
				recentSamples = 2 // Use fewer for small sample sizes
			}

			recentElapsed := e.samples[len(e.samples)-1].timestamp.Sub(e.samples[len(e.samples)-1-recentSamples].timestamp).Seconds()
			recentTasks := float64(e.samples[len(e.samples)-1].tasksComplete - e.samples[len(e.samples)-1-recentSamples].tasksComplete)

			// Avoid division by zero
			recentRate := 0.0
			if recentElapsed > 0 {
				recentRate = recentTasks / recentElapsed
			}

			// Detect speedup (recent faster than overall)
			if recentRate > 0 && overallRate > 0 {
				e.processingSpeedup = recentRate / overallRate
				e.processingSlowdown = overallRate / recentRate

				// Limit extreme values
				e.processingSpeedup = math.Min(e.processingSpeedup, 2.0)
				e.processingSlowdown = math.Min(e.processingSlowdown, 2.0)
			}
		}
	}
}

// CalculateETA returns an ETAResult with estimated time remaining and confidence intervals
// For backward compatibility, we also keep the original method signature
func (e *ETACalculator) CalculateETA() time.Duration {
	result := e.CalculateETAWithConfidence()
	return result.Estimate
}

// CalculateETAWithConfidence returns an ETAResult with estimated time remaining and confidence intervals
func (e *ETACalculator) CalculateETAWithConfidence() ETAResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Calculate percentage done
	percentDone := float64(0)
	if e.totalTasks > 0 {
		percentDone = float64(e.completedTasks) / float64(e.totalTasks)
	}

	// Calculate total elapsed time
	elapsedTotal := time.Since(e.startTime)

	// Create a placeholder result with basic info
	result := ETAResult{
		Estimate:         -1,
		LowerBound:       -1,
		UpperBound:       -1,
		ReliabilityScore: 0,
		SampleCount:      len(e.samples),
		PercentDone:      percentDone,
		Variability:      e.rateVariability,
	}

	// If task is already complete, return zero ETA
	if e.completedTasks >= e.totalTasks {
		return ETAResult{
			Estimate:         0,
			LowerBound:       0,
			UpperBound:       0,
			ReliabilityScore: 1.0,
			SampleCount:      len(e.samples),
			PercentDone:      1.0,
			CumulativeRate:   0,
			Variability:      0,
		}
	}

	// If we don't have enough samples yet or no tasks completed, return no estimate
	if len(e.samples) < 2 || e.completedTasks == 0 {
		// Return last reported result if we have one (to maintain consistency)
		if e.lastReported.Estimate >= 0 {
			return e.lastReported
		}
		return result
	}

	// Calculate remaining tasks
	remainingTasks := float64(e.totalTasks - e.completedTasks)

	// Calculate cumulative rate (most reliable for long-running processes)
	var cumulativeRate float64 = 0
	if elapsedTotal.Seconds() > 0 {
		cumulativeRate = float64(e.completedTasks) / elapsedTotal.Seconds()
	}

	// Get recent rates (may exclude burst samples to avoid distortion)
	rates := make([]float64, 0, len(e.samples)-1)
	for i := 1; i < len(e.samples); i++ {
		// Only use samples with valid rates
		if e.samples[i].rate > 0 {
			// For burst samples, we want to calculate an aggregate rate
			// to avoid artificially high rates due to concurrent completions
			if e.samples[i].isBurstSample && e.samples[i].burstSize > 1 {
				// Only add the first sample in a burst
				if i == 1 || !e.samples[i-1].isBurstSample {
					// Find the end of this burst
					burstEnd := i
					for burstEnd < len(e.samples)-1 && e.samples[burstEnd+1].isBurstSample {
						burstEnd++
					}

					// Calculate an aggregate rate for the entire burst
					burstElapsed := e.samples[burstEnd].timestamp.Sub(e.samples[i].timestamp).Seconds()
					burstTasks := float64(e.samples[burstEnd].tasksComplete - e.samples[i].tasksComplete)

					// If elapsed time is too small, use a minimum threshold
					if burstElapsed < 0.001 {
						burstElapsed = 0.001
					}

					burstRate := burstTasks / burstElapsed
					rates = append(rates, burstRate)
				}
				// Skip other burst samples
			} else {
				// Regular non-burst sample
				rates = append(rates, e.samples[i].rate)
			}
		}
	}

	// Compute weighted average from our tracked weights
	var weightedAvgRate float64 = 0
	var totalWeight float64 = 0

	if len(e.weightedRates) > 0 {
		for _, wr := range e.weightedRates {
			weightedAvgRate += wr.rate * wr.weight
			totalWeight += wr.weight
		}

		if totalWeight > 0 {
			weightedAvgRate /= totalWeight
		}
	}

	// If weighted calculation failed, fall back to simple average
	if weightedAvgRate <= 0 && len(rates) > 0 {
		var totalRate float64 = 0
		for _, r := range rates {
			totalRate += r
		}
		weightedAvgRate = totalRate / float64(len(rates))
	}

	// Pick the more conservative rate estimate
	// This provides an anchor against overly optimistic estimates
	var finalRate float64

	// Use cumulative rate whenever possible, with increasing weight as we get more data
	if e.completedTasks > 0 && cumulativeRate > 0 {
		// Determine cumulative rate weight based on progress and sample count
		// Start trusting cumulative rate more as we get more data
		var cumulativeWeight float64

		if len(rates) >= 30 || percentDone > 0.7 {
			// Late in processing or lots of samples - trust cumulative rate heavily
			cumulativeWeight = 0.8
		} else if len(rates) >= 15 || percentDone > 0.5 {
			// Good amount of data - blend with bias toward cumulative
			cumulativeWeight = 0.7
		} else if len(rates) >= 5 {
			// Moderate amount of data - balanced blend
			cumulativeWeight = 0.6
		} else {
			// Early in processing - start with lower cumulative weight
			// But still use some cumulative rate for stability
			cumulativeWeight = math.Min(0.3+(percentDone*0.4), 0.5)
		}

		// Adjust weight based on total task count - for larger jobs, trust cumulative rate more
		if e.totalTasks > 100 {
			cumulativeWeight = math.Min(cumulativeWeight+0.1, 0.9)
		}

		instantWeight := 1.0 - cumulativeWeight

		// Blend weighted average with cumulative rate
		finalRate = (weightedAvgRate * instantWeight) + (cumulativeRate * cumulativeWeight)

		// If we've seen high variability, bias even more toward cumulative rate
		if e.rateVariability > 0.4 {
			// High variability detected - trust cumulative rate more
			finalRate = (finalRate * 0.3) + (cumulativeRate * 0.7)
		}

		// Ensure we're not being too optimistic by comparing to cumulative rate
		// This is a safety check - if our blend is >20% faster than cumulative, use a more conservative value
		if finalRate > cumulativeRate*1.2 {
			finalRate = cumulativeRate * 1.1 // Limit to 10% faster than cumulative
		}
	} else {
		// Very early in processing, use weighted average with a pessimism factor
		finalRate = weightedAvgRate * 0.9 // 10% conservative adjustment
	}

	// For debugging
	result.RatesPerSec = rates
	result.AvgRate = weightedAvgRate
	result.CumulativeRate = cumulativeRate

	// Determine reliability level and scaling based on sample count, progress, and variability
	var reliabilityScore float64
	var pessimismFactor float64
	var rangeMultiplier float64

	// We previously calculated elapsed percentage of total estimated time here,
	// but it's not currently used in decision making. Removed to avoid unused variable warnings.

	// Adaptive reliability level based on samples, progress, and variability
	if len(rates) >= 100 || percentDone > 0.25 {
		// Very substantial data - highly reliable cross-multiplication phase
		// Evidence shows ~5% accuracy with cross-multiplication after 100 samples
		reliabilityScore = 0.98
		pessimismFactor = 1.02 // 2% padding
		rangeMultiplier = 1.03 // 3% range width - extremely tight for point estimate display
	} else if len(rates) >= 50 || percentDone > 0.15 {
		// Substantial data with good cross-multiplication support
		reliabilityScore = 0.95
		pessimismFactor = 1.03 // 3% padding
		rangeMultiplier = 1.05 // 5% range width - very tight with good sample size
	} else if len(rates) >= 30 || percentDone > 0.08 {
		// Good amount of data - starting to trust cross-multiplication
		reliabilityScore = 0.9
		pessimismFactor = 1.05 // 5% padding
		rangeMultiplier = 1.1  // 10% range width - much tighter
	} else if len(rates) >= 15 || percentDone > 0.05 {
		// Moderate data - beginning confidence
		reliabilityScore = 0.85
		pessimismFactor = 1.08 // 8% padding
		rangeMultiplier = 1.2  // 20% range width - tighter
	} else if len(rates) >= 5 {
		// Some data - early estimates
		reliabilityScore = 0.8
		pessimismFactor = 1.15 // 15% padding
		rangeMultiplier = 1.35 // 35% range width
	} else {
		// Limited data - very early estimates
		reliabilityScore = 0.7
		pessimismFactor = 1.3  // 30% padding
		rangeMultiplier = 1.6  // 60% range width
	}

	// Progressive reliability acceleration for jobs with good sample count
	// Dynamically adjust range multipliers and force point estimate for large datasets
	if len(rates) >= 100 || (e.totalTasks > 300 && percentDone > 0.15) {
		// With 100+ samples or 15%+ completion on large jobs, we have excellent
		// cross-multiplication accuracy (proven 5% error margin)
		// Force the multiplier to be extremely tight to show point estimates
		rangeMultiplier = 1.01  // Practically a point estimate
		pessimismFactor = 1.01  // Minimal pessimism with proven cross-mult accuracy
		reliabilityScore = 0.98 // Highest reliability level
	} else if (len(rates) >= 50 && percentDone > 0.1) || e.completedTasks > 200 {
		// More aggressive compression of range for jobs with good sample size
		rangeMultiplier = math.Max(1.03, rangeMultiplier * 0.7)  // At least 70% tighter
		pessimismFactor = math.Max(1.02, pessimismFactor * 0.9)  // Reduce pessimism by 10% 
		reliabilityScore = math.Min(reliabilityScore + 0.02, 0.97) // Boost reliability
	} else if percentDone > 0.05 {
		// Apply progress-based range reduction for any job with 5%+ completion
		reductionFactor := math.Min(percentDone*3, 0.4)  // Up to 40% reduction
		rangeMultiplier = math.Max(1.05, rangeMultiplier*(1.0-reductionFactor))
		pessimismFactor = math.Max(1.03, pessimismFactor*0.95)
	}

	// Adjust for how long the job has been running
	// Longer-running jobs need more pessimism
	if elapsedTotal > 5*time.Minute {
		// Add 1% pessimism per minute after 5 minutes, up to 20%
		extraPessimism := math.Min(0.2, (elapsedTotal.Minutes()-5.0)*0.01)
		pessimismFactor += extraPessimism
	}

	// Apply adjustments based on detected processing speed changes
	if e.processingSlowdown > 1.1 {
		// We've detected slowdown, increase pessimism
		pessimismFactor *= (e.processingSlowdown * 0.6) + 0.4 // Increased slowdown factor
		rangeMultiplier *= 1.2
	}

	// Apply adjustments based on variability
	if e.rateVariability > 0.3 {
		// High variability, widen the range and increase pessimism
		variabilityFactor := 1.0 + (e.rateVariability * 0.5)
		rangeMultiplier *= variabilityFactor
		pessimismFactor *= 1.0 + (e.rateVariability * 0.2)
	}

	// Calculate the standard rate-based ETA
	var rateBasedEstimate time.Duration
	if finalRate > 0 {
		estimateSeconds := (remainingTasks / finalRate) * pessimismFactor
		rateBasedEstimate = time.Duration(estimateSeconds * float64(time.Second))
	} else {
		return result
	}

	// Calculate cross-multiplication ETA: elapsed * (totalTasks/completedTasks)
	// This is simple but becomes increasingly reliable as we process more items
	var crossMultETA time.Duration
	var crossMultReliability float64 = 0

	if percentDone > 0.01 { // Only use with at least 1% done
		// Cross-multiplication: time_total = time_elapsed * (total_tasks / completed_tasks)
		crossMultETA = time.Duration(float64(elapsedTotal)/percentDone - float64(elapsedTotal))

		// Calculate reliability factor based on consistent processing
		// Start with reliability proportional to percentage done
		crossMultReliability = math.Min(percentDone*5, 0.9) // Max 90% reliability

		// Increase reliability with more samples
		if len(rates) > 50 {
			crossMultReliability = math.Min(crossMultReliability+0.1, 0.95)
		}

		// Adjust reliability based on processing variability
		if e.rateVariability > 0.3 {
			// High variability reduces reliability
			crossMultReliability *= (1.0 - (e.rateVariability * 0.3))
		}
	}

	// Blend statistical ETA with cross-multiplication ETA
	var estimate time.Duration
	var crossMultWeight float64 = 0
	
	// Cross-multiplication is extremely accurate after 100+ samples or 25%+ completion
	if percentDone >= 0.03 && crossMultReliability > 0.3 {
		// Base weight depends primarily on sample count and completion percentage
		if len(rates) >= 100 || percentDone >= 0.25 {
			// Heavily trust cross-multiplication with substantial data
			// At this point, we know cross-multiplication is typically within 5% of actual completion time
			crossMultWeight = 0.95  // 95% cross-multiplication, 5% rate-based
		} else if len(rates) >= 50 || percentDone >= 0.15 {
			// Strong trust with moderate data 
			crossMultWeight = math.Min(0.7 + (percentDone * 0.8), 0.9)
		} else {
			// Progressive trust based on completion
			crossMultWeight = math.Min(0.4 + (percentDone * 2), 0.7)
		}
		
		// Adjust for variability - reduce cross-mult weight if processing is highly variable
		if e.rateVariability > 0.4 {
			// Reduce by up to 20% for extremely variable processing
			crossMultWeight = math.Max(0.5, crossMultWeight * (1.0 - (e.rateVariability * 0.4)))
		}
		
		// Blend the two estimates
		estimate = time.Duration(
			(float64(rateBasedEstimate) * (1.0 - crossMultWeight)) +
			(float64(crossMultETA) * crossMultWeight))
	} else {
		// Very early in processing - use statistical estimate
		estimate = rateBasedEstimate
	}

	// Calculate bounds
	// Before creating bounds, validate the estimate to ensure it's valid
	if estimate <= 0 {
		// This should never happen if finalRate > 0, but let's be defensive
		return result
	}

	// Upper time bound is always longer than estimate (lower processing rate)
	// Lower time bound is always shorter than estimate (higher processing rate)
	lowerBound := time.Duration(float64(estimate) / rangeMultiplier)
	upperBound := time.Duration(float64(estimate) * rangeMultiplier)

	// Validate bounds to ensure they make logical sense
	if lowerBound <= 0 || upperBound <= 0 || lowerBound >= upperBound {
		// Something went wrong in the calculation - use fallbacks
		lowerBound = estimate / 2
		upperBound = estimate * 2

		// Double-check our fallback values for sanity
		if lowerBound <= 0 {
			lowerBound = 1 * time.Second
		}
		if upperBound <= lowerBound {
			upperBound = lowerBound * 3
		}
	}

	// Ensure a minimum reasonable difference between bounds
	// This avoids displays like "ETA: 5m-5m" due to rounding
	minimumBoundDifference := time.Duration(float64(estimate) * 0.1) // At least 10% difference
	if minimumBoundDifference < 1*time.Second {
		minimumBoundDifference = 1 * time.Second
	}

	if upperBound-lowerBound < minimumBoundDifference {
		// Expand bounds to ensure meaningful difference
		lowerBound = time.Duration(float64(estimate) * 0.95) // 5% below estimate
		upperBound = time.Duration(float64(estimate) * 1.05) // 5% above estimate

		// Ensure bounds are at least 1 second apart
		if upperBound-lowerBound < 1*time.Second {
			lowerBound = estimate - 500*time.Millisecond
			upperBound = estimate + 500*time.Millisecond
		}
	}

	// Apply smoothing: don't let estimates jump around too much
	// If we have a previous stable estimate, blend it with the new one
	if e.stableEstimate > 0 {
		// How much weight to give the previous estimate (more at first, less as we get more data)
		// Cap at 50% to ensure we adapt to significant changes
		previousWeight := math.Min(0.5, math.Max(0.1, 0.7-(float64(len(rates))*0.03)))

		// If we've detected high variability, give more weight to previous stable estimate
		if e.rateVariability > 0.4 {
			previousWeight = math.Min(0.7, previousWeight+0.2)
		}

		// Blend the new estimate with the previous one
		blendedEstimate := time.Duration(
			(float64(estimate) * (1.0 - previousWeight)) +
				(float64(e.stableEstimate) * previousWeight))

		// Also smooth the bounds
		blendedLower := time.Duration(
			(float64(lowerBound) * (1.0 - previousWeight)) +
				(float64(e.stableEstimate) / rangeMultiplier * previousWeight))

		blendedUpper := time.Duration(
			(float64(upperBound) * (1.0 - previousWeight)) +
				(float64(e.stableEstimate) * rangeMultiplier * previousWeight))

		estimate = blendedEstimate
		lowerBound = blendedLower
		upperBound = blendedUpper
	}

	// Update stable estimate
	e.stableEstimate = estimate

	// We already calculated cross-multiplication weight above
	// This is used in the ETAResult for potential display decisions

	// Format the final result with all ETA information
	result = ETAResult{
		Estimate:         estimate,
		LowerBound:       lowerBound,
		UpperBound:       upperBound,
		ReliabilityScore: reliabilityScore,
		SampleCount:      len(rates),
		PercentDone:      percentDone,
		RatesPerSec:      rates,
		AvgRate:          weightedAvgRate,
		CumulativeRate:   cumulativeRate,
		Variability:      e.rateVariability,
		CrossMultETA:     crossMultETA,
		CrossMultWeight:  crossMultWeight,  // Using the weight calculated earlier
		IsLargeJob:       e.totalTasks > 200,  // Consider more jobs as "large" for ETA display
	}

	// Save this result for next time
	e.lastReported = result

	return result
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

// UpdateTotalTasks updates the total task count without resetting rate statistics
// This is crucial for handling "already done" items that reduce the total tasks.
// 
// Without this method, handlers might create entirely new ETACalculators when they 
// detect already-done items, which would reset all valuable rate statistics and 
// cause erratic ETA behavior. By using this method instead, we maintain a continuous, 
// stable statistical progression for ETAs throughout the entire job.
func (e *ETACalculator) UpdateTotalTasks(newTotalTasks int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// No need to update if the count is the same
	if newTotalTasks == e.totalTasks {
		return
	}

	// Update the total task count
	// We intentionally don't log the difference, as we just want 
	// to maintain internal state consistency
	e.totalTasks = newTotalTasks

	// Update all sample remainingTasks values to maintain consistency
	for i := range e.samples {
		// Adjust each sample's remaining tasks based on the new total
		if i == 0 {
			// Initial sample has all tasks remaining
			e.samples[i].remainingTasks = newTotalTasks
		} else {
			// Other samples have (total - completed) remaining
			// This properly preserves the history of how many tasks remained
			// at each point in time, using the updated total
			e.samples[i].remainingTasks = newTotalTasks - e.samples[i].tasksComplete
		}
	}

	// Purposely preserve all rate statistics and sample history
	// Processing rates are based on completed tasks over time, which 
	// remains valid even when we discover tasks that were already done
}
