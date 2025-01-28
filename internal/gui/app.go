package gui

import (
	"context"
	
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
	a.handler = core.NewGUIHandler(ctx, core.NewLogger())
}

// domReady is called after front-end resources have been loaded
func (a App) domReady(ctx context.Context) {
	// Add your action here
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


