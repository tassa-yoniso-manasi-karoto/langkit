package summary

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

// Provider defines the interface for generating summaries
type Provider interface {
	// Generate creates a summary from the given subtitleText using the provided options.
	// The subtitleText is already prepared.
	// inputLanguageName is the English name of the subtitle's language (e.g., "Japanese").
	Generate(ctx context.Context, subtitleText string, inputLanguageName string, options Options) (string, error)
	GetName() string
	GetSupportedModels() []llms.ModelInfo
}

// BaseProvider (definition remains the same)
type BaseProvider struct {
	llmClient   *llms.Client
	llmProvider string 
}

// NewBaseProvider (definition remains the same)
func NewBaseProvider(llmClient *llms.Client, providerName string) BaseProvider {
	return BaseProvider{
		llmClient:   llmClient,
		llmProvider: providerName,
	}
}

// GetName (definition remains the same)
func (p *BaseProvider) GetName() string {
	return p.llmProvider
}

// GetSupportedModels returns the list of models supported by this provider
func (p *BaseProvider) GetSupportedModels() []llms.ModelInfo {
	provider, ok := p.llmClient.GetProvider(p.llmProvider)
	if !ok {
		if logger.GetLevel() <= zerolog.ErrorLevel { 
			logger.Error().Str("provider_name", p.llmProvider).Msg("Underlying LLM provider not found in BaseProvider.GetSupportedModels")
		}
		return nil
	}
	return provider.GetAvailableModels(context.Background())
}

// GeneratePrompt creates a prompt for the model based on options.
// If options.CustomPrompt is set, it's used directly, and subtitleText is appended.
// inputLanguageName is the English name of the subtitle's language (e.g., "Japanese").
func GeneratePrompt(subtitleText string, inputLanguageName string, options Options) string {
	if options.CustomPrompt != "" {
		return options.CustomPrompt + "\n\n--- Subtitle Content ---\n" + subtitleText
	}

	var prompt strings.Builder
	prompt.WriteString("Generate a pure factual summary of the narrative content below. ")
	prompt.WriteString("Your response must consist solely of the story's plot and events - no preambles, ")
	prompt.WriteString("no commentary, no thematic analysis, and no meta-statements about the content itself. ")
	prompt.WriteString("Begin directly with what happens in the story, presenting events in chronological order. ")
	prompt.WriteString("Focus exclusively on actions, dialogue, and plot developments that advance the narrative.")

	if inputLanguageName != "" {
		prompt.WriteString(fmt.Sprintf(" The content is in %s.", inputLanguageName))
	}

	if options.OutputLanguage != "" {
		prompt.WriteString(fmt.Sprintf(" Write the summary in %s.", options.OutputLanguage))
	} else {
		prompt.WriteString(" Write in English, or if the original content is clearly in another language and not English, write in that original language.")
	}

	if options.MaxLength > 0 {
		prompt.WriteString(fmt.Sprintf(" Keep the summary to approximately %d words.", options.MaxLength))
	}

	prompt.WriteString("\n\n--- Subtitle Content to Summarize ---\n")
	prompt.WriteString(subtitleText)

	return prompt.String()
}