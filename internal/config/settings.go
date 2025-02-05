package config

import (
	"os"
	"path/filepath"
	
	"github.com/spf13/viper"
	"github.com/adrg/xdg"
)

type Settings struct {
	APIKeys struct {
		Replicate   string `json:"replicate" mapstructure:"replicate"`
		AssemblyAI  string `json:"assemblyAI" mapstructure:"assemblyai"`
		ElevenLabs  string `json:"elevenLabs" mapstructure:"elevenlabs"`
	} `json:"apiKeys" mapstructure:"api_keys"`
	TargetLanguage        string `json:"targetLanguage" mapstructure:"target_language"`
	NativeLanguages      string `json:"nativeLanguages" mapstructure:"native_languages"`
	EnableGlow           bool   `json:"enableGlow" mapstructure:"enable_glow"`
	ShowLogViewerByDefault bool `json:"showLogViewerByDefault" mapstructure:"show_log_viewer_default"`
	MaxLogEntries        int    `json:"maxLogEntries" mapstructure:"max_log_entries"`
}

func getConfigPath() (string, error) {
	configDir := filepath.Join(xdg.ConfigHome, "langkit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
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
	viper.SetDefault("target_language", "")
	viper.SetDefault("native_languages", "")
	viper.SetDefault("enable_glow", true)
	viper.SetDefault("api_keys.replicate", "")
	viper.SetDefault("api_keys.assemblyai", "")
	viper.SetDefault("api_keys.elevenlabs", "")
	viper.SetDefault("target_language", "")
	viper.SetDefault("native_languages", "")
	viper.SetDefault("enable_glow", true)
	viper.SetDefault("show_log_viewer_default", false)
	viper.SetDefault("max_log_entries", 1000)

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
	viper.Set("target_language", settings.TargetLanguage)
	viper.Set("native_languages", settings.NativeLanguages)
	viper.Set("enable_glow", settings.EnableGlow)
	viper.Set("api_keys.replicate", settings.APIKeys.Replicate)
	viper.Set("api_keys.assemblyai", settings.APIKeys.AssemblyAI)
	viper.Set("api_keys.elevenlabs", settings.APIKeys.ElevenLabs)
	viper.Set("target_language", settings.TargetLanguage)
	viper.Set("native_languages", settings.NativeLanguages)
	viper.Set("enable_glow", settings.EnableGlow)
	viper.Set("show_log_viewer_default", settings.ShowLogViewerByDefault)
	viper.Set("max_log_entries", settings.MaxLogEntries)

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
	return settings, nil
}
