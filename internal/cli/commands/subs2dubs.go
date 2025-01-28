package commands

import (
	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)


var subs2dubsCmd = &cobra.Command{
	Use:   "subs2dubs <mediafile> <foreign-subs> [native-subs]",
	Short: "Use foreign subtitle file to create a dubtitle using transcriptions made by the selected STT service",

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(2)),
	Run: func(cmd *cobra.Command, args []string) {
		tsk := core.NewTask(core.NewCLIHandler())
		if len(args) < 1 {
			tsk.Handler.ZeroLog().Fatal().Msg("this command requires at least 1 argument:" +
				"the path to the media file to be processed")
		}
		tsk.ApplyFlags(cmd)
		tsk.MediaSourceFile = args[0]
		if len(args) > 1 {
			tsk.TargSubFile = args[1]
		}
		if len(args) > 2 {
			tsk.NativeSubFile = args[2]
		}
		if tsk.STT == "" {
			tsk.Handler.ZeroLog().Fatal().Msg("the STT service was not specified")
		}
		
		tsk.WantDubs = true
		tsk.DubsOnly = true
		tsk.Mode = core.Subs2Dubs
		tsk.Routing()
	},
}
