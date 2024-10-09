package cmd

import (
	"os"
	"time"
	"strings"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/extract"
)


var subs2dubsCmd = &cobra.Command{
	Use:   "subs2dubs <foreign-subs>",
	Short: subs2dubsDescr,

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(1)),
	Run: func(cmd *cobra.Command, args []string) {
		color.Greenln("WIP!")
		os.Exit(0)
		var foreignSubs, nativeSubs string
		if len(args) < 2 {
			logger.Fatal().Msg("this command requires at least 2 arguments:" +
				"–the path to the media file to be processed\n–the path to the reference subtitle")
		}
		mediafile = args[0]
		foreignSubs = args[1]
		if len(args) > 2 {
			nativeSubs = args[2]
		}
		targetChan, _ := cmd.Flags().GetInt("chan")
		audiotrack, _ := cmd.Flags().GetInt("a")
		Offset, _     := cmd.Flags().GetInt("offset")
		TimeoutSep, _ := cmd.Flags().GetInt("sep-to")
		TimeoutSTT, _    := cmd.Flags().GetInt("stt-to")
		//CC, _         := cmd.Flags().GetBool("cc")
		tsk := extract.Task{
			Log:                  logger,
			Langs:                langs,
			TargetChan:           targetChan,
			TimeoutSep:           TimeoutSep,
			TimeoutSTT:           TimeoutSTT,
			STT:                  STT,
			SeparationLib:        sep,
			//IsCC:                 CC,
			Offset:               time.Duration(Offset)*time.Millisecond,
			UseAudiotrack:        audiotrack-1,
			TargSubFile:          foreignSubs,
			RefSubFile:           nativeSubs,
			MediaSourceFile:      mediafile,
			FieldSep:             "\t",
			OutputFileExtension:  "tsv",
		}
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
	},
}




func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
