package commands

import (
	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

var subs2cardsCmd = &cobra.Command{
	Use:   "subs2cards <mediafile> <foreign-subs> [native-subs]",
	Short: "Decompose media into flash cards",
	Long: `This command generates flash cards for an SRS application like Anki from subtitles and media content.

Example:
  langkit subs2cards media-content.mp4 foreign.srt native.srt

Based on the given subtitle files and associated media file, the above
command would create the tab-separated file "foreign.tsv" and a directory
"foreign.media/" containing images and audio files. Among other fields,
"foreign.tsv" would have a current, previous and next subtitle item from
both subtitle files, but the timing reference would be "foreign.srt".`,

	Args: argFuncs(cobra.MinimumNArgs(0), cobra.MaximumNArgs(3)),
	Run: func(cmd *cobra.Command, args []string) {
		tsk := core.NewTask(core.NewCLIHandler())
		if len(args) == 0 {
			tsk.Handler.ZeroLog().Fatal().Msg("this command requires at least one argument: the path to the media file/directory to be processed")
		}
		tsk.ApplyFlags(cmd)
		tsk.MediaSourceFile = args[0]
		if len(args) > 1 {
			tsk.TargSubFile = args[1]
		}
		if len(args) > 2 { // TODO test without native subs
			tsk.NativeSubFile = args[2]
		}
		
		tsk.Mode = core.Subs2Cards
		if len(tsk.Langs) == 1 {
			tsk.Handler.ZeroLog().Fatal().Msg("Passed languages are improperly formatted or incomplete.")
		}
		tsk.Routing()
	},
}
