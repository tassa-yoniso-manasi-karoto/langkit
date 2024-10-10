package cmd

import (
	"github.com/spf13/cobra"
)


var subs2dubsCmd = &cobra.Command{
	Use:   "subs2dubs <mediafile> <foreign-subs> [native-subs]",
	Short: subs2dubsDescr,

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(2)),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			logger.Fatal().Msg("this command requires at least 1 argument:" +
				"the path to the media file to be processed")
		}		
		tsk := DefaultTask(cmd)
		tsk.MediaSourceFile = args[0]
		if len(args) > 1 {
			tsk.TargSubFile = args[1]
		}
		if len(args) > 2 {
			tsk.RefSubFile = args[2]
		}
		if STT == "" {
			logger.Fatal().Msg("the STT service was not specified")
		}
		tsk.STT = STT
		tsk.TimeoutSTT, _ = cmd.Flags().GetInt("stt-to")
		
		tsk.WantDubs = true
		tsk.DubsOnly = true
		tsk.routing()
	},
}
