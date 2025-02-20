package gui

import (
	"fmt"
	"os"
	"io"
	
	"github.com/ncruces/zenity"
	"github.com/gookit/color"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

func (a *App) ExportDebugReport() error {
	settings, err := config.LoadSettings()
	if err != nil {
		// Continue with empty settings if loading fails
		fmt.Printf("Warning: Failed to load settings: %v\n", err)
	}
	zipPath, err := crash.WriteReport(
		crash.ModeDebug,
		nil,
		settings,
		handler.GetLogBuffer(),
		false,
	)
	if err != nil {
		return err
	}

	// Prompt user for a place to save the file
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Debug Report",
		DefaultFilename: "langkit_debug_report.zip",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Zip Archive",
				Pattern:     "*.zip",
			},
		},
	})
	if err != nil || savePath == "" {
		// user canceled or error
		return err
	}

	// Copy the file from `zipPath` to `savePath`
	err = copyFile(zipPath, savePath)
	if err != nil {
		return err
	}

	// Possibly let them know it’s done
	runtime.EventsEmit(a.ctx, "debugReportExported", savePath)
	return nil
}

// Simple copyFile utility
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}


func exitOnError(mainErr error) {
	// Instead of logging the error (which might not be visible to a GUI user),
	// we create a crash dump and then display an error message dialog.
	go ShowErrorDialog(mainErr)

	settings, err := config.LoadSettings()
	if err != nil {
		// Continue with empty settings if loading fails
		fmt.Printf("Warning: Failed to load settings: %v\n", err)
	}
	
	_, err = crash.WriteReport(crash.ModeCrash, mainErr, settings, handler.GetLogBuffer(), false)
	if err != nil {
		color.Redf("failed to write crash report: %w", err)
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
