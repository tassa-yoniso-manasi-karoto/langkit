package commands

import (
	"context"
	
	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)


var subs2dubsCmd = &cobra.Command{
	Use:   "subs2dubs <mediafile> <foreign-subs> [native-subs]",
	Short: "Use foreign subtitle file to create a dubtitle using transcriptions made by the selected STT service",

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(2)),
	Run: RunWithExit(subs2dubs),
}


func subs2dubs(tsk *core.Task, ctx context.Context, cmd *cobra.Command, args []string) *core.ProcessingError {
	if len(args) < 1 {
		return tsk.Handler.Log(core.Error, "", "this command requires at least 1 argument: the path to the media file to be processed")
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
		return tsk.Handler.Log(core.Error, "", "the STT service was not specified")
	}
	
	tsk.WantDubs = true
	tsk.DubsOnly = true
	tsk.Mode = core.Subs2Dubs
	
	return tsk.Routing(ctx)
}
