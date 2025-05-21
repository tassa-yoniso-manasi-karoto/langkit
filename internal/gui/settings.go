package gui

import (
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)

func (a *App) InitSettings() error {
	return config.InitConfig("")
}

func (a *App) LoadSettings() (config.Settings, error) {
	return config.LoadSettings()
}

// SaveSettings saves the user settings and updates components that depend on settings
func (a *App) SaveSettings(settings config.Settings) error {
	a.logger.Debug().Msg("Saving settings")
	
	// Save the settings to disk
	err := config.SaveSettings(settings)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to save settings")
		return err
	}
	
	// Update throttler settings
	a.updateThrottlerSettings(settings)
	
	// Trigger registry update with new settings if available
	if a.llmRegistry != nil {
		a.logger.Info().Msg("Triggering LLM registry update with new settings")
		a.llmRegistry.TriggerUpdate(settings)
	}
	
	return nil
}


type STTModelUIInfo struct {
    Name               string `json:"name"`
    DisplayName        string `json:"displayName"` 
    Description        string `json:"description"`
    ProviderName       string `json:"providerName"`
    IsDepreciated      bool   `json:"isDepreciated"`
    IsRecommended      bool   `json:"isRecommended"`
    TakesInitialPrompt bool   `json:"takesInitialPrompt"`
    IsAvailable        bool   `json:"isAvailable"`
}

type STTModelsResponse struct {
	Models    []STTModelUIInfo `json:"models"`
	Names     []string         `json:"names"`
	Available bool             `json:"available"`
	Suggested string           `json:"suggested"`
}

// GetAvailableSTTModelsForUI returns ALL STT models for the UI
func (a *App) GetAvailableSTTModelsForUI() STTModelsResponse {
    models := voice.GetAllSTTModels() 
    
    response := STTModelsResponse{
        Models:    []STTModelUIInfo{},
        Names:     []string{},
        Available: false,
        Suggested: "",
    }
    
    // Count available models
    availableCount := 0
    
    for _, model := range models {
        modelInfo := STTModelUIInfo{
            Name:               model.Name,
            DisplayName:        model.DisplayName,
            Description:        model.Description,
            ProviderName:       model.ProviderName,
            IsDepreciated:      model.IsDepreciated,
            IsRecommended:      model.IsRecommended,
            TakesInitialPrompt: model.TakesInitialPrompt,
            IsAvailable:        model.IsAvailable,
        }
        
        response.Models = append(response.Models, modelInfo)
        response.Names = append(response.Names, model.Name)
        
        if model.IsAvailable {
            availableCount++
        }
    }
    
    // Set available flag based on at least one model being available
    response.Available = availableCount > 0
    
    // Always set the first model as the suggested one (if any models exist)
    if len(response.Names) > 0 {
        response.Suggested = response.Names[0]
    }
    
    return response
}

// RefreshSTTModelsAfterSettingsUpdate explicitly refreshes the STT models
// after settings have been updated to ensure new API keys are recognized
func (a *App) RefreshSTTModelsAfterSettingsUpdate() STTModelsResponse {
    // Force reload of API keys in the voice package
    settings, err := config.LoadSettings()
    if err != nil {
        // a.logger.Error("Failed to load settings for API key refresh", err)
    } else {
        // Explicitly load API keys to voice package
        settings.LoadKeys()
        // a.logger.Info("API keys reloaded for STT model refresh")
    }
    
    // Clear any provider caches in the voice package
    voice.UpdateDefaultFactory()
    
    // Now get the updated models with fresh API keys
    models := voice.GetAllSTTModels()
    
    response := STTModelsResponse{
        Models:    []STTModelUIInfo{},
        Names:     []string{},
        Available: false,
        Suggested: "",
    }
    
    // Count available models
    availableCount := 0
    
    for _, model := range models {
        modelInfo := STTModelUIInfo{
            Name:               model.Name,
            DisplayName:        model.DisplayName,
            Description:        model.Description,
            ProviderName:       model.ProviderName,
            IsDepreciated:      model.IsDepreciated,
            IsRecommended:      model.IsRecommended,
            TakesInitialPrompt: model.TakesInitialPrompt,
            IsAvailable:        model.IsAvailable,
        }
        
        response.Models = append(response.Models, modelInfo)
        response.Names = append(response.Names, model.Name)
        
        if model.IsAvailable {
            availableCount++
        }
    }
    
    // Set available flag based on at least one model being available
    response.Available = availableCount > 0
    
    // Always set the first model as the suggested one (if any models exist)
    if len(response.Names) > 0 {
        response.Suggested = response.Names[0]
    }
    
    return response
}