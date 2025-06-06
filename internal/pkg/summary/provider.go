package summary

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

const splitter = "--- Subtitle Content to Summarize ---"

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
	if strings.TrimSpace(options.CustomPrompt) != "" {
		return options.CustomPrompt + "\n\n--- Subtitle Content ---\n" + subtitleText
	}

	var prompt strings.Builder
	prompt.WriteString("Generate a thorough, narrative plot summary of the content provided below. ")
	prompt.WriteString("Your task is to synthesize the key events, character interactions, and significant dialogue into a coherent story. ")
	prompt.WriteString("Focus on what is needed for understanding the plot's progression & development in detail.")
	prompt.WriteString("Do not simply list events or transcribe dialogue. Instead, connect the dots for the reader into a summary. ")
	prompt.WriteString("Your response must contain no preambles, no commentary, no thematic analysis, and no meta-statements about the content itself. ")

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

	// Add symbolic emphasis instructions if requested
	if options.UseSymbolicEmphasis {
		prompt.WriteString("Format the summary using bold letters (ONLY the bold letters) ")
		prompt.WriteString("from the 'Mathematical Alphanumeric Symbols' subset of UTF-8 to fairly sparingly add emphasis ")
		prompt.WriteString("to important key points and relevant character names. DO NOT USE MARKDOWN OR HTML. ")
		prompt.WriteString("For example, 'Bob has been tasked by someone to infiltrate the Gang' would become ")
		prompt.WriteString("'Bob has 𝗯𝗲𝗲𝗻 𝘁𝗮𝘀𝗸𝗲𝗱 𝗯𝘆 𝘀𝗼𝗺𝗲𝗼𝗻𝗲 𝘁𝗼 𝗶𝗻𝗳𝗶𝗹𝘁𝗿𝗮𝘁𝗲 𝘁𝗵𝗲 𝗚𝗮𝗻𝗴'.")
	}

	prompt.WriteString("\n\n" + splitter + "\n")
	prompt.WriteString(subtitleText)

	return prompt.String()
}