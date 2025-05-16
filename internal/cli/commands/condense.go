package commands

import (
	"context"
	
	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

var condenseCmd = &cobra.Command{
	Use:   "condense <mediafile> <subtitle-file>",
	Short: "Create condensed audio from media and subtitles",
	Long: `This command generates a condensed audio file containing only the dialogue audio segments
based on subtitle timestamps.

Example:
  langkit condense media-content.mp4 subtitles.srt

The above command would extract audio segments from media-content.mp4 corresponding to the
subtitle times in subtitles.srt, and concatenate them into a single condensed audio file
(with the .CONDENSED.ogg extension).

Optionally, you can also enhance the voice audio by using the --enhance flag.`,

	Args: argFuncs(cobra.MinimumNArgs(0), cobra.MaximumNArgs(2)),
	Run: RunWithExit(condense),
}

func init() {
	// Add specific flags for condense command
	condenseCmd.PersistentFlags().Int("offset", 250, "pad before & after the timings of each audio clip\n"+
		"with this offset in millisecond")
	condenseCmd.PersistentFlags().Bool("enhance", false, "also enhance the dialogue audio using voice isolation")
	
	// Add command to root
	RootCmd.AddCommand(condenseCmd)
}

func condense(tsk *core.Task, ctx context.Context, cmd *cobra.Command, args []string) *core.ProcessingError {
	if len(args) == 0 {
		return tsk.Handler.Log(core.Error, "", "this command requires at least one argument: the path to the media file to be processed")
	}

	tsk.ApplyCLIFlags(cmd)
	
	tsk.MediaSourceFile = args[0]
	if len(args) > 1 {
		tsk.TargSubFile = args[1]
	}
	
	// Set the mode to Condense
	tsk.Mode = core.Condense
	
	// Check if enhance flag is set
	enhance, _ := cmd.Flags().GetBool("enhance")
	tsk.WantEnhancedTrack = enhance
	
	// Handle enhance flag
	if enhance {
		if tsk.SeparationLib == "" {
			return tsk.Handler.Log(core.Warn, core.AbortTask, "Enhanced audio requested but no separation library specified. Use --sep to specify a separation library.")
		}
		tsk.Handler.ZeroLog().Info().Msg("Will also enhance dialogue audio using " + tsk.SeparationLib)
	}

	return tsk.Routing(ctx)
}