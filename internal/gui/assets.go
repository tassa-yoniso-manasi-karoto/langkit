package gui

import "embed"

// Embedded frontend assets shared between Wails and server modes

//go:embed all:frontend/dist
var assets embed.FS

//go:embed frontend/icon/appicon.png
var icon []byte