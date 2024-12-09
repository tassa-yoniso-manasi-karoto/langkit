package cmd

import (
	"time"

	"github.com/spf13/cobra"
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

	Args: argFuncs(cobra.MinimumNArgs(0), cobra.MaximumNArgs(3)),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			logger.Fatal().Msg("this command requires at least one argument: the path to the media file/directory to be processed")
		}
		tsk := DefaultTask(cmd)
		tsk.MediaSourceFile = args[0]
		if len(args) > 1 {
			tsk.TargSubFile = args[1]
		}
		if len(args) > 2 { // TODO test without native subs
			tsk.NativeSubFile = args[2]
		}
		tsk.SeparationLib = sep
		tsk.TimeoutSep, _ = cmd.Flags().GetInt("sep-to")
		
		tsk.STT = STT
		tsk.TimeoutSTT, _ = cmd.Flags().GetInt("stt-to")
		tsk.WantDubs, _ = cmd.Flags().GetBool("stt-dub")
		
		Offset, _     := cmd.Flags().GetInt("offset")
		tsk.Offset = time.Duration(Offset)*time.Millisecond
		tsk.Mode = Subs2Cards
		if len(tsk.Langs) == 1 {
			tsk.Log.Fatal().Msg("Passed languages are improperly formatted or incomplete.")
		}
		tsk.routing()
	},
}

func init() {
	subs2cardsCmd.PersistentFlags().Int("offset", 250, "pad before & after the timings of each audio clip\n"+
		"with this offset in millisecond. Useful to compensate\ninaccurate sync between subs and voices")


	rootCmd.AddCommand(subs2cardsCmd)
}
