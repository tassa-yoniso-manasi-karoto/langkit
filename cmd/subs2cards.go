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

	Args: argFuncs(cobra.MinimumNArgs(0), cobra.MaximumNArgs(2)),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			logger.Fatal().Msg("this command requires at least one argument: the path to the media file/directory to be processed")
		}
		tsk := DefaultTask(cmd)
		if len(args) > 0 {
			tsk.MediaSourceFile = args[0]
		}
		if len(args) > 1 {
			tsk.TargSubFile = args[1]
		}
		if len(args) > 2 { // TODO test without native subs
			tsk.RefSubFile = args[2]
		}
		tsk.SeparationLib = sep
		tsk.TimeoutSep, _ = cmd.Flags().GetInt("sep-to")
		
		tsk.STT = STT
		tsk.TimeoutSTT, _ = cmd.Flags().GetInt("stt-to")
		
		Offset, _     := cmd.Flags().GetInt("offset")
		tsk.Offset = time.Duration(Offset)*time.Millisecond
		tsk.routing()
	},
}

func init() {
	subs2cardsCmd.PersistentFlags().Int("offset", 250, "pad before & after the timings of each audio clip with this\n" +
		"offset in millisecond. Useful to compensate inaccurate sync\n" +
			"between subs and voices.")

	rootCmd.AddCommand(subs2cardsCmd)
}
