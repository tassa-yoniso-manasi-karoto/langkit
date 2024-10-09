package cmd

import (
	"github.com/spf13/cobra"
)


var subs2dubsCmd = &cobra.Command{
	Use:   "subs2dubs <mediafile> <foreign-subs> [native-subs]",
	Short: subs2dubsDescr,

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(3)),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			logger.Fatal().Msg("this command requires at least 2 arguments:" +
				"–the path to the media file to be processed\n–the path to the reference subtitle")
		}		
		tsk := DefaultTask(cmd)
		tsk.MediaSourceFile = args[0]
		tsk.TargSubFile = args[1]
		if len(args) > 2 {
			tsk.RefSubFile = args[2]
		}
		
		tsk.STT = STT
		tsk.TimeoutSTT, _ = cmd.Flags().GetInt("stt-to")
		
		tsk.routing()
	},
}