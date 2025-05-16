package summary

import (
	"context"
	"fmt"
	"strconv"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

// Provider defines the interface for generating summaries
type Provider interface {
	// Generate a summary from text
	Generate(ctx context.Context, text string, options Options) (string, error)
	
	// Get the name of the provider
	GetName() string
	
	// Get supported models
	GetSupportedModels() []llms.ModelInfo
}

// BaseProvider implements common functionality for summary providers
type BaseProvider struct {
	llmClient   *llms.Client
	llmProvider string
}

// NewBaseProvider creates a new base provider
func NewBaseProvider(llmClient *llms.Client, providerName string) BaseProvider {
	return BaseProvider{
		llmClient:   llmClient,
		llmProvider: providerName,
	}
}

// GetName returns the provider name
func (p *BaseProvider) GetName() string {
	return p.llmProvider
}

// GetSupportedModels returns supported models for this provider
func (p *BaseProvider) GetSupportedModels() []llms.ModelInfo {
	provider, ok := p.llmClient.GetProvider(p.llmProvider)
	if !ok {
		return nil
	}
	
	return provider.GetAvailableModels()
}

// GeneratePrompt creates a prompt for the model based on options
func GeneratePrompt(text string, options Options) string {
	var prompt string
	
	if options.Style == "brief" {
		prompt = "Create a concise summary of the following content."
	} else if options.Style == "detailed" {
		prompt = "Provide a comprehensive summary of the following content."
	} else if options.Style == "character-focused" {
		prompt = "Create a summary focused on the main characters in the following content."
	} else {
		prompt = "Summarize the following content."
	}
	
	prompt += " The summary should be about " + 
		strconv.Itoa(options.MaxLength) + 
		" words. "
	
	if options.IncludeCharacters {
		prompt += "Include mention of key characters. "
	}
	
	if options.IncludePlot {
		prompt += "Focus on the main plot points and narrative arc. "
	}
	
	if options.IncludeThemes {
		prompt += "Include analysis of major themes and motifs. "
	}
	
	if options.ToneStyle == "analytical" {
		prompt += "Use an analytical tone. "
	} else if options.ToneStyle == "casual" {
		prompt += "Use a casual, conversational tone. "
	}
	
	if options.Language != "" {
		prompt += fmt.Sprintf("Provide the summary in %s. ", options.Language)
	}
	
	prompt += "\n\nContent to summarize:\n" + text
	
	return prompt
}