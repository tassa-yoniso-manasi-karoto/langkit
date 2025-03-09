package commands

import (
	"fmt"
	"context"
	
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

var enhanceCmd = &cobra.Command{
	Use:   "enhance <mediafile>",
	Short: "Make a new audiotrack with voices louder using this separation API to isolate the voice's audio",

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(1)),
	Run: RunWithExit(enhance),
}

func enhance(tsk *core.Task, ctx context.Context, cmd *cobra.Command, args []string) *core.ProcessingError {
	if len(args) == 0 {
		return tsk.Handler.Log(core.Error, "", "this command requires at least one argument: the path to the media file to be processed")
	}
	tsk.ApplyCLIFlags(cmd)
	tsk.MediaSourceFile = args[0]
	
	tsk.Mode = core.Enhance
	
	return tsk.Routing(ctx)
}



func placeholder234567() {
	fmt.Print("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
