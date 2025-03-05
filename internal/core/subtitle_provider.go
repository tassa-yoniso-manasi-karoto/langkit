package core

import (
	"fmt"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
)

// We're using the subs.Subtitles type for our interface
type Subtitles = subs.Subtitles

// DefaultSubtitleProvider implements the SubtitleProvider interface
type DefaultSubtitleProvider struct {
	handler MessageHandler
}

// NewSubtitleProvider creates a new DefaultSubtitleProvider
func NewSubtitleProvider(handler MessageHandler) SubtitleProvider {
	return &DefaultSubtitleProvider{
		handler: handler,
	}
}

// OpenFile opens a subtitle file
func (p *DefaultSubtitleProvider) OpenFile(path string, clean bool) (*Subtitles, error) {
	subtitles, err := subs.OpenFile(path, clean)
	if err != nil {
		return nil, fmt.Errorf("failed to open subtitle file %s: %w", path, err)
	}
	
	return subtitles, nil
}

// TrimCC2Dubs trims closed captions for dubbing
func (p *DefaultSubtitleProvider) TrimCC2Dubs(subs *Subtitles) {
	if subs == nil {
		p.handler.ZeroLog().Warn().Msg("Attempted to trim CC2Dubs on nil subtitles")
		return
	}
	
	subs.TrimCC2Dubs()
}

// Subs2Dubs converts subtitles to dubbing format
func (p *DefaultSubtitleProvider) Subs2Dubs(subs *Subtitles, path, sep string) error {
	if subs == nil {
		return fmt.Errorf("cannot convert nil subtitles to dubs")
	}
	
	err := subs.Subs2Dubs(path, sep)
	if err != nil {
		return fmt.Errorf("failed to convert subtitles to dubs: %w", err)
	}
	
	return nil
}

// Write writes subtitles to a file
func (p *DefaultSubtitleProvider) Write(subs *Subtitles, path string) error {
	if subs == nil {
		return fmt.Errorf("cannot write nil subtitles to file")
	}
	
	err := subs.Write(path)
	if err != nil {
		return fmt.Errorf("failed to write subtitles to file %s: %w", path, err)
	}
	
	return nil
}