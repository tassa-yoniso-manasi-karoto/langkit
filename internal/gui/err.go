package gui

import (
	// "bytes"
	"fmt"
	// "io"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
	

	"github.com/ncruces/zenity"
)

// ShowErrorDialog uses zenity to display an error dialog to the user.
func ShowErrorDialog(crashFilePath string, err error) {
	message := fmt.Sprintf("The application encountered an error.\n\nA crash report has been created at:\n%s\n\nError: %v\n", crashFilePath, err)
	errZenity := zenity.Error(message, zenity.Title("Langkit Error"), zenity.OKLabel("OK"))
	if errZenity != nil {
		fmt.Printf("Failed to show error dialog: %v\n", errZenity)
	}
	os.Exit(1)
}

// writeCrashLog creates a crash log file at logFilePath and writes the error details into it.
// TODO add stack trace, log history, runtime info, settings with API keys sanitized, network status
func writeCrashLog(err error, logFilePath string) error {
	f, err2 := os.Create(logFilePath)
	if err2 != nil {
		return err2
	}
	defer f.Close()

	_, err2 = f.WriteString(fmt.Sprintf("Application error: %v\nTimestamp: %s\n", err, time.Now().Format(time.RFC1123)))
	return err2
}

// dumpError writes the crash dump (including the error, stack trace, and log history)
// to a uniquely named file.
func dumpError(err error, logFilePath string) (string, error) {
	// Determine the directory in which to create the crash dump (using the same directory as our log file).
	crashDir := filepath.Dir(logFilePath)
	timestamp := time.Now().Format("20060102_150405")
	crashFileName := fmt.Sprintf("crash_%s.log", timestamp)
	crashFilePath := filepath.Join(crashDir, crashFileName)

	// Create the crash dump file.
	crashDump, err2 := os.Create(crashFilePath)
	if err2 != nil {
		return "", fmt.Errorf("failed to create crash dump file: %w", err2)
	}
	defer crashDump.Close()

	// Write a header, the error message, stack trace, and log history.
	_, err2 = crashDump.WriteString(fmt.Sprintf("CRASH REPORT - %s\n\n", time.Now().Format(time.RFC1123)))
	if err2 != nil {
		return "", err2
	}
	_, err2 = crashDump.WriteString(fmt.Sprintf("ERROR: %v\n\n", err))
	if err2 != nil {
		return "", err2
	}
	_, err2 = crashDump.WriteString("STACK TRACE:\n")
	if err2 != nil {
		return "", err2
	}
	_, err2 = crashDump.WriteString(string(debug.Stack()))
	if err2 != nil {
		return "", err2
	}
	_, err2 = crashDump.WriteString("\n\nLOG HISTORY:\n")
	if err2 != nil {
		return "", err2
	}
	// _, err2 = io.Copy(crashDump, &logBuffer)
	// if err2 != nil {
	// 	return "", err2
	// }

	return crashFilePath, nil
}
