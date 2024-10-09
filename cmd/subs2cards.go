package cmd

import (
	"os"
	"time"
	"strings"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/extract"
)

var subs2cardsCmd = &cobra.Command{
	Use:   "subs2cards <mediafile> <foreign-subs> [native-subs]",
	Short: "Decompose media into flash cards",
	Long: `This command generates flash cards for an SRS application like Anki from subtitles and optional associated media content.

Example:
  langkit subs2cards media-content.mp4 foreign.srt native.srt

Based on the given subtitle files and associated media file, the above
command would create the tab-separated file "foreign.tsv" and a directory
"foreign.media/" containing images and audio files. Among other fields,
"foreign.tsv" would have a current, previous and next subtitle item from
both subtitle files, but the timing reference would be "foreign.srt".`,

	Args: argFuncs(cobra.MinimumNArgs(0), cobra.MaximumNArgs(2)),
	Run: func(cmd *cobra.Command, args []string) {
		var foreignSubs, nativeSubs string
		if len(args) == 0 {
			logger.Fatal().Msg("this command requires at least one argument: the path to the media file/directory to be processed")
		}
		if len(args) > 0 {
			mediafile = args[0]
		}
		if len(args) > 1 {
			foreignSubs = args[1]
		}
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

func init() {
	subs2cardsCmd.PersistentFlags().Int("offset", 250, "pad before & after the timings of each audio clip with this\n" +
		"offset in millisecond. Useful to compensate inaccurate sync\n" +
			"between subs and voices.")

	rootCmd.AddCommand(subs2cardsCmd)
}
