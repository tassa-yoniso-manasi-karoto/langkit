package commands

import (
	"fmt"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

var enhanceCmd = &cobra.Command{
	Use:   "enhance <mediafile>",
	Short: "Make a new audiotrack with voices louder using this separation API to isolate the voice's audio",

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(1)),
	Run: func(cmd *cobra.Command, args []string) {
		tsk := core.NewTask(core.NewCLIHandler())
		if len(args) == 0 {
			tsk.Handler.ZeroLog().Fatal().Msg("this command requires at least one argument: the path to the media file to be processed")
		}
		tsk.ApplyFlags(cmd)
		tsk.MediaSourceFile = args[0]
		
		tsk.Mode = core.Enhance
		tsk.Routing()
	},
}


func placeholder234567() {
	fmt.Print("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
