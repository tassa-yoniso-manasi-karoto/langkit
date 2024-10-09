package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/extract"
)


var enhanceCmd = &cobra.Command{
	Use:   "enhance <mediafile>",
	Short: sepDescr,

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(1)),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			logger.Fatal().Msg("this command requires at least one argument: the path to the media file to be processed")
		}
		mediafile = args[0]
		targetChan, _ := cmd.Flags().GetInt("chan")
		audiotrack, _ := cmd.Flags().GetInt("a")
		TimeoutSep, _ := cmd.Flags().GetInt("sep-to")
		tsk := extract.Task{
			Log:                  logger,
			TargetChan:           targetChan,
			TimeoutSep:           TimeoutSep,
			SeparationLib:        sep,
			UseAudiotrack:        audiotrack-1,
			MediaSourceFile:      mediafile,
		}
		_, err := os.Stat(mediafile)
		if err != nil {
			logger.Fatal().Err(err).Msg("can't access passed media file/directory")
		}
		tsk.Enhance()
	},
}

