package llms

import (
	"sync"
)

var (
	defaultClient     *Client
	defaultClientOnce sync.Once
)

// GetDefaultClient returns the default LLM client instance
func GetDefaultClient() *Client {
	defaultClientOnce.Do(func() {
		defaultClient = NewClient()
	})
	return defaultClient
}

// RegisterDefaultProviders initializes and registers all default providers
// based on environment/configuration settings
func RegisterDefaultProviders() {
	client := GetDefaultClient()
	
	// Register OpenAI provider if configured
	if apiKey := getConfigValue("OPENAI_API_KEY"); apiKey != "" {
		client.RegisterProvider(NewOpenAIProvider(apiKey))
	}
	
	// Register LangChain provider if configured
	// This would depend on your LangChain Go implementation
	if isConfigEnabled("ENABLE_LANGCHAIN") {
		client.RegisterProvider(NewLangChainProvider())
	}
	
	// Register OpenRouter provider if configured
	if apiKey := getConfigValue("OPENROUTER_API_KEY"); apiKey != "" {
		client.RegisterProvider(NewOpenRouterProvider(apiKey))
	}
}

// Helper to get config values - replace with your actual config system
func getConfigValue(key string) string {
	// This is a placeholder - implement with your real config system
	// e.g. return os.Getenv(key) or config.GetString(key)
	return ""
}

// Helper to check if a feature is enabled
func isConfigEnabled(key string) bool {
	// This is a placeholder - implement with your real config system
	// e.g. return config.GetBool(key)
	return false
}