package gui

import (
	"fmt"
	"strings"

	//"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/translitkit"
	//"github.com/tassa-yoniso-manasi-karoto/translitkit/common"
)

// LanguageRequirements holds the requirements for a specific language
type LanguageRequirements struct {
	StandardTag      string `json:"standardTag"`
	IsValid          bool   `json:"isValid"`
	RequiresDocker   bool   `json:"requiresDocker"`
	RequiresInternet bool   `json:"requiresInternet"`
	Error            string `json:"error,omitempty"`
}

// validateLanguageTag is an internal helper to validate and standardize a language tag
func validateLanguageTag(tag string) (standardTag string, isValid bool, err error) {
	if tag == "" {
		return "", false, fmt.Errorf("language tag is empty")
	}

	// Validate the language tag using core
	langs, err := core.ParseLanguageTags([]string{strings.TrimSpace(tag)})
	if err != nil {
		return "", false, err
	}

	if len(langs) == 0 {
		return "", false, fmt.Errorf("invalid language tag")
	}

	// Get the standardized tag
	std := langs[0].Part3
	if langs[0].Subtag != "" {
		std += "-" + langs[0].Subtag
	}

	return std, true, nil
}

// GetLanguageRequirements validates a language tag and returns its requirements
func (a *App) GetLanguageRequirements(languageTag string) LanguageRequirements {
	resp := LanguageRequirements{
		IsValid: false,
	}

	// Use the internal validation helper
	std, isValid, err := validateLanguageTag(languageTag)
	if err != nil {
		resp.Error = err.Error()
		return resp
	}

	resp.StandardTag = std
	resp.IsValid = isValid

	// TODO slap translitkit get default scheme for lang & remove hardcoded maps
	// Languages that require Docker for linguistic processing
	dockerRequiredLanguages := map[string]bool{
		"jpn": true, // Japanese
		"hin": true, // Hindi
		"mar": true, // Marathi
		"ben": true, // Bengali
		"tam": true, // Tamil
		"tel": true, // Telugu
		"kan": true, // Kannada
		"mal": true, // Malayalam
		"guj": true, // Gujarati
		"pan": true, // Punjabi
		"ori": true, // Odia
		"urd": true, // Urdu
	}

	// Languages that require Internet for linguistic processing
	internetRequiredLanguages := map[string]bool{
		"tha": true, // Thai
		"jpn": true, // Japanese
		"hin": true, // Hindi
		"mar": true, // Marathi
		"ben": true, // Bengali
		"tam": true, // Tamil
		"tel": true, // Telugu
		"kan": true, // Kannada
		"mal": true, // Malayalam
		"guj": true, // Gujarati
		"pan": true, // Punjabi
		"ori": true, // Odia
		"urd": true, // Urdu
	}

	// Check requirements based on the ISO 639-3 code
	for code := range dockerRequiredLanguages {
		if strings.HasPrefix(std, code) {
			resp.RequiresDocker = true
			break
		}
	}

	for code := range internetRequiredLanguages {
		if strings.HasPrefix(std, code) {
			resp.RequiresInternet = true
			break
		}
	}

	return resp
}

type LanguageCheckResponse struct {
	StandardTag string `json:"standardTag"`
	IsValid     bool   `json:"isValid"`
	Error       string `json:"error,omitempty"`
}

func (a *App) ValidateLanguageTag(tagsString string, maxOne bool) LanguageCheckResponse {
	resp := LanguageCheckResponse{
		IsValid: false,
	}
	if tagsString == "" {
		resp.Error = "provided tagsString is empty"
		return resp
	}

	tags := core.TagsStr2TagsArr(tagsString)

	if maxOne && len(tags) > 1 {
		resp.Error = "more than one tag was provided"
		return resp
	}

	// Filter out empty strings
	var nonEmptyTags []string
	for _, tag := range tags {
		if tag != "" {
			nonEmptyTags = append(nonEmptyTags, strings.TrimSpace(tag))
		}
	}

	if len(nonEmptyTags) == 0 {
		resp.Error = "no valid tags provided"
		return resp
	}

	// For single tag validation, we can use the internal helper
	if len(nonEmptyTags) == 1 {
		std, isValid, err := validateLanguageTag(nonEmptyTags[0])
		if err != nil {
			resp.Error = err.Error()
			return resp
		}
		return LanguageCheckResponse{
			IsValid:     isValid,
			StandardTag: std,
		}
	}

	// For multiple tags, use the original logic
	langs, err := core.ParseLanguageTags(nonEmptyTags)
	if err != nil {
		resp.Error = err.Error()
		return resp
	}
	
	if len(langs) == 0 {
		resp.Error = "no valid language tags found"
		return resp
	}
	
	std := langs[0].Part3
	if langs[0].Subtag != "" {
		std += "-" + langs[0].Subtag
	}

	return LanguageCheckResponse{
		IsValid:     true,
		StandardTag: std,
	}
}

// NeedsTokenization checks if a language needs tokenization
func (a *App) NeedsTokenization(language string) bool {
	b, _ := translitkit.NeedsTokenization(language)
	return b
}

type RomanizationScheme struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Provider    string `json:"provider"`
}

type RomanizationStylesResponse struct {
	Schemes           []RomanizationScheme `json:"schemes"`
	DockerUnreachable bool                 `json:"dockerUnreachable"`
	DockerEngine      string               `json:"dockerEngine"`
	NeedsDocker       bool                 `json:"needsDocker"`
	NeedsScraper      bool                 `json:"needsScraper"`
}
/*
func (a *App) GetRomanizationStyles(languageCode string) (RomanizationStylesResponse, error) {
	resp := RomanizationStylesResponse{DockerEngine: dockerutil.DockerBackendName()}

	// Get available schemes for the language
	schemes, err := common.GetSchemes(languageCode)
	if err != nil {
		if err == common.ErrNoSchemesRegistered {
			handler.ZeroLog().Warn().Msgf("%v \"%s\"", err, languageCode)
		} else {
			handler.ZeroLog().Error().
				Err(err).
				Str("lang", languageCode).
				Msg("Failed to get romanization schemes")
		}
		return resp, err
	}
	for _, scheme := range schemes {
		if scheme.NeedsDocker {
			resp.NeedsDocker = true
			break
		}
	}
	for _, scheme := range schemes {
		if scheme.NeedsScraper {
			resp.NeedsScraper = true
			break
		}
	}

	if resp.NeedsDocker {
		if err := dockerutil.EngineIsReachable(); err != nil {
			handler.ZeroLog().Warn().
				Err(err).
				Str("lang", languageCode).
				Msg("Docker is required but not available")

			resp.DockerUnreachable = true
		}
	}

	// Convert schemes to resp format
	resp.Schemes = make([]RomanizationScheme, len(schemes))
	for i, scheme := range schemes {
		resp.Schemes[i] = RomanizationScheme{
			Name:        scheme.Name,
			Description: scheme.Description,
			Provider:    scheme.Provider,
		}
	}
	return resp, nil
}*/