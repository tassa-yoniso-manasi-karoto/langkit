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
	"github.com/schollz/progressbar/v3"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
)


var (
	itembar *progressbar.ProgressBar
	totalItems int
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
	tsk = &Task{
		Handler: handler,
		Meta: Meta { WorkersMax: runtime.NumCPU()-1 },
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
	// Load settings first as defaults
	settings, err := config.LoadSettings()
	if err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks, "failed to load settings")
	}

	// Set defaults from config
	if !cmd.Flags().Changed("langs") && settings.TargetLanguage != "" {
		tsk.Langs = []string{settings.TargetLanguage}
		if settings.NativeLanguages != "" {
			tsk.Langs = append(tsk.Langs, TagsStr2TagsArr(settings.NativeLanguages)...)
		}
	} else {
		// Get from flags if specified
		tsk.Langs, _ = cmd.Flags().GetStringSlice("langs")
	}
	
	for _, name := range []string{"ffmpeg", "mediainfo"} {
		dest := ""
		bin := name
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}
		if cmd.Flags().Changed(name) {
			tmp, _ := cmd.Flags().GetString(name)
			dest = tmp
			tsk.Handler.ZeroLog().Debug().Msg("using flag provided binary for " + name)
		}
		switch bin {
		case "ffmpeg":
			media.FFmpegPath = dest
		case "mediainfo":
			MediainfoPath = dest
		}
	}
	tsk.TargetChan, _ = cmd.Flags().GetInt("chan")
	audiotrack, _ := cmd.Flags().GetInt("a")
	tsk.UseAudiotrack = audiotrack-1
	if cmd.Flags().Changed("workers") {
		tsk.Meta.WorkersMax, _ = cmd.Flags().GetInt("workers")
	}
	if exists, value := IsFlagStrSet(cmd, "sep"); exists {
		tsk.SeparationLib = value
	}
	if exists, value := IsFlagStrSet(cmd, "stt"); exists {
		tsk.STT = value
	}
	if exists, value := IsFlagStrSet(cmd, "browser-access-url"); exists {
		tsk.BrowserAccessURL = value
	}
	
	
	if exists, value := IsFlagIntSet(cmd, "sep-to"); exists {
		tsk.TimeoutSep = value
	}
	if exists, value := IsFlagIntSet(cmd, "stt-to"); exists {
		tsk.TimeoutSTT = value
	}
	if exists, value := IsFlagIntSet(cmd, "offset"); exists {
		tsk.Offset = time.Duration(value)*time.Millisecond
	}
	if exists, value := IsFlagIntSet(cmd, "w"); exists {
		media.MaxWidth = value
	}
	if exists, value := IsFlagIntSet(cmd, "h"); exists {
		media.MaxHeight = value
	}
	
	
	if exists, value := IsFlagBoolSet(cmd, "stt-dub"); exists {
		tsk.WantDubs = value
	}
	if exists, value := IsFlagBoolSet(cmd, "translit"); exists {
		tsk.WantTranslit = value
	}
	
	if procErr := tsk.PrepareLangs(); procErr != nil {
		return procErr
	}
	tsk.Handler.ZeroLog().Trace().Strs("langs", tsk.Langs).Msg("PrepareLangs done:")
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

// DebugVals returns a string representation of a sanitized copy of the Task.
// It makes a shallow copy of the Task, sets the Handler field to nil (to avoid clutter),
// and then uses pp.Sprintln to generate the debug string.
// Note: This shallow copy approach should work fine with the pp package, but any pointer
// fields (other than Handler) will still reference the original underlying data.
func (tsk *Task) DebugVals() string {
	cp := *tsk // shallow copy
	cp.Handler = nil
	return pp.Sprintln(cp)
}


func IsFlagStrSet(cmd *cobra.Command, flagName string) (bool, string) {
	if flag := cmd.Flags().Lookup(flagName); flag != nil {
	    if cmd.Flags().Changed(flagName) {
	        value, _ := cmd.Flags().GetString(flagName)
	        return true, value
	    }
	}
	return false, ""
}



func IsFlagIntSet(cmd *cobra.Command, flagName string) (bool, int) {
	if flag := cmd.Flags().Lookup(flagName); flag != nil {
	    if cmd.Flags().Changed(flagName) {
	        value, _ := cmd.Flags().GetInt(flagName)
	        return true, value
	    }
	}
	return false, 0
}



func IsFlagBoolSet(cmd *cobra.Command, flagName string) (bool, bool) {
	if flag := cmd.Flags().Lookup(flagName); flag != nil {
	    if cmd.Flags().Changed(flagName) {
	        value, _ := cmd.Flags().GetBool(flagName)
	        return true, value
	    }
	}
	return false, false
}


func placeholder2345634567() {
	fmt.Print("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
