package cmd

import (
	"os"
	"time"
	"strings"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/extract"
)

var mediafile, sep, STT string
var langs []string

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
			OutputFieldSeparator: "\t",
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
	subs2cardsCmd.PersistentFlags().StringSliceVarP(&langs, "langs", "l", []string{}, "ISO-639-1/3 codes of target language followed by reference language(s) sorted by preference (i.e. learning spanish from english → \"es,en\"). For each language one subtag can be specified after a hyphen \"-\" (i.e. pt-BR or zh-Hant)")
	subs2cardsCmd.PersistentFlags().StringVarP(&sep, "sep", "s", "", "specifies which source separation library to use to isolate the voice's audio")
	subs2cardsCmd.PersistentFlags().StringVar(&STT, "stt", "", "transcribe audio using specified online Speech-To-Text API")
	//subs2cardsCmd.PersistentFlags().Bool("cc", false, "enforce treating the foreign subs as closed captions: strip it of its SDH material to keep only the dialog")
	subs2cardsCmd.PersistentFlags().Int("chan", 2, "prefer audiotracks with this number of channels")
	subs2cardsCmd.PersistentFlags().Int("stt-to", 45, "timeout in seconds for the request to the STT service.")
	subs2cardsCmd.PersistentFlags().Int("sep-to", 100*60, "timeout in seconds for the request to the voice separation service. Due to the upload and remote processing it should be set very high.")
	subs2cardsCmd.PersistentFlags().Int("offset", 250, "pad before & after the timings of each audio clip with this offset in millisecond. Useful to compensate inaccurate sync between subs and voices.")
	// uh? when using subs2cardsCmd.PersistentFlags().IntP <-- default negative int is reset to 0. maybe force signed integer?
	subs2cardsCmd.PersistentFlags().Int("a", -1, "force selection of the audiotrack at this index. Useful for audiotracks missing a language tag. Overrides --chan and -l flag. Indexing start at 1.")

	rootCmd.AddCommand(subs2cardsCmd)
}

// https://github.com/spf13/cobra/issues/648#issuecomment-393154805
func argFuncs(funcs ...cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		for _, f := range funcs {
			err := f(cmd, args)
			if err != nil {
				return err
			}
		}
		return nil
	}
}
