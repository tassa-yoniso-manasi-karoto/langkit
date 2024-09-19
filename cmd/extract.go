package cmd

import "github.com/spf13/cobra"

var mediaFile, targetLang string

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Decompose media for language study",
	Long: `The extract command group decomposes media into flash cards suitable
for studying a language, for example.`,
}

func init() {
	extractCmd.PersistentFlags().StringVarP(&mediaFile, "mediafile", "m", "", "media file to decompose")
	extractCmd.PersistentFlags().StringVarP(&targetLang, "target-lang", "t", "", "ISO 639 code of the target language to learn (ie. 'en', 'ja', 'es'...)")
	extractCmd.PersistentFlags().Bool("stt", false, "transcribe audio using an online Speech-To-Text API")
	extractCmd.PersistentFlags().Int("chan", 2, "prefer audiotracks with this number of channels")
	extractCmd.PersistentFlags().Int("offset", 250, "Pad timings of audio clips with this offset in millisecond to compensate inaccurate sync between subs and voices")
	// uh? when using extractCmd.PersistentFlags().IntP <-- default negative int is reset to 0. maybe force signed integer?
	extractCmd.PersistentFlags().Int("a", -1, "force selection of the audiotrack at this index which ignores --chan and -t flag (indexing start at 1)")

	rootCmd.AddCommand(extractCmd)
}
