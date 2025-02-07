package core

import (
	"os"
	"strings"
	"path"
	"path/filepath"
	"time"
	"runtime"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/schollz/progressbar/v3"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
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
	Runtime string
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
	
	// Subs2cards options
	CondensedAudio       bool
	
	// Audio track options
	TargetChan           int
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
	WantDubs             bool
	
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
	tsk.Meta.WorkersMax = runtime.NumCPU()-1
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

	// Apply API keys from config or environment
	for _, name := range []string{"replicate", "assemblyai", "elevenlabs"} {
		var key string
		switch name {
		case "replicate":
			key = settings.APIKeys.Replicate
			if s := os.Getenv("REPLICATE_API_KEY"); s != "" {
				key = s
			}
		case "assemblyai":
			key = settings.APIKeys.AssemblyAI
			if s := os.Getenv("ASSEMBLYAI_API_KEY"); s != "" {
				key = s
			}
		case "elevenlabs":
			key = settings.APIKeys.ElevenLabs
			if s := os.Getenv("ELEVENLABS_API_KEY"); s != "" {
				key = s
			}
		}
		voice.APIKeys[name] = key
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
	tmp, err := getFFmpegVersion(media.FFmpegPath)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks, "failed to access FFmpeg binary")
	}
	tsk.Meta.FFmpeg = tmp
	tsk.Meta.Runtime = getRuntimeInfo()
	
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
	
	if procErr := tsk.PrepareLangs(); procErr.Err != nil {
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



func getFFmpegVersion(FFmpegPath string) (string, error) {
	cmd := exec.Command(FFmpegPath, "-version")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run ffmpeg: %v", err)
	}

	// Convert output to a string
	output := out.String()

	// Define a regular expression to extract the version and commit hash
	// Example output: "ffmpeg version 4.3.2 Copyright (c) 2000-2021 the FFmpeg developers"
	re := regexp.MustCompile(`ffmpeg version (\S+)`)
	match := re.FindStringSubmatch(output)

	if len(match) < 2 {
		return "", fmt.Errorf("failed to extract ffmpeg version from output")
	}

	// Return the version found in the output
	return match[1], nil
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



func getRuntimeInfo() string {
	var sb strings.Builder

	// Store Go version
	sb.WriteString(fmt.Sprintf("\nGo version: %s\n", runtime.Version()))

	// Store OS and architecture information
	sb.WriteString(fmt.Sprintf("OS: %s\nArchitecture: %s\n", runtime.GOOS, runtime.GOARCH))

	// Store number of CPUs
	sb.WriteString(fmt.Sprintf("Number of CPUs: %d\n", runtime.NumCPU()))

	// Store number of Goroutines
	sb.WriteString(fmt.Sprintf("Number of Goroutines: %d\n", runtime.NumGoroutine()))

	// Store memory statistics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	sb.WriteString(fmt.Sprintf("Memory Allocated: %d bytes\n", memStats.Alloc))
	sb.WriteString(fmt.Sprintf("Memory Total Allocated: %d bytes\n", memStats.TotalAlloc))
	sb.WriteString(fmt.Sprintf("Memory System: %d bytes\n", memStats.Sys))
	/*sb.WriteString(fmt.Sprintf("Garbage Collection Cycles: %d\n", memStats.NumGC))

	// Store time since program started
	startTime := time.Now()
	sb.WriteString(fmt.Sprintf("Current time: %s\n", startTime.Format(time.RFC1123)))

	// Store process information
	pid := os.Getpid()
	sb.WriteString(fmt.Sprintf("Process ID: %d\n", pid))
	sb.WriteString(fmt.Sprintf("Parent Process ID: %d\n", os.Getppid()))

	// Store host information
	hostname, err := os.Hostname()
	if err != nil {
		sb.WriteString(fmt.Sprintf("Error retrieving hostname: %s\n", err))
	} else {
		sb.WriteString(fmt.Sprintf("Hostname: %s\n", hostname))
	}

	// Store environment variables (can filter sensitive variables if necessary)
	envVars := os.Environ()
	sb.WriteString("Environment Variables:\n")
	for _, env := range envVars {
		sb.WriteString(fmt.Sprintf("%s\n", env))
	}*/
	return sb.String()
}





func placeholder2345634567() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
