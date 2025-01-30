package gui

import (
	"context"
	
	"github.com/wailsapp/wails/v2/pkg/runtime"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

type App struct {
	ctx     context.Context
	handler core.MessageHandler // FIXME TBD if necessary
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.handler = core.NewGUIHandler(ctx)
}

func (a *App) domReady(ctx context.Context) {
	if err := config.InitConfig(""); err != nil {
		runtime.LogError(ctx, "Failed to initialize config: "+err.Error())
		return
	}

	// Load settings and emit to frontend
	settings, err := config.LoadSettings()
	if err != nil {
		runtime.LogError(ctx, "Failed to load settings: "+err.Error())
		return
	}

	// Emit settings to frontend
	runtime.EventsEmit(ctx, "settings-loaded", settings)
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	// Perform your teardown here
}


