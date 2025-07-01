package core

import (
	"fmt"
	"os"
	"io"
	"time"
	"context"
	"bytes"
	"encoding/json"
	"math"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"

	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/version"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/eta"
	"github.com/tassa-yoniso-manasi-karoto/translitkit/common"
)


type MessageHandler interface {
	IsCLI() bool

	// TODO log methods don't actually require an interface:
	// could use a Handler.Std() method to access Log* and thus regroup the Log* funcs under Std()
	Log(level int8, behavior string, msg string) *ProcessingError
	// this is a helper that logs to LevelError by default as most err log correspond to LevelError
	LogErr(err error, behavior string, msg string) *ProcessingError
	// this is a helper that returns an err but doesn't use a LevelError,
	// it is meant to be used to handle ctx.Err following user-requested context cancelation
	LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError

	LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError
	LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError

	ZeroLog() *zerolog.Logger
	GetLogBuffer() bytes.Buffer
	HandleStatus(status string) //TODO

	// Progress tracking methods with specific ETA algorithm choice
	IncrementProgress(taskID string, increment, total, priority int, operation, descr, size string) // Defaults to Simple ETA
	IncrementProgressAdvanced(taskID string, increment, total, priority int, operation, descr, size string) // Uses Advanced ETA
	ResetProgress()
	RemoveProgressBar(taskID string)

	// SetHighLoadMode enables high performance processing for intensive operations
	// Optional duration parameter - defaults to 5 seconds if not provided
	SetHighLoadMode(durations ...time.Duration)

	// GetContext returns the context for operations like crash reporting
	GetContext() context.Context
}

// #############################################################################
// #############################################################################
// #############################################################################


// CLI implementation
type CLIHandler struct {
	ctx	context.Context
	logger *zerolog.Logger
	buffer bytes.Buffer

	progressBars map[string]*progressbar.ProgressBar
	progressValues map[string]int // Track absolute progress values
	etaCalculators map[string]eta.Provider
}

func NewCLIHandler(ctx context.Context) *CLIHandler {
	h := &CLIHandler{
		ctx: ctx,
		progressBars: make(map[string]*progressbar.ProgressBar),
		progressValues: make(map[string]int),
		etaCalculators: make(map[string]eta.Provider),
	}
	crash.InitReporter(ctx)
	
	multiOut := io.MultiWriter(os.Stderr, &h.buffer)
	
	writer := zerolog.ConsoleWriter{
		Out: multiOut,
		TimeFormat: time.TimeOnly,
	}
	logger := zerolog.New(writer).With().Timestamp().Logger()
	h.logger = &logger
	common.Log = logger.With().Timestamp().Str("module", "translitkit").Logger()
	return h
}


func (h *CLIHandler) IsCLI() bool {
	return true
}

func (h *CLIHandler) GetLogBuffer() bytes.Buffer {
	return h.buffer
}


func (h *CLIHandler) Log(level int8, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, nil)
}

func (h *CLIHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	return log(h, int8(zerolog.ErrorLevel), err, behavior, msg, nil)
}

func (h *CLIHandler) LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), err, behavior, msg, nil)
}



func (h *CLIHandler) LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, fields)
}

func (h *CLIHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, int8(zerolog.ErrorLevel), err, behavior, msg, fields)
}



func (h *CLIHandler) ZeroLog() *zerolog.Logger {
	return h.logger
}


func (h *CLIHandler) HandleStatus(status string) {
	h.logger.Info().Msg(status)
}


func (h *CLIHandler) ResetProgress() {
	// Clear progress bars
	for id, bar := range h.progressBars {
		bar.Clear()
		delete(h.progressBars, id)
	}

	// Reset progress tracking and ETA calculators
	h.progressValues = make(map[string]int)
	h.etaCalculators = make(map[string]eta.Provider)
}


// RemoveProgressBar explicitly removes a specific progress bar by ID
func (h *CLIHandler) RemoveProgressBar(taskID string) {
	if h.progressBars == nil {
		return
	}

	// If the bar exists in CLI, clear and remove it
	if bar, exists := h.progressBars[taskID]; exists {
		bar.Clear()
		delete(h.progressBars, taskID)

		h.logger.Debug().
			Str("taskID", taskID).
			Msg("Removed progress bar")
	}

	// Also remove any progress tracking and ETA calculator for this task
	if h.progressValues != nil {
		delete(h.progressValues, taskID)
	}

	if h.etaCalculators != nil {
		delete(h.etaCalculators, taskID)
	}
}


// incrementProgressInternal handles progress bar updates with specific ETA algorithm
func (h *CLIHandler) incrementProgressInternal(
	taskID string,
	increment, total, priority int,
	operation, desc, size string,
	algoType eta.AlgorithmType,
) {
	if h.progressBars == nil {
		h.progressBars = make(map[string]*progressbar.ProgressBar)
	}

	if h.progressValues == nil {
		h.progressValues = make(map[string]int)
	}

	if h.etaCalculators == nil {
		h.etaCalculators = make(map[string]eta.Provider)
	}

	// Update absolute progress tracking
	h.progressValues[taskID] += increment
	current := h.progressValues[taskID]

	// Get or create ETA calculator for this task
	var provider eta.Provider
	isEtaEnabled := true // ETA is always potentially enabled if this internal method is called

	if isEtaEnabled {
		provider = h.etaCalculators[taskID]
		if provider == nil && total > 0 {
			// Create the appropriate ETA calculator based on the algorithm type
			if algoType == eta.AlgorithmAdvanced {
				provider = eta.NewETACalculator(int64(total))
			} else { // Default to Simple if algoType == eta.AlgorithmSimple
				provider = eta.NewSimpleETACalculator(int64(total))
			}
			h.etaCalculators[taskID] = provider
		} else if provider != nil && provider.GetAlgorithmType() != algoType && total > 0 {
			// Algorithm type mismatch for existing calculator, recreate
			// This handles cases where a taskID might be reused with a different ETA requirement
			h.ZeroLog().Warn().
				Str("taskID", taskID).
				Str("existingAlgo", provider.GetAlgorithmType().String()).
				Str("requestedAlgo", algoType.String()).
				Msg("ETA algorithm type mismatch for task, recreating calculator.")

			if algoType == eta.AlgorithmAdvanced {
				provider = eta.NewETACalculator(int64(total))
			} else {
				provider = eta.NewSimpleETACalculator(int64(total))
			}
			h.etaCalculators[taskID] = provider
		}
	}

	bar, exists := h.progressBars[taskID]
	if !exists {
		// Create a new progress bar for this ID
		options := []progressbar.Option{
			progressbar.OptionSetDescription(desc),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(31),
			progressbar.OptionClearOnFinish(),
			progressbar.OptionSetWriter(os.Stdout),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "#",
				SaucerPadding: "-",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		}

		// Only enable built-in ETA prediction if we're not using our custom ETA
		if !isEtaEnabled {
			options = append(options, progressbar.OptionSetPredictTime(true))
		}

		bar = progressbar.NewOptions(total, options...)
		h.progressBars[taskID] = bar
		fmt.Printf("\n%s\n", operation) // Show an extra line with the name of the operation if you like
	}

	// If the total changed, adjust the bar's max and update ETA calculator
	if bar.GetMax() != total {
		bar.ChangeMax(total)
		bar.Describe(desc) // update text if you want

		if isEtaEnabled && total > 0 {
			if provider != nil {
				// Update existing calculator (preserves rate history)
				provider.UpdateTotalTasks(int64(total))
			} else {
				// If no calculator exists yet, create a new one
				if algoType == eta.AlgorithmAdvanced {
					provider = eta.NewETACalculator(int64(total))
				} else {
					provider = eta.NewSimpleETACalculator(int64(total))
				}
				h.etaCalculators[taskID] = provider
			}
		}
	}

	// Update ETA calculator with the absolute progress
	if isEtaEnabled && provider != nil {
		// Send the absolute progress to the ETA calculator
		// This allows for proper handling of resumption
		provider.TaskCompleted(int64(current))

		// Calculate the ETA with confidence intervals
		etaResult := provider.CalculateETAWithConfidence()
		if etaResult.Estimate >= 0 {
			// Conditional ETA display logic based on algorithm type
			if etaResult.Algorithm == eta.AlgorithmAdvanced ||
			   increment > 0 ||
			   provider.ElapsedTime() >= eta.SimpleETAMinimumElapsed {
				// Format the ETA with confidence information
				etaStr := formatETAWithConfidence(etaResult)

				// Update the progress bar description with the ETA
				bar.Describe(fmt.Sprintf("%s [%s]", desc, etaStr))
			}
		}
	}

	// Increment by the specified amount
	bar.Add(increment)

	// If we're done, clear & remove from map so we don't keep unused bars
	if bar.State().CurrentPercent >= 1.0 {
		bar.Clear()
		delete(h.progressBars, taskID)
		delete(h.progressValues, taskID)
		delete(h.etaCalculators, taskID)
	}
}

// IncrementProgress updates progress with simple ETA calculation
func (h *CLIHandler) IncrementProgress(taskID string, increment, total, priority int, operation, desc, size string) {
	h.incrementProgressInternal(taskID, increment, total, priority, operation, desc, size, eta.AlgorithmSimple)
}

// IncrementProgressAdvanced updates progress with advanced ETA calculation
func (h *CLIHandler) IncrementProgressAdvanced(taskID string, increment, total, priority int, operation, desc, size string) {
	h.incrementProgressInternal(taskID, increment, total, priority, operation, desc, size, eta.AlgorithmAdvanced)
}

// SetHighLoadMode is a no-op for CLI mode since there's no throttling
func (h *CLIHandler) SetHighLoadMode(durations ...time.Duration) {
	// No-op for CLI mode
	h.logger.Trace().Msg("handler.SetHighLoadMode called (no-op in CLI mode)")
}

// GetContext returns the handler's context for use in crash handling
func (h *CLIHandler) GetContext() context.Context {
	return h.ctx
}


// #############################################################################
// #############################################################################
// #############################################################################



// GUI implementation
type GUIHandler struct {
	ctx	       context.Context
	logger       *zerolog.Logger
	buffer       bytes.Buffer
	progressMap  map[string]int
	throttler    *batch.AdaptiveEventThrottler
	etaCalculators map[string]eta.Provider
}

// LogWriter is the io.Writer that processes logs and routes them through the throttler
type LogWriter struct {
	ctx       context.Context
	throttler *batch.AdaptiveEventThrottler
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	// Parse the log message
	var logMessage map[string]interface{}
	if err := json.Unmarshal(p, &logMessage); err != nil {
		return 0, err
	}

	// Check the log level
	if level, ok := logMessage["level"]; ok {
		// If it's TRACE (-1), don't send to frontend
		if level == -1 {
			// Return the original length to satisfy the Writer interface
			return len(p), nil
		}
	}

	// Send logs through the throttler if available
	if w.throttler != nil {
		w.throttler.AddLog(string(p))
	} else {
		// Fall back to direct emission if throttler isn't available
		runtime.EventsEmit(w.ctx, "log", string(p))
	}
	
	return len(p), nil
}


func NewGUIHandler(ctx context.Context, throttler *batch.AdaptiveEventThrottler) *GUIHandler {
	h := &GUIHandler{
		ctx:         ctx,
		progressMap: make(map[string]int),
		throttler:   throttler,
		etaCalculators: make(map[string]eta.Provider),
	}
	crash.InitReporter(ctx)
	
	// 1. Writer for the GUI Log Viewer (sends raw JSON to the throttler)
	guiLogWriter := &LogWriter{
		ctx:       ctx,
		throttler: throttler,
	}

	// 2. Writer for the in-memory crash/debug report buffer.
	bufferWriter := zerolog.ConsoleWriter{
		Out:        &h.buffer, // Write DIRECTLY to the buffer
		TimeFormat: time.TimeOnly,
		NoColor:    true,
	}

	// 3. Writer for the developer's console (os.Stderr).
	// This is for live debugging and will fail silently on Windows GUI, which is fine.
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.TimeOnly,
	}

	// Each writer now operates independently: failure of one won't affect others.
	multiWriter := zerolog.MultiLevelWriter(
		guiLogWriter,
		bufferWriter,
		consoleWriter,
	)
	
	logger := zerolog.New(multiWriter).With().Timestamp().Logger()
	h.logger = &logger
	common.Log = logger.With().Timestamp().Str("module", "translitkit").Logger()
	
	return h
}


func (h *GUIHandler) IsCLI() bool {
	return false
}

func (h *GUIHandler) GetLogBuffer() bytes.Buffer {
	return h.buffer
}



func (h *GUIHandler) Log(level int8, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, nil)
}

func (h *GUIHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	return log(h, Error, err, behavior, msg, nil)
}

func (h *GUIHandler) LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), err, behavior, msg, nil)
}



func (h *GUIHandler) LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, fields)
}

func (h *GUIHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, Error, err, behavior, msg, fields)
}

// formatETA converts a time.Duration to a human-readable ETA string
func formatETA(etaDuration time.Duration) string {
	if etaDuration < 0 {
		return ""
	}
	
	if etaDuration == 0 {
		return "Done"
	}
	
	// Format the ETA nicely
	if etaDuration.Hours() >= 1 {
		return fmt.Sprintf("ETA: %.0fh %.0fm", 
			math.Floor(etaDuration.Hours()), 
			math.Floor(math.Mod(etaDuration.Minutes(), 60)))
	} else if etaDuration.Minutes() >= 1 {
		return fmt.Sprintf("ETA: %.0fm %.0fs", 
			math.Floor(etaDuration.Minutes()), 
			math.Floor(math.Mod(etaDuration.Seconds(), 60)))
	} else {
		return fmt.Sprintf("ETA: %.0fs", math.Floor(etaDuration.Seconds()))
	}
}

// formatDuration formats a time.Duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d.Hours() >= 1 {
		return fmt.Sprintf("%.0fh", math.Floor(d.Hours()))
	} else if d.Minutes() >= 1 {
		return fmt.Sprintf("%.0fm", math.Floor(d.Minutes()))
	} else {
		seconds := math.Floor(d.Seconds())
		if seconds < 1 && d > 0 {
			seconds = 1
		}
		return fmt.Sprintf("%.0fs", seconds)
	}
}

// formatETAWithConfidence formats an ETAResult with reliability information into a human-readable string
func formatETAWithConfidence(result eta.ETAResult) string {
	if result.Estimate < 0 {
		return ""
	}

	if result.Estimate == 0 {
		return "Done"
	}

	// Format bounds with helper function
	lowerStr := formatDuration(result.LowerBound)
	upperStr := formatDuration(result.UpperBound)
	estimateStr := formatDuration(result.Estimate)

	// Check algorithm type directly instead of using implementation details
	if result.Algorithm == eta.AlgorithmSimple {
		// SimpleETACalculator case - never show reliability regardless of dev mode
		return fmt.Sprintf("ETA: %s", estimateStr)
	}

	// Format reliability level as percentage - only used for advanced calculator
	reliabilityStr := fmt.Sprintf("%.0f%%", result.ReliabilityScore*100)

	// Determine whether to show reliability indicator (only in dev mode)
	showReliability := version.Version == "dev"

	// Calculate whether range is narrow enough to show as point estimate
	etaSeconds := result.Estimate.Seconds()
	rangeDifference := (result.UpperBound.Seconds() - result.LowerBound.Seconds())
	isRangeNarrow := rangeDifference < etaSeconds * 0.2  // Range is within 20% of estimate

	// After 100 samples or 25% completion, evidence shows cross-multiplication
	// is extremely accurate (proven ~5% error margin)
	if result.SampleCount >= 100 || result.PercentDone > 0.25 {
		// Always show point estimate with high sample count
		if showReliability {
			return fmt.Sprintf("ETA: %s (%s)", estimateStr, reliabilityStr)
		}
		return fmt.Sprintf("ETA: %s", estimateStr)
	}

	// With cross-multiplication, we can be more confident at lower thresholds
	if result.CrossMultETA > 0 && result.CrossMultWeight > 0.7 {
		// High cross-mult weight indicates math is reliable for this estimate
		if result.SampleCount >= 50 || result.PercentDone > 0.15 {
			// Show point estimate with high cross-mult weight and good sample count
			if showReliability {
				return fmt.Sprintf("ETA: %s (%s)", estimateStr, reliabilityStr)
			}
			return fmt.Sprintf("ETA: %s", estimateStr)
		}
	}

	// Medium confidence with cross-multiplication - show very narrow range
	if result.CrossMultETA > 0 && result.CrossMultWeight > 0.4 {
		// Medium cross-mult weight (40-70%)
		if result.SampleCount >= 30 || result.PercentDone > 0.1 {
			// Create tight visual bounds (±5%) for good sample counts
			visualLowerBound := time.Duration(float64(result.Estimate) * 0.95)
			visualUpperBound := time.Duration(float64(result.Estimate) * 1.05)

			tighterLowerStr := formatDuration(visualLowerBound)
			tighterUpperStr := formatDuration(visualUpperBound)

			// If strings are the same after formatting, use point estimate
			if tighterLowerStr == tighterUpperStr {
				if showReliability {
					return fmt.Sprintf("ETA: %s (%s)", estimateStr, reliabilityStr)
				}
				return fmt.Sprintf("ETA: %s", estimateStr)
			}

			if showReliability {
				return fmt.Sprintf("ETA: %s-%s (%s)", tighterLowerStr, tighterUpperStr, reliabilityStr)
			}
			return fmt.Sprintf("ETA: %s-%s", tighterLowerStr, tighterUpperStr)
		}
	}

	// Standard display formats based on sample count, reliability, and variability
	switch {
	case (result.SampleCount >= 30 || result.PercentDone > 0.7) && isRangeNarrow && result.Variability < 0.25:
		// Very high reliability, low variability, many samples, narrow range: Show point estimate
		if showReliability {
			return fmt.Sprintf("ETA: %s (%s)", estimateStr, reliabilityStr)
		}
		return fmt.Sprintf("ETA: %s", estimateStr)

	case result.SampleCount >= 15 && result.Variability < 0.4:
		// High reliability, low-moderate variability: Show narrower range with reliability
		// Use average of estimate and bounds to create a narrower display
		tighterLower := time.Duration((float64(result.LowerBound) * 0.3) + (float64(result.Estimate) * 0.7))
		tighterUpper := time.Duration((float64(result.UpperBound) * 0.3) + (float64(result.Estimate) * 0.7))

		tighterLowerStr := formatDuration(tighterLower)
		tighterUpperStr := formatDuration(tighterUpper)

		// If the strings ended up the same after formatting, use the point estimate
		if tighterLowerStr == tighterUpperStr {
			if showReliability {
				return fmt.Sprintf("ETA: %s (%s)", estimateStr, reliabilityStr)
			}
			return fmt.Sprintf("ETA: %s", estimateStr)
		}

		if showReliability {
			return fmt.Sprintf("ETA: %s-%s (%s)", tighterLowerStr, tighterUpperStr, reliabilityStr)
		}
		return fmt.Sprintf("ETA: %s-%s", tighterLowerStr, tighterUpperStr)

	case result.SampleCount >= 5:
		// Moderate samples, show range with reliability
		if showReliability {
			return fmt.Sprintf("ETA: %s-%s (%s)", lowerStr, upperStr, reliabilityStr)
		}
		return fmt.Sprintf("ETA: %s-%s", lowerStr, upperStr)

	default:
		// Limited data, show range without reliability
		return fmt.Sprintf("ETA: %s-%s", lowerStr, upperStr)
	}
}

func log(h MessageHandler, level int8, err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	event := h.ZeroLog().WithLevel(zerolog.Level(level))
	if err != nil {
		//event.Err(err)
		msg = fmt.Sprintf("%s: %v", msg, err)
	}
	if fields != nil {
		event = event.Fields(fields)
	} // TODO check if need to make fields when they are nil
	event.Str("behavior", behavior).Msg(msg)

	if err != nil {
		return &ProcessingError{
			Behavior: behavior,
			Err:      err,
		}
	}
	return nil
}

func (h *GUIHandler) ZeroLog() *zerolog.Logger {
	return h.logger
}

// ResetProgress clears all progress bars and resets the progress tracking map
func (h *GUIHandler) ResetProgress() {
	// Clear the progress map
	h.progressMap = make(map[string]int)
	
	// Reset ETA calculators
	h.etaCalculators = make(map[string]eta.Provider)
	
	// Emit event to frontend to reset all progress bars
	runtime.EventsEmit(h.ctx, "progress-reset", true)
}


// RemoveProgressBar explicitly removes a specific progress bar by ID
func (h *GUIHandler) RemoveProgressBar(taskID string) {
	delete(h.progressMap, taskID)
	
	// Also remove any ETA calculator for this task
	if h.etaCalculators != nil {
		delete(h.etaCalculators, taskID)
	}

	runtime.EventsEmit(h.ctx, "progress-remove", taskID)

	h.ZeroLog().Debug().
		Str("taskID", taskID).
		Msg("Explicitly removed progress bar")
}

// incrementProgressInternal handles progress bar updates with specific ETA algorithm
func (h *GUIHandler) incrementProgressInternal(
	taskID string,
	increment, total, priority int,
	operation, descr, size string,
	algoType eta.AlgorithmType,
) {
	// Make sure we have the ETA calculator map initialized
	if h.etaCalculators == nil {
		h.etaCalculators = make(map[string]eta.Provider)
	}

	// Only create or update ETA for media-bar and item-bar
	isEtaEnabled := taskID == ProgressBarIDMedia || taskID == ProgressBarIDItem

	// Update local progress tracking
	h.progressMap[taskID] += increment
	current := h.progressMap[taskID]

	// Get or create ETA calculator
	var etaStr string
	if isEtaEnabled {
		provider := h.etaCalculators[taskID]
		if provider == nil && total > 0 {
			// Create the appropriate ETA calculator based on the algorithm type
			if algoType == eta.AlgorithmAdvanced {
				provider = eta.NewETACalculator(int64(total))
			} else { // Default to Simple if algoType == eta.AlgorithmSimple
				provider = eta.NewSimpleETACalculator(int64(total))
			}
			h.etaCalculators[taskID] = provider
		} else if provider != nil && provider.GetAlgorithmType() != algoType && total > 0 {
			// Algorithm type mismatch for existing calculator, recreate
			// This handles cases where a taskID might be reused with a different ETA requirement
			h.ZeroLog().Warn().
				Str("taskID", taskID).
				Str("existingAlgo", provider.GetAlgorithmType().String()).
				Str("requestedAlgo", algoType.String()).
				Msg("ETA algorithm type mismatch for task, recreating calculator.")

			if algoType == eta.AlgorithmAdvanced {
				provider = eta.NewETACalculator(int64(total))
			} else {
				provider = eta.NewSimpleETACalculator(int64(total))
			}
			h.etaCalculators[taskID] = provider
		} else if provider != nil && provider.GetTotalTasks() != int64(total) && total > 0 {
			// Update existing calculator when total changes (preserves rate history)
			provider.UpdateTotalTasks(int64(total))
		}

		// Update ETA calculation
		if provider != nil {
			provider.TaskCompleted(int64(current))

			// Get formatted ETA with confidence if available
			etaResult := provider.CalculateETAWithConfidence()

			// Only show ETA if a valid estimate is available
			if etaResult.Estimate >= 0 {
				// Check algorithm type directly for clean decisioning
				// For SimpleETACalculator, add extra conditions to avoid premature ETAs
				if etaResult.Algorithm == eta.AlgorithmAdvanced || // Advanced calculator - show ETA normally
				   increment > 0 || // New work was done - show ETA
				   provider.ElapsedTime() >= eta.SimpleETAMinimumElapsed { // Enough time elapsed - show ETA
					etaStr = formatETAWithConfidence(etaResult)
				}
			}
		}
	}

	percent := 0.0
	if total > 0 {
		percent = (float64(current) / float64(total)) * 100.0
	} else {
		// fallback if total=0
		percent = float64(current)
	}

	// If we have ETA, include it in the description
	if isEtaEnabled && etaStr != "" {
		descr = fmt.Sprintf("%s [%s]", descr, etaStr)
	}

	// Create payload for event
	payload := map[string]interface{}{
		"id":          taskID,
		"progress":    percent,
		"current":     current,
		"total":       total,
		"operation":   operation,
		"description": descr,
		"color":       "",
		"size":        size,
		"striped":     true,
		"animated":    true,
		"priority":    priority,
	}

	// Send through throttler if available
	if h.throttler != nil {
		h.throttler.UpdateProgress(taskID, payload)
	} else {
		// Fallback to direct emission
		runtime.EventsEmit(h.ctx, "progress", payload)
	}

	// Cleanup if complete
	if total > 0 && current >= total {
		delete(h.progressMap, taskID)
		delete(h.etaCalculators, taskID)
	}
}

// IncrementProgress updates progress with simple ETA calculation
func (h *GUIHandler) IncrementProgress(taskID string, increment, total, priority int, operation, descr, size string) {
	h.incrementProgressInternal(taskID, increment, total, priority, operation, descr, size, eta.AlgorithmSimple)
}

// IncrementProgressAdvanced updates progress with advanced ETA calculation
func (h *GUIHandler) IncrementProgressAdvanced(taskID string, increment, total, priority int, operation, descr, size string) {
	h.incrementProgressInternal(taskID, increment, total, priority, operation, descr, size, eta.AlgorithmAdvanced)
}

// BulkUpdateProgress handles multiple progress updates efficiently
// Useful for task resumption with thousands of updates
func (h *GUIHandler) BulkUpdateProgress(updates map[string]map[string]interface{}) {
	// Make sure we have the ETA calculator map initialized
	if h.etaCalculators == nil {
		h.etaCalculators = make(map[string]eta.Provider)
	}
	
	// First pass: update our progress map and ETAs
	for id, data := range updates {
		if current, ok := data["current"].(int); ok {
			h.progressMap[id] = current
			
			// Only create or update ETA for media-bar and item-bar
			isEtaEnabled := id == ProgressBarIDMedia || id == ProgressBarIDItem
			
			if isEtaEnabled {
				if total, ok := data["total"].(int); ok && total > 0 {
					// Get or create the ETA calculator
					provider := h.etaCalculators[id]
					if provider == nil {
						// Use the appropriate calculator based on task ID
						if id == ProgressBarIDItem {
							provider = eta.NewETACalculator(int64(total))
						} else {
							provider = eta.NewSimpleETACalculator(int64(total))
						}
						h.etaCalculators[id] = provider
					} else if provider.GetTotalTasks() != int64(total) {
						// Update total without resetting (preserves rate history)
						provider.UpdateTotalTasks(int64(total))
					}

					// Update ETA with current progress
					// Only track progressive changes since the previous update
					if provider.GetCompletedTasks() < int64(current) {
						provider.TaskCompleted(int64(current))
					}

					// Calculate and format ETA string with confidence
					etaResult := provider.CalculateETAWithConfidence()
					var etaStr string

					if etaResult.Estimate >= 0 {
						// Using algorithm type directly for clean decisioning
						// Only show ETA if:
						// - For advanced calculator - show normally
						// - For SimpleETACalculator - only show if enough time passed
						// This prevents showing ETA immediately on resumption for SimpleETACalculator
						if etaResult.Algorithm == eta.AlgorithmAdvanced || // Advanced calculator - show ETA normally
						   provider.ElapsedTime() >= eta.MinBulkProgressElapsed { // More time elapsed - show ETA
							etaStr = formatETAWithConfidence(etaResult)

							// Update description with ETA info
							if desc, ok := data["description"].(string); ok && etaStr != "" {
								data["description"] = fmt.Sprintf("%s [%s]", desc, etaStr)
							}
						}
					}
				}
			}
		}
	}
	
	// Process through throttler if available
	if h.throttler != nil {
		h.throttler.BulkUpdateProgress(updates)
	} else {
		// Fallback to individual updates
		for _, update := range updates {
			runtime.EventsEmit(h.ctx, "progress", update)
		}
	}
	
	// Cleanup completed items
	for id, data := range updates {
		current, hasC := data["current"].(int)
		total, hasT := data["total"].(int)
		if hasC && hasT && total > 0 && current >= total {
			delete(h.progressMap, id)
			delete(h.etaCalculators, id)
		}
	}
}

func (h *GUIHandler) HandleStatus(status string) {
	runtime.EventsEmit(h.ctx, "status", status)
}

// SetHighLoadMode pre-emptively enables high load mode of Adaptive Event Throttling System
// This gives a "head start" instead of waiting for auto-detection
// helpful for previousy interrupted task resumption
func (h *GUIHandler) SetHighLoadMode(durations ...time.Duration) {
	if h.throttler != nil {
		// Pass the optional duration to the throttler
		if len(durations) > 0 {
			h.ZeroLog().Trace().Dur("duration", durations[0]).Msg("Entering high load mode with custom duration")
			h.throttler.SetHighLoadModeWithTimeout(durations[0])
		} else {
			h.ZeroLog().Trace().Msg("Entering high load mode with default duration")
			h.throttler.SetHighLoadModeWithTimeout()
		}
	} else {
		h.ZeroLog().Warn().Msg("Cannot enter high load mode: throttler is nil")
	}
}

// GetContext returns the handler's context for use in crash handling
func (h *GUIHandler) GetContext() context.Context {
	return h.ctx
}

// GetScraperLibLogForwarder returns a callback function that logs messages with a [scraper-lib] prefix
// This is used to forward browser-related logs from go-rod to the frontend
func GetScraperLibLogForwarder(handler MessageHandler) func(string) {
	return func(statusMessage string) {
		handler.ZeroLog().Info().Msgf("[scraper-lib] %s", statusMessage)
	}
}



func placeholder3456() {
	fmt.Println("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓂𝓹𝓲𝓵𝓮𝓻")
}