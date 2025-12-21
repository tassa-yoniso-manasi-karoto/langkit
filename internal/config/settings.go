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
		ElevenLabs string `json:"elevenLabs" mapstructure:"elevenlabs"`
		OpenAI     string `json:"openAI" mapstructure:"openai"`
		OpenRouter string `json:"openRouter" mapstructure:"openrouter"`
		Google     string `json:"google" mapstructure:"google"`
	} `json:"apiKeys" mapstructure:"api_keys"`
	TargetLanguage         string `json:"targetLanguage" mapstructure:"target_language"`
	NativeLanguages        string `json:"nativeLanguages" mapstructure:"native_languages"`
	LiteMode               bool   `json:"liteMode" mapstructure:"lite_mode"`
	ShowLogViewerByDefault bool   `json:"showLogViewerByDefault" mapstructure:"show_log_viewer_default"`
	MaxLogEntries          int    `json:"maxLogEntries" mapstructure:"max_log_entries"`
	MaxAPIRetries          int    `json:"maxAPIRetries" mapstructure:"max_api_retries"`
	MaxWorkers             int    `json:"maxWorkers" mapstructure:"max_workers"`

	// Timeout settings
	TimeoutSep int `json:"timeoutSep" mapstructure:"timeout_sep"` // seconds
	TimeoutSTT int `json:"timeoutSTT" mapstructure:"timeout_stt"` // seconds
	TimeoutDL  int `json:"timeoutDL" mapstructure:"timeout_dl"`   // seconds

	// Demucs settings
	DemucsMaxSegmentMinutes int `json:"demucsMaxSegmentMinutes" mapstructure:"demucs_max_segment_minutes"` // max minutes per segment to avoid GPU OOM

	// NEW: LogViewer settings
	LogViewerVirtualizationThreshold int `json:"logViewerVirtualizationThreshold" mapstructure:"log_viewer_virtualization_threshold"`

	// Event throttling settings
	EventThrottling struct {
		Enabled     bool `json:"enabled" mapstructure:"enabled"`
		MinInterval int  `json:"minInterval" mapstructure:"min_interval"` // milliseconds
		MaxInterval int  `json:"maxInterval" mapstructure:"max_interval"` // milliseconds
	} `json:"eventThrottling" mapstructure:"event_throttling"`
	
	// File handling settings
	IntermediaryFileMode IntermediaryFileMode `json:"intermediaryFileMode" mapstructure:"intermediary_file_mode"`
	DeleteResumptionFiles bool               `json:"deleteResumptionFiles" mapstructure:"delete_resumption_files"`

	// WebAssembly settings
	UseWasm           bool   `json:"useWasm" mapstructure:"use_wasm"`
	WasmSizeThreshold int    `json:"wasmSizeThreshold" mapstructure:"wasm_size_threshold"`
	ForceWasmMode     string `json:"forceWasmMode" mapstructure:"force_wasm_mode"`

	// Paths for binaries
	FFmpegPath    string `json:"ffmpegPath" mapstructure:"ffmpeg_path"`
	MediaInfoPath string `json:"mediainfoPath" mapstructure:"mediainfo_path"`

	// Custom endpoints for local inference
	CustomEndpoints struct {
		STT struct {
			Enabled  bool   `json:"enabled" mapstructure:"enabled"`
			Endpoint string `json:"endpoint" mapstructure:"endpoint"`
			Model    string `json:"model" mapstructure:"model"`
		} `json:"stt" mapstructure:"stt"`
		LLM struct {
			Enabled  bool   `json:"enabled" mapstructure:"enabled"`
			Endpoint string `json:"endpoint" mapstructure:"endpoint"`
			Model    string `json:"model" mapstructure:"model"`
		} `json:"llm" mapstructure:"llm"`
	} `json:"customEndpoints" mapstructure:"custom_endpoints"`

	// Version tracking for changelog display
	LastSeenVersion           string `json:"lastSeenVersion" mapstructure:"last_seen_version"`
	ChangelogDisplayFrequency string `json:"changelogDisplayFrequency" mapstructure:"changelog_display_frequency"` // "all", "medium_major", "major_only"
}

func GetConfigDir() (string, error) {
	configDir := filepath.Join(xdg.ConfigHome, "langkit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return configDir, nil
}

func GetToolsDir() (string, error) {
	toolsDir := filepath.Join(xdg.DataHome, "langkit", "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return "", err
	}
	return toolsDir, nil
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
	viper.SetDefault("api_keys.elevenlabs", "")
	viper.SetDefault("api_keys.openai", "")
	viper.SetDefault("api_keys.openrouter", "")
	viper.SetDefault("api_keys.google", "")

	viper.SetDefault("target_language", "")
	viper.SetDefault("native_languages", "en")

	viper.SetDefault("lite_mode", false)
	viper.SetDefault("show_log_viewer_default", false)
	viper.SetDefault("max_log_entries", 10000)
	viper.SetDefault("max_api_retries", 10)
	viper.SetDefault("max_workers", runtime.NumCPU()-2)

	// Set default timeout values
	viper.SetDefault("timeout_sep", 2100) // 35 minutes for voice separation (Demucs, etc.)
	viper.SetDefault("timeout_stt", 90)   // 90 seconds for each subtitle segment transcription
	viper.SetDefault("timeout_dl", 600)   // 10 minutes for downloading files

	// Demucs settings - split long files to avoid GPU OOM (output tensor must fit in VRAM)
	viper.SetDefault("demucs_max_segment_minutes", 15) // 15 min segments: at least 4GB VRAM needed

	viper.SetDefault("log_viewer_virtualization_threshold", 500)

	// Default throttling settings
	viper.SetDefault("event_throttling.enabled", true)
	viper.SetDefault("event_throttling.min_interval", 0)   // 0ms = no throttle when quiet
	viper.SetDefault("event_throttling.max_interval", 250) // 250ms max interval
	
	// Default intermediary file mode
	viper.SetDefault("intermediary_file_mode", string(RecompressIntermediaryFiles))
	viper.SetDefault("delete_resumption_files", false)

	// Default WebAssembly settings
	viper.SetDefault("use_wasm", true)
	viper.SetDefault("wasm_size_threshold", 1)
	viper.SetDefault("force_wasm_mode", "enabled")

	viper.SetDefault("ffmpeg_path", "")
	viper.SetDefault("mediainfo_path", "")

	// Custom endpoints defaults
	viper.SetDefault("custom_endpoints.stt.enabled", false)
	viper.SetDefault("custom_endpoints.stt.endpoint", "http://localhost:8080/v1/audio/transcriptions")
	viper.SetDefault("custom_endpoints.stt.model", "")
	viper.SetDefault("custom_endpoints.llm.enabled", false)
	viper.SetDefault("custom_endpoints.llm.endpoint", "http://localhost:11434/v1/chat/completions")
	viper.SetDefault("custom_endpoints.llm.model", "")

	// Version tracking defaults
	viper.SetDefault("last_seen_version", "")
	viper.SetDefault("changelog_display_frequency", "minor_major")

	// Get the config path
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Create empty config file if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Ensure directory exists
		configDir := filepath.Dir(configPath)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return err
		}

		// Create empty file (like touch command)
		file, err := os.Create(configPath)
		if err != nil {
			return err
		}
		file.Close()

		// Now write defaults to the newly created file
		if err := viper.WriteConfig(); err != nil {
			return err
		}
	}

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}

func SaveSettings(settings Settings) error {
	// Define API key environment variables and their corresponding settings fields
	apiKeyEnvMap := map[string]string{
		"REPLICATE_API_KEY":  settings.APIKeys.Replicate,
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
	viper.Set("lite_mode", settings.LiteMode)
	viper.Set("show_log_viewer_default", settings.ShowLogViewerByDefault)
	viper.Set("max_log_entries", settings.MaxLogEntries)
	viper.Set("max_api_retries", settings.MaxAPIRetries)
	viper.Set("max_workers", settings.MaxWorkers)
	viper.Set("timeout_sep", settings.TimeoutSep)
	viper.Set("timeout_stt", settings.TimeoutSTT)
	viper.Set("timeout_dl", settings.TimeoutDL)
	viper.Set("demucs_max_segment_minutes", settings.DemucsMaxSegmentMinutes)
	viper.Set("log_viewer_virtualization_threshold", settings.LogViewerVirtualizationThreshold)

	// Save event throttling settings
	viper.Set("event_throttling.enabled", settings.EventThrottling.Enabled)
	viper.Set("event_throttling.min_interval", settings.EventThrottling.MinInterval)
	viper.Set("event_throttling.max_interval", settings.EventThrottling.MaxInterval)
	
	// Save file handling settings
	viper.Set("intermediary_file_mode", string(settings.IntermediaryFileMode))
	viper.Set("delete_resumption_files", settings.DeleteResumptionFiles)

	// Save WebAssembly settings
	viper.Set("use_wasm", settings.UseWasm)
	viper.Set("wasm_size_threshold", settings.WasmSizeThreshold)
	viper.Set("force_wasm_mode", settings.ForceWasmMode)

	// Save binary paths
	viper.Set("ffmpeg_path", filepath.ToSlash(settings.FFmpegPath))
	viper.Set("mediainfo_path", filepath.ToSlash(settings.MediaInfoPath))

	// Save custom endpoint settings
	viper.Set("custom_endpoints.stt.enabled", settings.CustomEndpoints.STT.Enabled)
	viper.Set("custom_endpoints.stt.endpoint", settings.CustomEndpoints.STT.Endpoint)
	viper.Set("custom_endpoints.stt.model", settings.CustomEndpoints.STT.Model)
	viper.Set("custom_endpoints.llm.enabled", settings.CustomEndpoints.LLM.Enabled)
	viper.Set("custom_endpoints.llm.endpoint", settings.CustomEndpoints.LLM.Endpoint)
	viper.Set("custom_endpoints.llm.model", settings.CustomEndpoints.LLM.Model)

	// Save version tracking settings
	viper.Set("last_seen_version", settings.LastSeenVersion)
	viper.Set("changelog_display_frequency", settings.ChangelogDisplayFrequency)

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
	
	// Update settings struct with environment variables
	applyEnvironmentVariables(&settings)
	
	// Load keys into the voice package
	settings.LoadKeys()
	
	return settings, nil
}

// Apply environment variables directly to settings struct
func applyEnvironmentVariables(settings *Settings) {
	if key := os.Getenv("REPLICATE_API_KEY"); key != "" {
		settings.APIKeys.Replicate = key
	}
	if key := os.Getenv("ELEVENLABS_API_KEY"); key != "" {
		settings.APIKeys.ElevenLabs = key
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		settings.APIKeys.OpenAI = key
	}
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		settings.APIKeys.OpenRouter = key
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		settings.APIKeys.Google = key
	}
}

// Apply API keys from config or environment
func (settings Settings) LoadKeys() {
	providers := []string{"replicate", "elevenlabs", "openai", "openrouter", "google"}

	for idx, name := range providers {
		var key string
		switch idx {
		case 0:
			key = settings.APIKeys.Replicate
		case 1:
			key = settings.APIKeys.ElevenLabs
		case 2:
			key = settings.APIKeys.OpenAI
		case 3:
			key = settings.APIKeys.OpenRouter
		case 4:
			key = settings.APIKeys.Google
		}
		if s := os.Getenv(strings.ToUpper(name) + "_API_KEY"); s != "" {
			key = s
		}
		voice.APIKeys.Store(name, key)
	}

	// Store custom endpoint settings for the voice package
	voice.CustomEndpoints.Store("stt_enabled", settings.CustomEndpoints.STT.Enabled)
	voice.CustomEndpoints.Store("stt_endpoint", settings.CustomEndpoints.STT.Endpoint)
	voice.CustomEndpoints.Store("stt_model", settings.CustomEndpoints.STT.Model)

	// Set demucs max segment setting
	if settings.DemucsMaxSegmentMinutes > 0 {
		voice.DemucsMaxSegmentMinutes = settings.DemucsMaxSegmentMinutes
	}
}
