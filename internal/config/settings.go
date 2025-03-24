package config

import (
	"os"
	"strings"
	"runtime"
	"path/filepath"
	
	"github.com/spf13/viper"
	"github.com/adrg/xdg"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)

type Settings struct {
	APIKeys struct {
		Replicate  string `json:"replicate" mapstructure:"replicate"`
		AssemblyAI string `json:"assemblyAI" mapstructure:"assemblyai"`
		ElevenLabs string `json:"elevenLabs" mapstructure:"elevenlabs"`
		OpenAI     string `json:"openAI" mapstructure:"openai"`
	} `json:"apiKeys" mapstructure:"api_keys"`
	TargetLanguage         string `json:"targetLanguage" mapstructure:"target_language"`
	NativeLanguages        string `json:"nativeLanguages" mapstructure:"native_languages"`
	EnableGlow             bool   `json:"enableGlow" mapstructure:"enable_glow"`
	ShowLogViewerByDefault bool   `json:"showLogViewerByDefault" mapstructure:"show_log_viewer_default"`
	MaxLogEntries          int    `json:"maxLogEntries" mapstructure:"max_log_entries"`
	MaxAPIRetries          int    `json:"maxAPIRetries" mapstructure:"max_api_retries"`
	MaxWorkers             int    `json:"maxWorkers" mapstructure:"max_workers"`
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
	
	viper.SetDefault("target_language", "")
	viper.SetDefault("native_languages", "en,en-US")
	
	viper.SetDefault("enable_glow", true)
	viper.SetDefault("show_log_viewer_default", false)
	viper.SetDefault("max_log_entries", 10000)
	viper.SetDefault("max_api_retries", 10)
	viper.SetDefault("max_workers", runtime.NumCPU()-2)

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
	// Update Viper config
	viper.Set("api_keys.replicate", settings.APIKeys.Replicate)
	viper.Set("api_keys.assemblyai", settings.APIKeys.AssemblyAI)
	viper.Set("api_keys.elevenlabs", settings.APIKeys.ElevenLabs)
	viper.Set("api_keys.openai", settings.APIKeys.OpenAI)
	
	viper.Set("target_language", settings.TargetLanguage)
	viper.Set("native_languages", settings.NativeLanguages)
	
	viper.Set("enable_glow", settings.EnableGlow)
	viper.Set("show_log_viewer_default", settings.ShowLogViewerByDefault)
	viper.Set("max_log_entries", settings.MaxLogEntries)
	viper.Set("max_api_retries", settings.MaxAPIRetries)
	viper.Set("max_workers", settings.MaxWorkers)

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
	providers := []string{"replicate", "assemblyai", "elevenlabs", "openai"}
	
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
		}
		if s := os.Getenv(strings.ToUpper(name) + "_API_KEY"); s != "" {
			key = s
		}
		voice.APIKeys.Store(name, key)
	}
}

