package gui

import (
	"fmt"
	"os"

	"github.com/gookit/color"
	"github.com/ncruces/zenity"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

func exitOnError(mainErr error) {
	// Instead of logging the error (which might not be visible to a GUI user),
	// we create a crash dump and then display an error message dialog.
	go ShowErrorDialog(mainErr)

	settings, err := config.LoadSettings()
	if err != nil {
		// Continue with empty settings if loading fails
		fmt.Printf("Warning: Failed to load settings: %v\n", err)
	}

	// Flush any pending events if throttler is available
	if appThrottler != nil {
		appThrottler.SyncFlush()
	}

	_, err = crash.WriteReport(crash.ModeCrash, mainErr, settings, handler.GetLogBuffer(), false)
	if err != nil {
		color.Redf("failed to write crash report: %v", err)
	}
	os.Exit(1)
}

// ShowErrorDialog uses zenity to display an error dialog to the user.
func ShowErrorDialog(mainErr error) {
	message := fmt.Sprintf("𝘈𝘩 𝘴𝘩𝘪𝘵, 𝘩𝘦𝘳𝘦 𝘸𝘦 𝘨𝘰 𝘢𝘨𝘢𝘪𝘯. Langkit encountered an error.\n\n"+
		"𝗔 𝗰𝗿𝗮𝘀𝗵 𝗿𝗲𝗽𝗼𝗿𝘁 𝗶𝘀 𝗯𝗲𝗶𝗻𝗴 𝗰𝗿𝗲𝗮𝘁𝗲𝗱 𝗮𝘁:\n%s\n"+
		"𝗣𝗹𝗲𝗮𝘀𝗲 𝘀𝘂𝗯𝗺𝗶𝘁 𝗶𝘁 𝘁𝗼 𝘁𝗵𝗲 𝗱𝗲𝘃𝗲𝗹𝗼𝗽𝗲𝗿.\n\nError: %v\n", crash.GetCrashDir(), mainErr)
	err := zenity.Error(message, zenity.Title("Langkit Error"), zenity.OKLabel("OK"))
	if err != nil {
		fmt.Printf("Failed to show error dialog: %v\n", err)
	}
}
