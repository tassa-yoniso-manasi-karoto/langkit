package ui

import (
	"sync"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui/dialogs"
)

var (
	instance *Manager
	once     sync.Once
)

// Manager provides access to UI runtime-specific operations
type Manager struct {
	fileDialog dialogs.FileDialog
	// Future extensions:
	// messageDialog MessageDialog
	// clipboard     Clipboard
	// systemTray    SystemTray
}

// Initialize sets up the UI manager with specific implementations
func Initialize(fileDialog dialogs.FileDialog) {
	once.Do(func() {
		instance = &Manager{
			fileDialog: fileDialog,
		}
	})
}

// GetFileDialog returns the file dialog interface
func GetFileDialog() dialogs.FileDialog {
	if instance == nil {
		panic("ui manager not initialized")
	}
	return instance.fileDialog
}

// IsInitialized returns whether the UI manager has been initialized
func IsInitialized() bool {
	return instance != nil
}