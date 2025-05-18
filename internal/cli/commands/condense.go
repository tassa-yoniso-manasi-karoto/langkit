package commands

import (
	"context"
	"fmt" // Added for error formatting

	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

var condenseCmd = &cobra.Command{
	Use:   "condense <mediafile> [subtitle-file]", // Made subtitle-file optional
	Short: "Create condensed audio from media and (optionally) subtitles",
	Long: `This command generates a condensed audio file containing only the dialogue audio segments.
If a subtitle file is provided, its timestamps are used. If not, an attempt might be made
to use other methods for dialogue detection (though this is not yet implemented - subtitles are currently required by the core logic).

Example:
  langkit condense media-content.mp4 subtitles.srt
  langkit condense media-content.mp4 subtitles.srt --summary --summary-provider openai --summary-model gpt-4o

The first command extracts audio segments from media-content.mp4 corresponding to the
subtitle times in subtitles.srt, and concatenates them into a single condensed audio file.
The second command does the same but also generates a summary using the specified LLM
and embeds it into the condensed audio file's metadata.

Optionally, you can also enhance the voice audio by using the --enhance flag.`,

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(2)), // Media file is mandatory
	Run:  RunWithExit(condense),
}

func init() {
	// Add specific flags for condense command
	condenseCmd.PersistentFlags().Int("offset", 0, "Pad before & after the timings of each audio clip (milliseconds). Note: Currently only affects OGG extraction for other modes, not WAV for condensed audio.") // Default 0 as WAV extraction for condensed audio uses 0 offset.
	condenseCmd.PersistentFlags().Bool("enhance", false, "Also enhance the dialogue audio using voice isolation")

	// Flags for summary generation
	condenseCmd.PersistentFlags().Bool("summary", false, "Generate a summary and add it to condensed audio metadata")
	condenseCmd.PersistentFlags().String("summary-provider", "", "LLM provider for summary (e.g., openai, google-gemini, openrouter)")
	condenseCmd.PersistentFlags().String("summary-model", "", "LLM model for summary (e.g., gpt-4o, models/gemini-1.5-pro-latest)")
	condenseCmd.PersistentFlags().String("summary-custom-prompt", "", "Custom prompt for summarization. Subtitle text will be appended.")
	condenseCmd.PersistentFlags().Int("summary-max-words", 300, "Approximate target maximum word count for the summary")
	condenseCmd.PersistentFlags().Float64("summary-temperature", 0.7, "Temperature for summary generation (0.0-2.0, -1 for LLM default)")

	// Add command to root
	RootCmd.AddCommand(condenseCmd)
}

func condense(tsk *core.Task, ctx context.Context, cmd *cobra.Command, args []string) *core.ProcessingError {
	// Media file is always args[0] due to MinimumNArgs(1)
	tsk.MediaSourceFile = args[0]

	if len(args) > 1 {
		tsk.TargSubFile = args[1]
	} else {
		// If no subtitle file is provided, Autosub will be attempted by tsk.Routing -> tsk.Execute -> tsk.setupSubtitles
		// The core logic in item.go (ConcatWAVstoOGG) checks if tsk.TargSubs is nil.
		// If Autosub fails to find a subtitle, tsk.TargSubs will remain nil, and summary/condensed audio (which needs subs) won't proceed.
		tsk.Handler.ZeroLog().Info().Msg("No subtitle file provided for condense command. Will attempt Autosub.")
	}

	// Apply global CLI flags first (e.g., -l for languages, --sep for separation lib)
	// This will set tsk.NativeLang among other things.
	if procErr := tsk.ApplyCLIFlags(cmd); procErr != nil {
		return procErr
	}

	// Set the mode to Condense
	tsk.Mode = core.Condense

	// Handle --enhance flag
	enhance, _ := cmd.Flags().GetBool("enhance")
	tsk.WantEnhancedTrack = enhance
	if enhance {
		if tsk.SeparationLib == "" {
			return tsk.Handler.Log(core.Warn, core.AbortTask, "Enhanced audio requested (--enhance) but no separation library specified. Use --sep to specify a separation library (e.g., --sep demucs).")
		}
		tsk.Handler.ZeroLog().Info().Msg("Will also enhance dialogue audio using " + tsk.SeparationLib)
	}

	// Handle summary flags
	wantSummary, _ := cmd.Flags().GetBool("summary")
	tsk.WantSummary = wantSummary

	if tsk.WantSummary {
		tsk.SummaryProvider, _ = cmd.Flags().GetString("summary-provider")
		tsk.SummaryModel, _ = cmd.Flags().GetString("summary-model")
		tsk.SummaryCustomPrompt, _ = cmd.Flags().GetString("summary-custom-prompt")
		tsk.SummaryMaxLength, _ = cmd.Flags().GetInt("summary-max-words")
		tsk.SummaryTemperature, _ = cmd.Flags().GetFloat64("summary-temperature")

		// Validate that provider and model are set if summary is requested
		if tsk.SummaryProvider == "" {
			return tsk.Handler.Log(core.Error, core.AbortTask, "Summary generation requested (--summary) but --summary-provider is not specified.")
		}
		if tsk.SummaryModel == "" {
			return tsk.Handler.Log(core.Error, core.AbortTask, "Summary generation requested (--summary) but --summary-model is not specified.")
		}

		// tsk.SummaryOutputLang will be set in item.go based on tsk.NativeLang
		// If tsk.NativeLang is not set (e.g. user didn't provide -l flag),
		// item.go will pass an empty string for outputLangName, and the LLM will default (usually to English or source lang).
		// This is consistent with how summary options are handled.
		logMsg := fmt.Sprintf("Summary generation enabled. Provider: %s, Model: %s", tsk.SummaryProvider, tsk.SummaryModel)
		if tsk.SummaryCustomPrompt != "" {
			logMsg += " (using custom prompt)"
		}
		tsk.Handler.ZeroLog().Info().Msg(logMsg)
	}

	return tsk.Routing(ctx)
}