package cmd

import (
	"os"
	"strings"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)


type Task struct {
	Log                  zerolog.Logger
	Meta                 MediaInfo
	OriginalLang         string // FIXME what for?
	Langs                []string
	RefLangs             []Lang
	Targ                 Lang
	SeparationLib        string
	STT                  string
	TargetChan           int
	UseAudiotrack        int
	TimeoutSTT           int
	TimeoutSep           int
	Offset               time.Duration
	WantDubs             bool
	DubsOnly             bool
	IsCCorDubs           bool
	TargSubFile          string
	RefSubFile           string
	MediaSourceFile      string
	FieldSep             string // defaults to "\t"
	OutputFileExtension  string // defaults to ".tsv" for "\t" and ".csv", otherwise
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
	return &tsk
}


func (tsk *Task) routing() {
	mediafile := tsk.MediaSourceFile
	media, err := os.Stat(mediafile)
	if err != nil {
		logger.Fatal().Err(err).Msg("can't access passed media file/directory")
	}
	if !media.IsDir() {
		tsk.Execute()
	} else {
		err = filepath.Walk(mediafile, func(path string, info os.FileInfo, err error) error {
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
			tsk.RefSubFile = ""
			tsk.TargSubFile = ""
			tsk.MediaSourceFile = path
			tsk.Execute() // TODO go tsk.Execute()?
			return nil
		})
	}
}
