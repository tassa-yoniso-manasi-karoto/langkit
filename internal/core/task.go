package core

import (
	"os"
	"path"
	"path/filepath"
	"time"
	"runtime"
	"fmt"
	"os/exec"
	
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
)


func init() {	
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
}

type Mode int

const (
	Subs2Cards = iota
	Subs2Dubs
	Enhance
	Translit
)

func (m Mode) String() string{
	return []string{"Subs2Cards", "Subs2Dubs", "Enhance", "Translit"}[m]
}

type Meta struct {
	FFmpeg string
	MediaInfo MediaInfo
	WorkersMax int
}


type Task struct {
	Handler              MessageHandler
	Meta                 Meta
	Mode                 Mode
	
	// Injected services for testability
	LanguageDetector     LanguageDetector
	MediaInfoProvider    MediaInfoProvider
	SubtitleProvider     SubtitleProvider
	TrackSelector        TrackSelector
	WorkerPool           WorkerPool
	ResumptionService    ResumptionService
	PathService          PathService
	ProgressTracker      ProgressTracker
	
	// Language settings
	OriginalLang         string // FIXME what for?
	Langs                []string
	RefLangs             []Lang
	Targ                 Lang
	Native               Lang
	
	// File paths
	// mediaSourceFile is the path of the actual media provided or any media found while routing()
	MediaSourceFile      string
	TargSubFile          string
	NativeSubFile        string
	// mediaprefix is the base string for building AVIF / OPUS to which timecodes of a subtitle line will be added.
	MediaPrefix          string // base string for building AVIF/OPUS
	
	// Subtitles
	NativeSubs           *subs.Subtitles
	TargSubs             *subs.Subtitles
	
	// Processing options
	IsBulkProcess        bool
	DubsOnly             bool
	IsCCorDubs           bool
	
	// Common options
	FieldSep             string // defaults to "\t"
	OutputFileExtension  string // defaults to ".tsv" for "\t" and ".csv" otherwise
	Offset               time.Duration
	MaxAPIRetries        int
	
	// Subs2cards options
	WantCondensedAudio   bool
	
	// Audio track options
	TargetChan           int // TODO rename TargetChanNum
	UseAudiotrack        int
	
	// Voice enhancement options
	SeparationLib        string
	TimeoutSep           int
	VoiceBoost           float64
	OriginalBoost        float64
	Limiter              float64
	MergingFormat        string
	
	// STT options
	STT                  string
	TimeoutSTT           int
	WantDubs             bool // controls whether dubtitle file should be made too when using STT for subs2cards
	InitialPrompt        string
	
	// Romanization options
	WantTranslit         bool
	RomanizationStyle    string
	KanjiThreshold       int
	BrowserAccessURL     string
	DockerRecreate       bool
}

func NewTask(handler MessageHandler) (tsk *Task) {
	// Initialize task with default services
	fileScanner := NewFileScanner(handler)
	pathSanitizer := NewPathSanitizer()
	
	tsk = &Task{
		Handler: handler,
		Meta: Meta { WorkersMax: runtime.NumCPU()-1 },
		
		// Initialize service interfaces with default implementations
		LanguageDetector:  NewLanguageDetector(),
		MediaInfoProvider: NewMediaInfoProvider("mediainfo", crash.Reporter), // Use crash.Reporter as it implements Reporter interface
		SubtitleProvider:  NewSubtitleProvider(handler),
		TrackSelector:     NewTrackSelector(handler),
		PathService:       NewPathService(pathSanitizer),
		ProgressTracker:   NewProgressTracker(handler, "item-bar"),
		ResumptionService: NewResumptionService(fileScanner, "\t", handler),
		// WorkerPool is created on demand with task-specific configuration
		
		// Default task settings
		UseAudiotrack: -1,
		TargetChan: 2,
		VoiceBoost: 13,
		OriginalBoost: -9,
		Limiter:  0.9,
		MaxAPIRetries: 10,
		// the actual control over STT activation remains in STT string being != "",
		// by default assume a subtitle file is wanted and therefore
		// let that value be overwritten as needed (currently only by CLI).
		WantDubs: true,
		KanjiThreshold: -1,
	}
	
	if tsk.FieldSep == "" {
		tsk.FieldSep = "\t"
	}
	if tsk.OutputFileExtension == "" {
		switch tsk.FieldSep {
		case "\t":
			tsk.OutputFileExtension = ".tsv"
		default:
			tsk.OutputFileExtension = ".csv"
		}
	}
	
	for _, name := range []string{"ffmpeg", "mediainfo"} {
		dest := ""
		bin := name
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}
		// get dir of langkit bin
		ex, err := os.Executable()
		if err != nil {
			tsk.Handler.ZeroLog().Debug().Err(err).Msg("failed to access directory where langkit is " +
				bin + " path must be provided by PATH or specified manually")
		}
		local := path.Join(filepath.Dir(ex), "bin", bin)
		path, _ := exec.LookPath(bin)
		if _, err := os.Stat(local); err == nil {
			dest = local
			tsk.Handler.ZeroLog().Debug().Msg("found a local binary for " + name)
		} else {
			dest = path
			tsk.Handler.ZeroLog().Trace().Msg("PATH provided binary path for " + name)
		}
		switch bin {
		case "ffmpeg":
			media.FFmpegPath = dest
		case "mediainfo":
			MediainfoPath = dest
		}
	}
	
	if settings, err := config.LoadSettings(); err == nil {
		tsk.MaxAPIRetries = settings.MaxAPIRetries
	}
	
	return tsk
}

func (tsk *Task) ApplyFlags(cmd *cobra.Command) *ProcessingError {
	settings, err := config.LoadSettings()
	if err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks, "failed to load settings")
	}

	tsk.applyConfigSettings(settings)
	
	// override config settings if specified
	tsk.applyCLIFlags(cmd)
	
	if procErr := tsk.PrepareLangs(); procErr != nil {
		return procErr
	}
	tsk.Handler.ZeroLog().Trace().Strs("langs", tsk.Langs).Msg("PrepareLangs done")
	
	switch tsk.STT {
	case "wh":
		tsk.STT = "whisper"
	case "fast", "incredibly-fast-whisper":
		tsk.STT = "insanely-fast-whisper"
	case "u1":
		tsk.STT = "universal-1"
	}
	switch tsk.SeparationLib {
	case "de":
		tsk.SeparationLib = "demucs"
	case "ft":
		tsk.SeparationLib = "demucs_ft"
	case "sp":
		tsk.SeparationLib = "spleeter"
	case "11", "el":
		tsk.SeparationLib = "elevenlabs"
	}
	return nil
}

func (tsk *Task) applyConfigSettings(settings config.Settings) {
	if settings.MaxAPIRetries > 0 {
		tsk.MaxAPIRetries = settings.MaxAPIRetries
	}
	
	// FIXME CLI wasn't designed with a default language in config in mind → undefined behavior down the line
	if settings.TargetLanguage != "" {
		tsk.Langs = []string{settings.TargetLanguage}
		if settings.NativeLanguages != "" {
			tsk.Langs = append(tsk.Langs, TagsStr2TagsArr(settings.NativeLanguages)...)
		}
	}
}

// applyCLIFlags applies settings from command line flags
func (tsk *Task) applyCLIFlags(cmd *cobra.Command) {
	strFlags := map[string]*string{
		"sep":                &tsk.SeparationLib,
		"stt":                &tsk.STT,
		"browser-access-url": &tsk.BrowserAccessURL,
	}
	
	intFlags := map[string]*int{
		"chan":    &tsk.TargetChan,
		"workers": &tsk.Meta.WorkersMax,
		"sep-to":  &tsk.TimeoutSep,
		"stt-to":  &tsk.TimeoutSTT,
		"w":       &media.MaxWidth,
		"h":       &media.MaxHeight,
	}
	
	boolFlags := map[string]*bool{
		"stt-dub":  &tsk.WantDubs,
		"translit": &tsk.WantTranslit,
	}
	
	// Special case for "offset" which needs conversion to time.Duration
	if cmd.Flags().Changed("offset") {
		if val, err := cmd.Flags().GetInt("offset"); err == nil {
			tsk.Offset = time.Duration(val) * time.Millisecond
		}
	}
	
	// Special case for "a" (audiotrack) which needs adjustment
	if cmd.Flags().Changed("a") {
		if val, err := cmd.Flags().GetInt("a"); err == nil {
			tsk.UseAudiotrack = val - 1
		}
	}
	
	// Special case for "langs" which is a string slice
	if cmd.Flags().Changed("langs") {
		if val, err := cmd.Flags().GetStringSlice("langs"); err == nil {
			tsk.Langs = val
		}
	}
	
	// STRING
	for name, dest := range strFlags {
		if cmd.Flags().Changed(name) {
			if val, err := cmd.Flags().GetString(name); err == nil {
				*dest = val
			}
		}
	}
	
	// INT
	for name, dest := range intFlags {
		if cmd.Flags().Changed(name) {
			if val, err := cmd.Flags().GetInt(name); err == nil {
				*dest = val
			}
		}
	}
	
	// BOOL
	for name, dest := range boolFlags {
		if cmd.Flags().Changed(name) {
			if val, err := cmd.Flags().GetBool(name); err == nil {
				*dest = val
			}
		}
	}
	
	// Binary paths
	for _, name := range []string{"ffmpeg", "mediainfo"} {
		if cmd.Flags().Changed(name) {
			path, _ := cmd.Flags().GetString(name)
			tsk.Handler.ZeroLog().Debug().Msgf("using flag-provided binary for %s: %s", name, path)
			
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			
			switch name {
			case "ffmpeg", "ffmpeg.exe":
				media.FFmpegPath = path
			case "mediainfo", "mediainfo.exe":
				MediainfoPath = path
			}
		}
	}
}


// DebugVals returns a string representation of a sanitized copy of the Task for crash reporting.
// It makes a shallow copy of the Task and sets all interface fields to nil before pretty printing.
// This prevents memory leaks that can occur when large interface implementations or circular 
// references are included in the pretty-printed output. The returned string contains all primitive
// values and configuration settings while excluding potentially problematic service references.
func (tsk *Task) DebugVals() string {
	cp := *tsk // shallow copy
	
	cp.Handler = nil
	cp.LanguageDetector = nil
	cp.MediaInfoProvider = nil
	cp.SubtitleProvider = nil
	cp.TrackSelector = nil
	cp.WorkerPool = nil
	cp.ResumptionService = nil
	cp.PathService = nil
	cp.ProgressTracker = nil
	
	return pp.Sprintln(cp)
}


func placeholder2345634567() {
	fmt.Print("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
