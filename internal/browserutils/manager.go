package browserutils

import (
	"fmt"

	"github.com/go-rod/rod/lib/launcher"
)

// EnsureBrowserAndGetControlURL manages the browser binary and returns its control URL
// It checks if a browser is already available, downloads if necessary, and launches it
func EnsureBrowserAndGetControlURL(logForwarder LogForwarderCallback) (controlURL string, l *launcher.Launcher, err error) {
	bm := launcher.NewBrowser()

	logForwarder(fmt.Sprintf("Checking for browser at %s...", bm.BinPath()))
	
	// Check if browser binary already exists and is valid
	if errValidate := bm.Validate(); errValidate == nil {
		logForwarder(fmt.Sprintf("Browser found and valid at %s", bm.BinPath()))
		
		// Create launcher with existing binary
		l = launcher.New().Bin(bm.BinPath())
		
		// Set headless mode for background operation
		l.Headless(true)
		
		// Launch the browser
		controlURL, errLaunch := l.Launch()
		if errLaunch != nil {
			logForwarder(fmt.Sprintf("Failed to launch existing browser: %v", errLaunch))
			return "", nil, fmt.Errorf("failed to launch existing browser: %w", errLaunch)
		}
		
		logForwarder("Browser launched successfully")
		return controlURL, l, nil
	}
	
	// Browser not found or invalid, need to download
	logForwarder(fmt.Sprintf("Browser not found or invalid (revision %d), preparing for download...", bm.Revision))

	// Set our custom logger to capture download progress
	bm.Logger = NewCustomRodLogger(logForwarder)

	// Get() will trigger download/extraction if needed
	// Our CustomRodLogger will parse and forward progress updates
	binPath, errGet := bm.Get()
	if errGet != nil {
		logForwarder(fmt.Sprintf("Error ensuring browser is available: %v", errGet))
		return "", nil, fmt.Errorf("error ensuring browser: %w", errGet)
	}
	
	// Browser is ready
	logForwarder(fmt.Sprintf("Browser binary ready at: %s", binPath))

	// Create launcher with the downloaded binary
	l = launcher.New().Bin(binPath)
	
	// Set headless mode
	l.Headless(true)

	// Launch the browser
	logForwarder("Launching browser...")
	controlURL, errLaunch := l.Launch()
	if errLaunch != nil {
		logForwarder(fmt.Sprintf("Failed to launch browser: %v", errLaunch))
		return "", l, fmt.Errorf("failed to launch browser: %w", errLaunch)
	}
	
	logForwarder("Browser launched successfully")
	return controlURL, l, nil
}