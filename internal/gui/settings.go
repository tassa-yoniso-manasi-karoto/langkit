package gui

import (
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

func (a *App) InitSettings() error {
	return config.InitConfig("")
}

func (a *App) LoadSettings() (config.Settings, error) {
	return config.LoadSettings()
}

func (a *App) SaveSettings(settings config.Settings) error {
	return config.SaveSettings(settings)
}