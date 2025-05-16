package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)

// IntermediaryFileMode defines how to handle intermediary files
type IntermediaryFileMode string

const (
	// KeepIntermediaryFiles preserves intermediary files as-is
	KeepIntermediaryFiles IntermediaryFileMode = "keep"
	// RecompressIntermediaryFiles compresses intermediary files to save space
	RecompressIntermediaryFiles IntermediaryFileMode = "recompress"
	// DeleteIntermediaryFiles removes intermediary files immediately
	DeleteIntermediaryFiles IntermediaryFileMode = "delete"
)

type Settings struct {
	APIKeys struct {
		Replicate  string `json:"replicate" mapstructure:"replicate"`
		AssemblyAI string `json:"assemblyAI" mapstructure:"assemblyai"`
		ElevenLabs string `json:"elevenLabs" mapstructure:"elevenlabs"`
		OpenAI     string `json:"openAI" mapstructure:"openai"`
		OpenRouter string `json:"openRouter" mapstructure:"openrouter"`
		Google     string `json:"google" mapstructure:"google"`
	} `json:"apiKeys" mapstructure:"api_keys"`
	TargetLanguage         string `json:"targetLanguage" mapstructure:"target_language"`
	NativeLanguages        string `json:"nativeLanguages" mapstructure:"native_languages"`
	EnableGlow             bool   `json:"enableGlow" mapstructure:"enable_glow"`
	ShowLogViewerByDefault bool   `json:"showLogViewerByDefault" mapstructure:"show_log_viewer_default"`
	MaxLogEntries          int    `json:"maxLogEntries" mapstructure:"max_log_entries"`
	MaxAPIRetries          int    `json:"maxAPIRetries" mapstructure:"max_api_retries"`
	MaxWorkers             int    `json:"maxWorkers" mapstructure:"max_workers"`

	// Timeout settings
	TimeoutSep int `json:"timeoutSep" mapstructure:"timeout_sep"` // seconds
	TimeoutSTT int `json:"timeoutSTT" mapstructure:"timeout_stt"` // seconds
	TimeoutDL  int `json:"timeoutDL" mapstructure:"timeout_dl"`   // seconds

	// NEW: LogViewer settings
	LogViewerVirtualizationThreshold int `json:"logViewerVirtualizationThreshold" mapstructure:"log_viewer_virtualization_threshold"`

	// Event throttling settings
	EventThrottling struct {
		Enabled     bool `json:"enabled" mapstructure:"enabled"`
		MinInterval int  `json:"minInterval" mapstructure:"min_interval"` // milliseconds
		MaxInterval int  `json:"maxInterval" mapstructure:"max_interval"` // milliseconds
	} `json:"eventThrottling" mapstructure:"event_throttling"`
	
	// Internal counters
	CountAppStart    int               `json:"countAppStart" mapstructure:"count_app_start"`
	CountProcessStart int               `json:"countProcessStart" mapstructure:"count_process_start"`
	
	// File handling settings
	IntermediaryFileMode IntermediaryFileMode `json:"intermediaryFileMode" mapstructure:"intermediary_file_mode"`
}

func GetConfigDir() (string, error) {
	configDir := filepath.Join(xdg.ConfigHome, "langkit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return configDir, nil
}

func getConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

func InitConfig(customPath string) error {
	if customPath != "" {
		viper.SetConfigFile(customPath)
	} else {
		configPath, err := getConfigPath()
		if err != nil {
			return err
		}

		viper.SetConfigFile(configPath)
		viper.SetConfigType("yaml")
	}

	viper.SetDefault("api_keys.replicate", "")
	viper.SetDefault("api_keys.assemblyai", "")
	viper.SetDefault("api_keys.elevenlabs", "")
	viper.SetDefault("api_keys.openai", "")
	viper.SetDefault("api_keys.openrouter", "")
	viper.SetDefault("api_keys.google", "")

	viper.SetDefault("target_language", "")
	viper.SetDefault("native_languages", "en,en-US")

	viper.SetDefault("enable_glow", true)
	viper.SetDefault("show_log_viewer_default", false)
	viper.SetDefault("max_log_entries", 10000)
	viper.SetDefault("max_api_retries", 10)
	viper.SetDefault("max_workers", runtime.NumCPU()-2)

	// Set default timeout values
	viper.SetDefault("timeout_sep", 2100) // 35 minutes for voice separation (Demucs, etc.)
	viper.SetDefault("timeout_stt", 90)   // 90 seconds for each subtitle segment transcription 
	viper.SetDefault("timeout_dl", 600)   // 10 minutes for downloading files

	viper.SetDefault("log_viewer_virtualization_threshold", 2000)

	// Default throttling settings
	viper.SetDefault("event_throttling.enabled", true)
	viper.SetDefault("event_throttling.min_interval", 0)   // 0ms = no throttle when quiet
	viper.SetDefault("event_throttling.max_interval", 250) // 250ms max interval
	
	// Default counter values
	viper.SetDefault("count_app_start", 0)
	viper.SetDefault("count_process_start", 0)
	
	// Default intermediary file mode
	viper.SetDefault("intermediary_file_mode", string(KeepIntermediaryFiles))

	// Create config if it doesn't exist
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Save default config
			if err := viper.SafeWriteConfig(); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func SaveSettings(settings Settings) error {
	// Define API key environment variables and their corresponding settings fields
	apiKeyEnvMap := map[string]string{
		"REPLICATE_API_KEY":  settings.APIKeys.Replicate,
		"ASSEMBLYAI_API_KEY": settings.APIKeys.AssemblyAI,
		"ELEVENLABS_API_KEY": settings.APIKeys.ElevenLabs,
		"OPENAI_API_KEY":     settings.APIKeys.OpenAI,
		"OPENROUTER_API_KEY": settings.APIKeys.OpenRouter,
		"GOOGLE_API_KEY":     settings.APIKeys.Google,
	}

	// For each API key, only save to config if it's not the same as the environment variable
	// This preserves user privacy by not duplicating environment variables in the config file
	for envName, settingValue := range apiKeyEnvMap {
		envValue := os.Getenv(envName)
		configKey := "api_keys." + strings.ToLower(strings.TrimSuffix(envName, "_API_KEY"))

		if envValue != "" {
			if settingValue == envValue {
				// If the setting matches the environment variable, use empty string in config
				// This prevents saving environment variable values to the config file
				viper.Set(configKey, "")
			} else {
				// If different, save the setting value (user explicitly changed it in the UI)
				viper.Set(configKey, settingValue)
			}
		} else {
			// No environment variable, just save the setting
			viper.Set(configKey, settingValue)
		}
	}

	// Set all other non-sensitive settings
	viper.Set("target_language", settings.TargetLanguage)
	viper.Set("native_languages", settings.NativeLanguages)
	viper.Set("enable_glow", settings.EnableGlow)
	viper.Set("show_log_viewer_default", settings.ShowLogViewerByDefault)
	viper.Set("max_log_entries", settings.MaxLogEntries)
	viper.Set("max_api_retries", settings.MaxAPIRetries)
	viper.Set("max_workers", settings.MaxWorkers)

	// Save event throttling settings
	viper.Set("event_throttling.enabled", settings.EventThrottling.Enabled)
	viper.Set("event_throttling.min_interval", settings.EventThrottling.MinInterval)
	viper.Set("event_throttling.max_interval", settings.EventThrottling.MaxInterval)

	// Ensure config path exists
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Write config to file
	viper.SetConfigFile(configPath)
	return viper.WriteConfig()
}

func LoadSettings() (Settings, error) {
	var settings Settings
	if err := viper.Unmarshal(&settings); err != nil {
		return Settings{}, err
	}
	settings.LoadKeys()
	return settings, nil
}

// Apply API keys from config or environment
func (settings Settings) LoadKeys() {
	providers := []string{"replicate", "assemblyai", "elevenlabs", "openai", "openrouter", "google"}

	for idx, name := range providers {
		var key string
		switch idx {
		case 0:
			key = settings.APIKeys.Replicate
		case 1:
			key = settings.APIKeys.AssemblyAI
		case 2:
			key = settings.APIKeys.ElevenLabs
		case 3:
			key = settings.APIKeys.OpenAI
		case 4:
			key = settings.APIKeys.OpenRouter
		case 5:
			key = settings.APIKeys.Google
		}
		if s := os.Getenv(strings.ToUpper(name) + "_API_KEY"); s != "" {
			key = s
		}
		voice.APIKeys.Store(name, key)
	}
}
