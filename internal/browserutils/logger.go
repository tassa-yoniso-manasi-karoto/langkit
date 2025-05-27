package browserutils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LogForwarderCallback is a function that forwards status messages
type LogForwarderCallback func(statusMessage string)

// CustomRodLogger implements github.com/go-rod/rod/lib/utils.Logger
// It parses go-rod logs and forwards meaningful status messages via the callback
type CustomRodLogger struct {
	logForwarder       LogForwarderCallback
	progressRegex      *regexp.Regexp
	unzipRegex         *regexp.Regexp
	downloadedRegex    *regexp.Regexp
	downloadStartRegex *regexp.Regexp
	isExtracting       bool // Track if we're in extraction phase
	lastProgressPercent int // Track last reported progress to avoid duplicates
}

// NewCustomRodLogger creates a new custom logger for go-rod
func NewCustomRodLogger(forwarder LogForwarderCallback) *CustomRodLogger {
	return &CustomRodLogger{
		logForwarder:       forwarder,
		progressRegex:      regexp.MustCompile(`Progress:\s*(\d{2,3})%`),
		unzipRegex:         regexp.MustCompile(`Unzip:\s*(.+)`),
		downloadedRegex:    regexp.MustCompile(`Downloaded:\s*(.+)`),
		downloadStartRegex: regexp.MustCompile(`Download:\s*(https?://\S+)`),
		isExtracting:       false,
		lastProgressPercent: -1, // Initialize to -1 so first 0% gets reported
	}
}

// Println implements the utils.Logger interface
func (cl *CustomRodLogger) Println(args ...interface{}) {
	fullMsg := fmt.Sprint(args...)
	
	// Strip the launcher prefix and timestamp
	// Example: "[launcher.Browser]2025/05/27 16:21:46 "
	prefixPattern := regexp.MustCompile(`^\[launcher\.Browser\]\d{4}/\d{2}/\d{2}\s\d{2}:\d{2}:\d{2}\s*`)
	coreMsg := prefixPattern.ReplaceAllString(fullMsg, "")

	var statusToLog string

	// Check for download start
	if matches := cl.downloadStartRegex.FindStringSubmatch(coreMsg); len(matches) > 1 {
		cl.isExtracting = false // Reset extraction state
		cl.lastProgressPercent = -1 // Reset progress tracking
		statusToLog = fmt.Sprintf("Starting browser download from %s", matches[1])
		
	// Check for unzip start
	} else if matches := cl.unzipRegex.FindStringSubmatch(coreMsg); len(matches) > 1 {
		cl.isExtracting = true
		cl.lastProgressPercent = -1 // Reset progress tracking for extraction phase
		statusToLog = fmt.Sprintf("Extracting browser to %s...", matches[1])
		
	// Check for download completion
	} else if matches := cl.downloadedRegex.FindStringSubmatch(coreMsg); len(matches) > 1 {
		cl.isExtracting = false
		statusToLog = fmt.Sprintf("Browser ready at: %s", matches[1])
		
	// Check for progress updates
	} else if matches := cl.progressRegex.FindStringSubmatch(coreMsg); len(matches) > 1 {
		progressPercent, _ := strconv.Atoi(matches[1])
		
		// Only process if this is a different percentage than last time
		if progressPercent != cl.lastProgressPercent {
			// Update last progress to prevent duplicate logs
			cl.lastProgressPercent = progressPercent
			
			// Determine the current phase based on state
			var phase string
			if cl.isExtracting {
				phase = "Extracting browser"
			} else {
				phase = "Downloading browser"
			}
			
			// Throttle progress logs - only report at 5% intervals (and 99%, 100%)
			if progressPercent%5 == 0 || progressPercent == 99 || progressPercent == 100 {
				statusToLog = fmt.Sprintf("%s: %s%%", phase, matches[1])
			}
		}
		
	// Check for errors
	} else if strings.Contains(strings.ToLower(coreMsg), "failed") || 
	          strings.Contains(strings.ToLower(coreMsg), "error") {
		statusToLog = fmt.Sprintf("Browser preparation error: %s", coreMsg)
	}

	// Forward the status if we have something to log
	if statusToLog != "" {
		cl.logForwarder(statusToLog)
	}
}