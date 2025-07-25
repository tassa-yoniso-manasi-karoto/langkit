package gui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2"
	wailslogger "github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

const name = "langkit"

func Run() {
	defer func() {
		if r := recover(); r != nil {
			exitOnError(fmt.Errorf("panic: %v", r))
		}
	}()
	
	// Setup logger for server initialization
	writer := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.TimeOnly,
	}
	logger := zerolog.New(writer).With().Timestamp().Str("module", "gui").Logger()
	
	// Initialize servers before creating Wails app
	logger.Info().Msg("Initializing servers...")
	servers, err := InitializeServers(context.Background(), logger)
	if err != nil {
		exitOnError(fmt.Errorf("failed to initialize servers: %w", err))
	}
	
	// Create app with pre-initialized servers
	app := NewAppWithServers(servers)
	
	// Create runtime config for middleware
	config := RuntimeConfig{
		APIPort: servers.APIServer.GetPort(),
		WSPort:  servers.WSServer.GetPort(),
		Mode:    "wails",
		Runtime: "wails",
	}

	err = wails.Run(&options.App{
		Title:             name,
		Height:            1024,
		MinWidth:          1030,
		MinHeight:         200,
		//MaxWidth:          1280,
		//MaxHeight:         800,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 26, G: 26, B: 26, A: 255},
		AssetServer:       &assetserver.Options{
			Assets: assets,
			Middleware: NewConfigInjectionMiddleware(config),
		},
		Menu:              nil,
		Logger:            nil,
		LogLevel:          wailslogger.DEBUG,
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnBeforeClose:     app.beforeClose,
		OnShutdown:        app.shutdown,
		WindowStartState:  options.Normal,
		Bind: []interface{}{
			app,
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:       true,
			DisableWebViewDrop:   false, // keeps this false or else drag & drop will be entirely disabled(!)
			CSSDropProperty:      "--wails-drop-target",
			CSSDropValue:         "drop",
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			BackdropType:         windows.Auto,
			Theme:	              windows.Dark,
			WebviewUserDataPath:  "",
			ZoomFactor:           1.0,
		},
		// Mac platform specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   name,
				Message: "",
				Icon:    icon,
			},
		},
		Linux: &linux.Options{
		    Icon: icon,
		    WindowIsTranslucent: false,
		    WebviewGpuPolicy: linux.WebviewGpuPolicyAlways,
		    ProgramName: name,
		},
	})
	
	// handler != nil is to support Wails' double start that wails dev performs
	if err != nil && handler != nil {
		exitOnError(err)
	}
}


