package gui

import (
	"embed"
	"fmt"

	"github.com/gookit/color"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)


//go:embed all:frontend/dist
var assets embed.FS

//TODO go:embed build/appicon.png
var icon []byte

func Run() {
	defer func() {
		// TODO maybe use panicwrap because this doesn't seem to recover panic from go code called from frontend
		if r := recover(); r != nil {
			exitOnError(fmt.Errorf("panic: %v", r))
		}
	}()
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:             "langkit",
		/*Width:             1024,
		Height:            768,
		MinWidth:          1024,
		MinHeight:         768,
		MaxWidth:          1280,
		MaxHeight:         800,*/
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		AssetServer:       &assetserver.Options{
			Assets: assets,
		},
		Menu:              nil,
		Logger:            nil,
		LogLevel:          logger.DEBUG,
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnBeforeClose:     app.beforeClose,
		OnShutdown:        app.shutdown,
		WindowStartState:  options.Normal,
		Bind: []interface{}{
			app,
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			// DisableFramelessWindowDecorations: false,
			WebviewUserDataPath: "",
			ZoomFactor: 1.0,
		},
		// Mac platform specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "langkit",
				Message: "",
				Icon:    icon,
			},
		},
	})
	
	//err = fmt.Errorf("TEST ERROR")
	
	// handler != nil is to support Wails' double start that wails dev performs
	if err != nil && handler != nil {
		exitOnError(err)
	}
}

func exitOnError(err error) {
	// Instead of logging the error (which might not be visible to a GUI user),
	// we create a crash dump and then display an error message dialog.
	go ShowErrorDialog(err)

	if _, dumpErr := writeCrashLog(err); dumpErr != nil {
		color.Redf("Error dumping log file: %v\n", dumpErr)
	}
}

