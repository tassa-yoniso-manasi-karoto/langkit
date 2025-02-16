package gui

import (
	"fmt"
	"os"
	"bytes"
	
	"github.com/ncruces/zenity"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

// ShowErrorDialog uses zenity to display an error dialog to the user.
func ShowErrorDialog(err error) {
	message := fmt.Sprintf("𝘈𝘩 𝘴𝘩𝘪𝘵, 𝘩𝘦𝘳𝘦 𝘸𝘦 𝘨𝘰 𝘢𝘨𝘢𝘪𝘯. Langkit encountered an error.\n\n"+
		"𝗔 𝗰𝗿𝗮𝘀𝗵 𝗿𝗲𝗽𝗼𝗿𝘁 𝗶𝘀 𝗯𝗲𝗶𝗻𝗴 𝗰𝗿𝗲𝗮𝘁𝗲𝗱 𝗮𝘁:\n%s\n"+
		"𝗣𝗹𝗲𝗮𝘀𝗲 𝘀𝘂𝗯𝗺𝗶𝘁 𝗶𝘁 𝘁𝗼 𝘁𝗵𝗲 𝗱𝗲𝘃𝗲𝗹𝗼𝗽𝗲𝗿.\n\nError: %v\n", crash.GetCrashDir(), err)
	errZenity := zenity.Error(message, zenity.Title("Langkit Error"), zenity.OKLabel("OK"))
	if errZenity != nil {
		fmt.Printf("Failed to show error dialog: %v\n", errZenity)
	}
	os.Exit(1)
}

func writeCrashLog(mainErr error) (string, error) {
	settings, err := config.LoadSettings()
	if err != nil {
		// Continue with empty settings if loading fails
		fmt.Printf("Warning: Failed to load settings: %v\n", err)
	}

	runtimeInfo := crash.NewRuntimeInfo().String()

	var logBuffer bytes.Buffer
	if handler != nil {
		logBuffer = handler.GetLogBuffer()
	}

	crashPath, err := crash.WriteReport(
		mainErr,
		runtimeInfo,
		settings,
		logBuffer,
	)
	if err != nil {
		return "", fmt.Errorf("failed to write crash report: %w", err)
	}
	return crashPath, nil
}
