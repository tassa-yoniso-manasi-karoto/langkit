package cmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/subs2cards/pkg/extract"
)

var extractCardsCmd = &cobra.Command{
	Use:   "cards <foreign-subs> [native-subs]",
	Short: "Decompose media into flash cards",
	Long: `This command generates flash cards for an SRS application like
Anki from subtitles and optional associated media content.

Example:
  subs2cards extract cards -m media-content.mp4 foreign.srt native.srt

Based on the given subtitle files and associated media file, the above
command would create the tab-separated file "foreign.tsv" and a directory
"foreign.media/" containing images and audio files. Among other fields,
"foreign.tsv" would have a current, previous and next subtitle item from
both subtitle files, but the timing reference would be "foreign.srt".`,

	Args: argFuncs(cobra.MinimumNArgs(0), cobra.MaximumNArgs(2)),
	Run: func(cmd *cobra.Command, args []string) {
		var foreignSubs, nativeSubs string

		if len(args) > 0 {
			foreignSubs = args[0]
		}
		if len(args) > 1 {
			nativeSubs = args[1]
		}
		targetChan, _ := cmd.Flags().GetInt("chan")
		audiotrack, _ := cmd.Flags().GetInt("a")
		Offset, _     := cmd.Flags().GetInt("offset")
		timeout, _    := cmd.Flags().GetInt("timeout")
		STT, _        := cmd.Flags().GetBool("stt")
		CC, _         := cmd.Flags().GetBool("cc")
		action := extract.Task{
			Log:                  logger,
			Langs:                langs,
			TargetChan:           targetChan,
			Timeout:              timeout,
			STT:                  STT,
			SeparationLib:        sep,
			IsCC:                 CC,
			Offset:               time.Duration(Offset)*time.Millisecond,
			UseAudiotrack:        audiotrack-1,
			TargSubFile:          foreignSubs,
			RefSubFile:           nativeSubs,
			MediaSourceFile:      mediaFile,
			OutputFieldSeparator: "\t",
			OutputFileExtension:  "tsv",
		}
		action.Execute()
	},
}

func init() {
	extractCmd.AddCommand(extractCardsCmd)
}

// https://github.com/spf13/cobra/issues/648#issuecomment-393154805
func argFuncs(funcs ...cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		for _, f := range funcs {
			err := f(cmd, args)
			if err != nil {
				return err
			}
		}
		return nil
	}
}
