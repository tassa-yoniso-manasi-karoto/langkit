package core

import "github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"

// Progress bar identifiers - aliases to progress package constants for backward compatibility
const (
	ProgressBarIDMedia = progress.BarMediaBar // Overall media processing progress
	ProgressBarIDItem  = progress.BarItemBar  // Individual item processing progress
)