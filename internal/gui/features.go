package gui

import (
	"time"
	
	"fmt"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

func (a *App) ProcessFiles(request ProcessRequest) {
	// Create task with the handler
	task := core.NewTask(a.handler)
	
	// Configure the task
	a.configureTask(task, request)

	if len(request.Files) != 1 {
		task.Handler.ZeroLog().Error().
			Msg("exactly one file or directory must be selected")
		return
	}

	// Set the source path
	task.MediaSourceFile = request.Files[0]
	
	/*/ Log the processing mode
	stat, err := os.Stat(task.MediaSourceFile)
	if err != nil {
		task.Handler.ZeroLog().Error().Err(err).Msg("Failed to access source path")
		return err
	}

	task.IsBulkProcess = stat.IsDir()
	task.Handler.ZeroLog().Info().
		Str("path", task.MediaSourceFile).
		Bool("is_directory", task.IsBulkProcess).
		Msg("Processing mode determined")*/

	pp.Println(request)
	color.Redln("WIP: Blocking indefinitely...")
	select {}
	
	task.Routing()
}


// ProcessRequest represents the incoming request from the frontend
type ProcessRequest struct {
	Files            []string          `json:"files"`
	SelectedFeatures map[string]bool   `json:"selectedFeatures"`
	Options          FeatureOptions    `json:"options"`
	LanguageCode     string           `json:"languageCode"`
}

type FeatureOptions struct {
	Subs2Cards struct {
		PadTiming       int  `json:"padTiming"`
		ScreenshotWidth int  `json:"screenshotWidth"`
		ScreenshotHeight int `json:"screenshotHeight"`
		CondensedAudio  bool `json:"condensedAudio"`
	} `json:"subs2cards"`
	
	Dubtitles struct {
		PadTiming   int    `json:"padTiming"`
		STT         string `json:"stt"`
		STTtimeout  int    `json:"sttTimeout"`
	} `json:"dubtitles"`
	
	VoiceEnhancing struct {
		SepLib        string  `json:"sepLib"`
		VoiceBoost    float64 `json:"voiceBoost"`
		OriginalBoost float64 `json:"originalBoost"`
		Limiter       float64 `json:"limiter"`
		MergingFormat string  `json:"mergingFormat"`
	} `json:"voiceEnhancing"`
	
	SubtitleRomanization struct {
		Style                   string `json:"style"`
		SelectiveTransliteration int    `json:"selectiveTransliteration,omitempty"`
	} `json:"subtitleRomanization"`
}


func (a *App) configureTask(task *core.Task, request ProcessRequest) {
	if request.LanguageCode != "" {
		task.Langs = []string{request.LanguageCode}
	}

	// Configure based on selected features starting from the most specific,
	// restricted processing mode to the most general, multipurpose in order to
	// have the correct task.Mode at the end to pass on downstream.
	
	if request.SelectedFeatures["subtitleRomanization"] {
		opts := request.Options.SubtitleRomanization
		task.Mode             = core.Translit
		task.WantTranslit     = true
		task.RomanizationStyle= opts.Style
		
		if opts.SelectiveTransliteration > 0 {
			task.KanjiThreshold = opts.SelectiveTransliteration
		}
		
		task.Handler.ZeroLog().Debug().
			Interface("romanization_options", opts).
			Msg("Configured Subtitle Romanization")
	}
	
	if request.SelectedFeatures["voiceEnhancing"] {
		opts := request.Options.VoiceEnhancing
		task.Mode          = core.Enhance
		task.SeparationLib = opts.SepLib
		task.VoiceBoost    = opts.VoiceBoost
		task.OriginalBoost = opts.OriginalBoost
		task.Limiter       = opts.Limiter
		task.MergingFormat = opts.MergingFormat
		
		task.Handler.ZeroLog().Debug().
			Interface("voice_enhancing_options", opts).
			Msg("Configured Voice Enhancing")
	}
	
	if request.SelectedFeatures["dubtitles"] {
		opts := request.Options.Dubtitles
		task.Mode       = core.Subs2Dubs
		task.Offset     = time.Duration(opts.PadTiming) * time.Millisecond
		task.STT        = opts.STT
		task.TimeoutSTT = opts.STTtimeout
		
		task.Handler.ZeroLog().Debug().
			Interface("dubtitles_options", opts).
			Msg("Configured Dubtitles")
	}
	
	if request.SelectedFeatures["subs2cards"] {
		opts := request.Options.Subs2Cards
		task.Mode            = core.Subs2Cards
		task.Offset          = time.Duration(opts.PadTiming) * time.Millisecond
		task.ScreenshotWidth = opts.ScreenshotWidth
		task.ScreenshotHeight= opts.ScreenshotHeight
		task.CondensedAudio  = opts.CondensedAudio
		
		task.Handler.ZeroLog().Debug().
			Interface("subs2cards_options", opts).
			Msg("Configured Subs2Cards")
	}
	return
}


/*func (a *App) updateProgress(update ProgressUpdate) {
	runtime.EventsEmit(a.ctx, "download-progress", update)
}*/




func placeholder3234567() {
	fmt.Print("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}


