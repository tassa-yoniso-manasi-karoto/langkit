package cmd

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
	"io"
	
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/schollz/progressbar/v3"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/subs"
)

var itembar *progressbar.ProgressBar
var totalItems int

type Mode int

const (
	Subs2Cards = iota
	Subs2Dubs
	Subs2Translit
	Enhance
)

func (m Mode) String() string{
	return []string{"Subs2Cards", "Subs2Dubs", "Subs2Translit", "Enhance"}[m]
}

type Task struct {
	Log                  zerolog.Logger
	Meta                 Meta
	Mode                 Mode
	OriginalLang         string // FIXME what for?
	Langs                []string
	RefLangs             []Lang
	Targ                 Lang
	Native               Lang
	SeparationLib        string
	STT                  string
	TargetChan           int
	UseAudiotrack        int
	TimeoutSTT           int
	TimeoutSep           int
	Offset               time.Duration
	WantDubs             bool
	IsBulkProcess        bool
	DubsOnly             bool
	IsCCorDubs           bool
	TargSubFile          string
	NativeSubFile        string
	NativeSubs           *subs.Subtitles
	// mediaprefix is the base string for building AVIF / OPUS to which timecodes of a subtitle line will be added.
	MediaPrefix          string
	// mediaSourceFile is the path of the actual media provided or any media found while routing()
	MediaSourceFile      string
	FieldSep             string // defaults to "\t"
	OutputFileExtension  string // defaults to ".tsv" for "\t" and ".csv", otherwise
}


type Meta struct {
	FFmpeg string
	MediaInfo MediaInfo
	Runtime string
}

func DefaultTask(cmd *cobra.Command) (*Task) {
	var tsk Task
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
			logger.Debug().Err(err).Msg("failed to access directory where langkit is " +
				bin + " path must be provided by PATH or specified manually")
		}
		local := path.Join(filepath.Dir(ex), "bin", bin)
		path, _ := exec.LookPath(bin)
		if _, err := os.Stat(local); err == nil {
			dest = local
			logger.Debug().Msg("found a local binary for " + name)
		} else {
			dest = path
			logger.Trace().Msg("PATH provided binary path for " + name)
		}
		if cmd.Flags().Changed(name) {
			tmp, _ := cmd.Flags().GetString(name)
			dest = tmp
			logger.Debug().Msg("using flag provided binary for " + name)
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
		logger.Fatal().Err(err).Msg("failed to access FFmpeg binary")
	}
	tsk.Meta.FFmpeg = tmp
	tsk.Meta.Runtime = getRuntimeInfo()
	targetChan, _ := cmd.Flags().GetInt("chan")
	audiotrack, _ := cmd.Flags().GetInt("a")
	//CC, _         := cmd.Flags().GetBool("cc")
	tsk = Task{
		Log:                  logger,
		Langs:                langs,
		TargetChan:           targetChan,
		//IsCC:                 CC,
		UseAudiotrack:        audiotrack-1,
		FieldSep:             "\t",
		OutputFileExtension:  "tsv",
	}
	tsk.PrepareLangs()
	logger.Trace().Err(err).Strs("langs", langs).Msg("PrepareLangs done:")
	return &tsk
}


func (tsk *Task) routing() {
	// reassign to have root dir if IsBulkProcess
	userProvided := tsk.MediaSourceFile
	stat, err := os.Stat(userProvided)
	if err != nil {
		logger.Fatal().Err(err).Msg("can't access passed media file/directory")
	}
	if tsk.IsBulkProcess = stat.IsDir(); !tsk.IsBulkProcess {
		if ok := tsk.checkIntegrity(); ok  {
			tsk.Execute()
		}		
	} else {
		var tasks []Task
		err = filepath.Walk(userProvided, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				tsk.Log.Fatal().Err(err).Msg("error during recursive exploration of passed directory")
			}
			if info.IsDir() && strings.HasSuffix(info.Name(), ".media") {
				return filepath.SkipDir
			}
			filename := filepath.Base(path)
			if !strings.HasSuffix(path, ".mp4") && !strings.HasSuffix(filename, ".mkv")  {
				return nil
			}
			tsk.NativeSubFile = ""
			tsk.TargSubFile = ""
			tsk.MediaSourceFile = path
			if ok := tsk.checkIntegrity(); !ok  {
				return nil
			}
			tsk.Autosub()
			foreignSubs, err := subs.OpenFile(tsk.TargSubFile, false)
			if err != nil {
				tsk.Log.Fatal().Err(err).Msg("can't read foreign subtitles")
			}
			if strings.Contains(strings.ToLower(tsk.TargSubFile), "closedcaption") { //TODO D.R.Y. cards.go#L120
				foreignSubs.TrimCC2Dubs()
			}
			totalItems += len(foreignSubs.Items)
			tasks = append(tasks, *tsk)
			return nil
		})
		mediabar := mkMediabar(len(tasks))
		for _, tsk := range tasks {
			mediabar.Add(1)
			// trick to have a new line without the log prefix
			tsk.Log.Info().Msg("\r             \n"+mediabar.String())
			tsk.Log.Info().Msg("now: ." + strings.TrimPrefix(tsk.MediaSourceFile, userProvided))
			tsk.Execute()
		}
	}
}

func (tsk *Task) checkIntegrity() bool {
	isCorrupted, err := media.CheckValidData(tsk.MediaSourceFile)
	l := logger.Error().Err(err).Str("video", tsk.MediaSourceFile)
	if isCorrupted {
		l.Msg("Invalid data found when processing video. Video is misformed or corrupted.")
	} else if err != nil {
		l.Msg("unspecified error found trying to check the video's integrity")
	}
	return !isCorrupted
}

// i is the total sum
func mkMediabar(i int) *progressbar.ProgressBar {
	return progressbar.NewOptions(i,
		progressbar.OptionSetDescription("Processing videos..."),
		progressbar.OptionShowCount(),
		//progressbar.OptionUseANSICodes(false),
		//progressbar.OptionSetRenderBlankState(true),
		//progressbar.OptionSetVisibility(false),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionSetWriter(io.Discard),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "#",
			SaucerPadding: "-",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

func mkItemBar(i int, descr string) *progressbar.ProgressBar {
	return progressbar.NewOptions(i,
		progressbar.OptionSetDescription(descr),
		progressbar.OptionShowCount(),		
		progressbar.OptionSetWidth(31),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "#",
			SaucerPadding: "-",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
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
