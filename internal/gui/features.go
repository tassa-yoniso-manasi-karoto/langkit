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

func (a *App) ProcessFiles(request ProcessRequest) {
	processCtx, cancel := context.WithCancel(a.ctx)
	defer cancel() // TODO assign this cancel in App as procCancel

	task := core.NewTask(a.handler)
	a.configureTask(task, request)
	
	task.MediaSourceFile = request.Path
	
	task.Handler.ZeroLog().Info().
		Str("file", task.MediaSourceFile).
		Str("mode", string(task.Mode)).
		Msg("Starting processing")

	pp.Println(request)
	
	task.Routing(processCtx)
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
	Subs2Cards struct {
		PadTiming        int  `json:"padTiming"`
		ScreenshotWidth  int  `json:"screenshotWidth"`
		ScreenshotHeight int  `json:"screenshotHeight"`
		CondensedAudio   bool `json:"condensedAudio"`
	} `json:"subs2cards"`

	Dubtitles struct {
		PadTiming  int    `json:"padTiming"`
		STT        string `json:"stt"`
		STTtimeout int    `json:"sttTimeout"`
	} `json:"dubtitles"`

	VoiceEnhancing struct {
		SepLib        string  `json:"sepLib"`
		VoiceBoost    float64 `json:"voiceBoost"`
		OriginalBoost float64 `json:"originalBoost"`
		Limiter       float64 `json:"limiter"`
		MergingFormat string  `json:"mergingFormat"`
	} `json:"voiceEnhancing"`

	SubtitleRomanization struct {
		Style                    string `json:"style"`
		SelectiveTransliteration int    `json:"selectiveTransliteration,omitempty"`
		DockerRecreate           bool   `json:"dockerRecreate"`
		BrowserAccessURL         string `json:"browserAccessURL"`
	} `json:"subtitleRomanization"`
}



func (a *App) configureTask(task *core.Task, request ProcessRequest) {
	settings, err := config.LoadSettings()
	if err != nil {
		// TODO return tsk.Handler.LogErr(err, AbortAllTasks, "failed to load settings")
	}

	if request.LanguageCode != "" {
		task.Langs = []string{request.LanguageCode}
		if settings.NativeLanguages != "" {
			task.Langs = append(task.Langs, core.TagsStr2TagsArr(settings.NativeLanguages)...)
		} else {
			// TODO return ERR
		}
	}
	
	if procErr := task.PrepareLangs(); procErr != nil {
		task.Handler.ZeroLog().Error().Err(procErr.Err).
			Msg("PrepareLangs failed")
	}
	// Configure based on selected features starting from the most specific,
	// restricted processing mode to the most general, multipurpose in order to
	// have the correct task.Mode at the end to pass on downstream.

	// Set audio track if specified
	if request.AudioTrackIndex > 0 {
		task.UseAudiotrack = request.AudioTrackIndex // TODO check if UseAudiotrack start from 0 or not
		task.Handler.ZeroLog().Debug().
			Int("track_index", request.AudioTrackIndex).
			Msg("Set audio track index")
	}

	if request.SelectedFeatures["subtitleRomanization"] {
		opts := request.Options.SubtitleRomanization
		task.Mode = core.Translit
		task.WantTranslit = true
		task.RomanizationStyle = opts.Style

		if opts.SelectiveTransliteration > 0 {
			task.KanjiThreshold = opts.SelectiveTransliteration
		}

		// New options
		task.DockerRecreate = opts.DockerRecreate
		task.BrowserAccessURL = opts.BrowserAccessURL

		task.Handler.ZeroLog().Debug().
			Interface("romanization_options", opts).
			Bool("docker_recreate", opts.DockerRecreate).
			Str("browser_url", opts.BrowserAccessURL).
			Msg("Configured Subtitle Romanization")
	}

	if request.SelectedFeatures["voiceEnhancing"] {
		opts := request.Options.VoiceEnhancing
		task.Mode = core.Enhance
		task.SeparationLib = opts.SepLib
		task.VoiceBoost = opts.VoiceBoost
		task.OriginalBoost = opts.OriginalBoost
		task.Limiter = opts.Limiter
		task.MergingFormat = opts.MergingFormat

		task.Handler.ZeroLog().Debug().
			Interface("voice_enhancing_options", opts).
			Msg("Configured Voice Enhancing")
	}

	if request.SelectedFeatures["dubtitles"] {
		opts := request.Options.Dubtitles
		task.Mode = core.Subs2Dubs
		task.Offset = time.Duration(opts.PadTiming) * time.Millisecond
		task.STT = opts.STT
		task.TimeoutSTT = opts.STTtimeout

		task.Handler.ZeroLog().Debug().
			Interface("dubtitles_options", opts).
			Msg("Configured Dubtitles")
	}

	if request.SelectedFeatures["subs2cards"] {
		opts := request.Options.Subs2Cards
		task.Mode = core.Subs2Cards
		task.Offset = time.Duration(opts.PadTiming) * time.Millisecond
		media.MaxWidth = opts.ScreenshotWidth
		media.MaxHeight = opts.ScreenshotHeight
		task.CondensedAudio = opts.CondensedAudio

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


