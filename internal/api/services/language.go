package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/translitkit"
	"github.com/tassa-yoniso-manasi-karoto/translitkit/common"
)

// Compile-time check that LanguageService implements api.Service
var _ api.Service = (*LanguageService)(nil)

// LanguageService implements the WebRPC LanguageService interface
type LanguageService struct {
	logger  zerolog.Logger
	handler http.Handler
}

// NewLanguageService creates a new language service
func NewLanguageService(logger zerolog.Logger) *LanguageService {
	svc := &LanguageService{
		logger: logger,
	}
	
	// Create the WebRPC handler
	svc.handler = generated.NewLanguageServiceServer(svc)
	
	return svc
}

// Name implements api.Service
func (s *LanguageService) Name() string {
	return "LanguageService"
}

// Handler implements api.Service
func (s *LanguageService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *LanguageService) Description() string {
	return "Language validation and processing service"
}

// Implement generated.LanguageService interface

func (s *LanguageService) ValidateLanguage(ctx context.Context, validation *generated.LanguageValidation) (*generated.ValidationResponse, error) {
	resp := &generated.ValidationResponse{
		Valid: false,
	}
	
	if validation.Tag == "" {
		errorMsg := "provided tag is empty"
		resp.Error = &errorMsg
		return resp, nil
	}

	tags := core.TagsStr2TagsArr(validation.Tag)

	if validation.Single && len(tags) > 1 {
		errorMsg := "more than one tag was provided"
		resp.Error = &errorMsg
		return resp, nil
	}

	// Filter out empty strings
	var nonEmptyTags []string
	for _, tag := range tags {
		if tag != "" {
			nonEmptyTags = append(nonEmptyTags, strings.TrimSpace(tag))
		}
	}

	if len(nonEmptyTags) == 0 {
		errorMsg := "no valid tags provided"
		resp.Error = &errorMsg
		return resp, nil
	}

	// For single tag validation
	if len(nonEmptyTags) == 1 {
		std, isValid, err := validateLanguageTag(nonEmptyTags[0])
		if err != nil {
			errorMsg := err.Error()
			resp.Error = &errorMsg
			return resp, nil
		}
		return &generated.ValidationResponse{
			Valid:       isValid,
			StandardTag: std,
		}, nil
	}

	// For multiple tags, use the original logic
	langs, err := core.ParseLanguageTags(nonEmptyTags)
	if err != nil {
		errorMsg := err.Error()
		resp.Error = &errorMsg
		return resp, nil
	}
	
	if len(langs) == 0 {
		errorMsg := "no valid language tags found"
		resp.Error = &errorMsg
		return resp, nil
	}
	
	std := langs[0].Part3
	if langs[0].Subtag != "" {
		std += "-" + langs[0].Subtag
	}

	return &generated.ValidationResponse{
		Valid:       true,
		StandardTag: std,
	}, nil
}

func (s *LanguageService) GetLanguageRequirements(ctx context.Context, languageTag string) (*generated.LanguageRequirements, error) {
	resp := &generated.LanguageRequirements{
		IsValid: false,
	}

	// Use the internal validation helper
	std, isValid, err := validateLanguageTag(languageTag)
	if err != nil {
		errorMsg := err.Error()
		resp.Error = &errorMsg
		return resp, nil
	}

	resp.StandardTag = std
	resp.IsValid = isValid

	// TODO slap translitkit get default scheme for lang & remove hardcoded maps
	// Languages that require Docker for linguistic processing
	dockerRequiredLanguages := map[string]bool{
		"jpn": true, "hin": true, "mar": true, "ben": true,
		"tam": true, "tel": true, "kan": true, "mal": true,
		"guj": true, "pan": true, "ori": true, "urd": true,
	}

	// Languages that require Internet for linguistic processing
	internetRequiredLanguages := map[string]bool{
		"tha": true, "jpn": true, "hin": true, "mar": true,
		"ben": true, "tam": true, "tel": true, "kan": true,
		"mal": true, "guj": true, "pan": true, "ori": true,
		"urd": true,
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

	return resp, nil
}

func (s *LanguageService) NeedsTokenization(ctx context.Context, language string) (bool, error) {
	b, _ := translitkit.NeedsTokenization(language)
	return b, nil
}

func (s *LanguageService) GetRomanizationStyles(ctx context.Context, languageCode string) (*generated.RomanizationStylesResponse, error) {
	resp := &generated.RomanizationStylesResponse{
		DockerEngine: dockerutil.DockerBackendName(),
		Schemes:      []*generated.RomanizationScheme{},
	}

	// Get available schemes for the language
	schemes, err := common.GetSchemes(languageCode)
	if err != nil {
		if err == common.ErrNoSchemesRegistered {
			s.logger.Warn().Msgf("%v \"%s\"", err, languageCode)
			// Return empty response instead of error
			return resp, nil
		} else {
			s.logger.Error().
				Err(err).
				Str("lang", languageCode).
				Msg("Failed to get romanization schemes")
			return nil, err
		}
	}
	
	for _, scheme := range schemes {
		if scheme.NeedsDocker {
			resp.NeedsDocker = true
		}
		if scheme.NeedsScraper {
			resp.NeedsScraper = true
		}
	}

	if resp.NeedsDocker {
		if err := dockerutil.EngineIsReachable(); err != nil {
			s.logger.Warn().
				Err(err).
				Str("lang", languageCode).
				Msg("Docker is required but not available")
			resp.DockerUnreachable = true
		}
	}

	// Convert schemes to response format
	resp.Schemes = make([]*generated.RomanizationScheme, len(schemes))
	for i, scheme := range schemes {
		resp.Schemes[i] = &generated.RomanizationScheme{
			Name:        scheme.Name,
			Description: scheme.Description,
			Provider:    scheme.Provider,
		}
	}
	
	return resp, nil
}

// Internal helper function (reused from lang.go)
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