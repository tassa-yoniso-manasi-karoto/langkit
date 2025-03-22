package gui

import (
	"time"
	"context"
	
	"fmt"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
)

func (a *App) SendProcessingRequest(req ProcessRequest) {
	defer func() {
		if r := recover(); r != nil {
			exitOnError(fmt.Errorf("panic in SendProcessingRequest: %v", r))
		}
	}()

	processCtx, cancel := context.WithCancel(a.ctx)
	a.procCancel = cancel
	defer cancel()
	
	handler.ResetProgress()

	tsk := core.NewTask(handler)
	
	a.translateReq2Tsk(req, tsk)
	
	// FIXME will be problematic if enhance include as sub-task related to subs like translit
	if tsk.Mode != core.Enhance {
		settings, err := config.LoadSettings()
		if err != nil {
			tsk.Handler.LogErr(err, core.AbortAllTasks, "failed to load settings")
			return
		}
		
		if req.LanguageCode == "" || settings.NativeLanguages == "" {
			tsk.Handler.Log(core.Error, core.AbortAllTasks,
				"No target language was passed or no native languages is configured in settings")
			return
		}
		tsk.Langs = append([]string{req.LanguageCode}, core.TagsStr2TagsArr(settings.NativeLanguages)...)
		if procErr := tsk.PrepareLangs(); procErr != nil {
			tsk.Handler.ZeroLog().Error().Err(procErr.Err).
				Msg("PrepareLangs failed")
			return
		}
	}
	
	tsk.MediaSourceFile = req.Path
	
	tsk.Handler.ZeroLog().Info().
		Str("file", tsk.MediaSourceFile).
		Int("mode", int(tsk.Mode)).
		Bool("MergeOutputFiles", tsk.MergeOutputFiles).
		Msg("Starting processing")

	pp.Println(req)
	
	tsk.Routing(processCtx)
}


func (a *App) CancelProcessing() {
	handler.ZeroLog().Debug().
		Msg("Calling procCancel()")
	a.procCancel()
}


// ProcessRequest represents the incoming request from the frontend
type ProcessRequest struct {
	Path             string          `json:"path"`
	SelectedFeatures map[string]bool `json:"selectedFeatures"`
	Options          FeatureOptions  `json:"options"`
	LanguageCode     string          `json:"languageCode"`
	AudioTrackIndex  int             `json:"audioTrackIndex,omitempty"`
}

type FeatureOptions struct {
	// Dynamic options map for all features based on featureModel.ts
	Options map[string]map[string]interface{} `json:"options"`
}



func (a *App) translateReq2Tsk(request ProcessRequest, tsk *core.Task) {
	// Configure based on selected features starting from the most specific,
	// restricted processing mode to the most general, multipurpose in order to
	// have the correct tsk.Mode at the end to pass on downstream.
	
	// internally tsk.UseAudiotrack refers to first track as the track whose index is 0
	request.AudioTrackIndex -= 1
	
	// Set audio track if specified
	if request.AudioTrackIndex >= 0 {
		tsk.UseAudiotrack = request.AudioTrackIndex
		tsk.Handler.ZeroLog().Debug().
			Int("UseAudiotrack", tsk.UseAudiotrack).
			Msg("Set audio track index")
	}
	
	// Initialize feature groups mapping to track which features belong to which groups
	// featureGroups := map[string][]string{
	// 	"merge": {"dubtitles", "voiceEnhancing", "subtitleRomanization", "selectiveTransliteration", "subtitleTokenization"},
	// 	"subtitle": {"subtitleRomanization", "selectiveTransliteration", "subtitleTokenization"},
	// }
	
	// Check all enabled features for mergeOutputFiles=true
	tsk.MergeOutputFiles = false
	for feature, enabled := range request.SelectedFeatures {
		if !enabled {
			continue
		}
		
		featureOpts, ok := request.Options.Options[feature]
		if !ok {
			continue
		}
		
		if mergeOutput, ok := featureOpts["mergeOutputFiles"]; ok {
			if shouldMerge, ok := mergeOutput.(bool); ok && shouldMerge {
				tsk.MergeOutputFiles = true
				
				// Get the mergingFormat from this feature
				if mergingFormat, ok := featureOpts["mergingFormat"]; ok {
					if format, ok := mergingFormat.(string); ok {
						tsk.MergingFormat = format
						
						tsk.Handler.ZeroLog().Debug().
							Str("feature", feature).
							Str("mergingFormat", format).
							Msg("Enabling merge output files")
					}
				}
				
				// We found a feature with mergeOutputFiles=true, no need to check others
				break
			}
		}
	}
	
	// Initialize subtitle processing options
	// We'll capture all transliteration-related features first
	var subtitleFeatures []string
	if request.SelectedFeatures["subtitleRomanization"] {
		subtitleFeatures = append(subtitleFeatures, "subtitleRomanization")
	}
	if request.SelectedFeatures["selectiveTransliteration"] {
		subtitleFeatures = append(subtitleFeatures, "selectiveTransliteration")
	}
	if request.SelectedFeatures["subtitleTokenization"] {
		subtitleFeatures = append(subtitleFeatures, "subtitleTokenization")
	}
	
	// If any subtitle feature is selected, set up the transliteration mode
	if len(subtitleFeatures) > 0 {
		tsk.Mode = core.Translit
		tsk.WantTranslit = true
		
		// Initialize TranslitTypes to ensure we know which outputs to generate
		tsk.TranslitTypes = []core.TranslitType{}
		
		// Process common provider settings from subtitleRomanization
		// (or from other features if romanization isn't selected)
		var providerFeature string
		if request.SelectedFeatures["subtitleRomanization"] {
			providerFeature = "subtitleRomanization"
		} else if request.SelectedFeatures["subtitleTokenization"] {
			providerFeature = "subtitleTokenization"
		} else if request.SelectedFeatures["selectiveTransliteration"] {
			providerFeature = "selectiveTransliteration"
		}
		
		if providerFeature != "" {
			featureOpts, ok := request.Options.Options[providerFeature]
			if !ok {
				tsk.Handler.Log(core.Error, core.AbortTask, providerFeature + " options not found")
				return
			}
			
			// Process common provider settings
			if dockerRecreate, ok := featureOpts["dockerRecreate"]; ok {
				if recreate, ok := dockerRecreate.(bool); ok {
					tsk.DockerRecreate = recreate
				}
			}
			
			if browserAccessURL, ok := featureOpts["browserAccessURL"]; ok {
				if url, ok := browserAccessURL.(string); ok {
					tsk.BrowserAccessURL = url
				}
			}
			
			if style, ok := featureOpts["style"]; ok {
				if styleStr, ok := style.(string); ok {
					tsk.RomanizationStyle = styleStr
				}
			}
			
			if provider, ok := featureOpts["provider"]; ok {
				// Provider info is captured in the style selection for romanization
				tsk.Handler.ZeroLog().Debug().Interface("provider", provider).Msg("Provider info")
			}
			
			tsk.Handler.ZeroLog().Debug().
				Interface("subtitle_provider_options", featureOpts).
				Bool("docker_recreate", tsk.DockerRecreate).
				Str("browser_url", tsk.BrowserAccessURL).
				Str("romanization_style", tsk.RomanizationStyle).
				Msg("Configured Subtitle Provider")
		}
		
		// Process feature-specific settings and add the corresponding TranslitType
		
		// Selective Transliteration
		if request.SelectedFeatures["selectiveTransliteration"] {
			tsk.TranslitTypes = append(tsk.TranslitTypes, core.Selective)
			
			// Get selective transliteration specific options
			featureOpts, ok := request.Options.Options["selectiveTransliteration"]
			if ok {
				if kanjiThreshold, ok := featureOpts["kanjiFrequencyThreshold"]; ok {
					if threshold, ok := kanjiThreshold.(float64); ok {
						tsk.KanjiThreshold = int(threshold)
					}
				}
			
				tsk.Handler.ZeroLog().Debug().
					Interface("selective_transliteration_options", featureOpts).
					Int("kanji_threshold", tsk.KanjiThreshold).
					Msg("Configured Selective Transliteration")
			}
		}
		
		// Subtitle Romanization
		if request.SelectedFeatures["subtitleRomanization"] {
			tsk.TranslitTypes = append(tsk.TranslitTypes, core.Romanize)
			tsk.Handler.ZeroLog().Debug().Msg("Subtitle Romanization enabled")
		}
		
		// Subtitle Tokenization
		if request.SelectedFeatures["subtitleTokenization"] {
			tsk.TranslitTypes = append(tsk.TranslitTypes, core.Tokenize)
			tsk.Handler.ZeroLog().Debug().Msg("Subtitle Tokenization enabled")
		}
	}

	if request.SelectedFeatures["voiceEnhancing"] {
		featureOpts, ok := request.Options.Options["voiceEnhancing"]
		if !ok {
			tsk.Handler.Log(core.Error, core.AbortTask, "voiceEnhancing options not found")
			return
		}
		
		tsk.Mode = core.Enhance
		
		if sepLib, ok := featureOpts["sepLib"]; ok {
			if sepLibStr, ok := sepLib.(string); ok {
				tsk.SeparationLib = sepLibStr
			}
		}
		
		if voiceBoost, ok := featureOpts["voiceBoost"]; ok {
			if boost, ok := voiceBoost.(float64); ok {
				tsk.VoiceBoost = boost
			}
		}
		
		if originalBoost, ok := featureOpts["originalBoost"]; ok {
			if boost, ok := originalBoost.(float64); ok {
				tsk.OriginalBoost = boost
			}
		}
		
		if limiter, ok := featureOpts["limiter"]; ok {
			if limit, ok := limiter.(float64); ok {
				tsk.Limiter = limit
			}
		}

		tsk.Handler.ZeroLog().Debug().
			Interface("voice_enhancing_options", featureOpts).
			Msg("Configured Voice Enhancing")
	}
	

	if request.SelectedFeatures["dubtitles"] {
		featureOpts, ok := request.Options.Options["dubtitles"]
		if !ok {
			tsk.Handler.Log(core.Error, core.AbortTask, "dubtitles options not found")
			return
		}
		
		tsk.Mode = core.Subs2Dubs
		
		if padTiming, ok := featureOpts["padTiming"]; ok {
			if padding, ok := padTiming.(float64); ok {
				tsk.Offset = time.Duration(int(padding)) * time.Millisecond
			}
		}
		
		if stt, ok := featureOpts["stt"]; ok {
			if sttStr, ok := stt.(string); ok {
				tsk.STT = sttStr
			}
		}
		
		if sttTimeout, ok := featureOpts["sttTimeout"]; ok {
			if timeout, ok := sttTimeout.(float64); ok {
				tsk.TimeoutSTT = int(timeout)
			}
		}
		
		if initialPrompt, ok := featureOpts["initialPrompt"]; ok {
			if prompt, ok := initialPrompt.(string); ok {
				tsk.InitialPrompt = prompt
			}
		}

		tsk.Handler.ZeroLog().Debug().
			Interface("dubtitles_options", featureOpts).
			Msg("Configured Dubtitles")
	}
	

	if request.SelectedFeatures["subs2cards"] {
		featureOpts, ok := request.Options.Options["subs2cards"]
		if !ok {
			tsk.Handler.Log(core.Error, core.AbortTask, "subs2cards options not found")
			return
		}
		
		tsk.Mode = core.Subs2Cards
		
		if padTiming, ok := featureOpts["padTiming"]; ok {
			if padding, ok := padTiming.(float64); ok {
				tsk.Offset = time.Duration(int(padding)) * time.Millisecond
			}
		}
		
		if screenshotWidth, ok := featureOpts["screenshotWidth"]; ok {
			if width, ok := screenshotWidth.(float64); ok {
				media.MaxWidth = int(width)
			}
		}
		
		if screenshotHeight, ok := featureOpts["screenshotHeight"]; ok {
			if height, ok := screenshotHeight.(float64); ok {
				media.MaxHeight = int(height)
			}
		}
		
		if condensedAudio, ok := featureOpts["condensedAudio"]; ok {
			if condensed, ok := condensedAudio.(bool); ok {
				tsk.WantCondensedAudio = condensed
			}
		}

		tsk.Handler.ZeroLog().Debug().
			Interface("subs2cards_options", featureOpts).
			Msg("Configured Subs2Cards")
	}
}



func placeholder3234567() {
	fmt.Print("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}