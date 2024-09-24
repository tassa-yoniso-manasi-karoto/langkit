package cmd

import "github.com/spf13/cobra"

var mediaFile, targetLang, separator string

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Decompose media for language study",
	Long: `The extract command group decomposes media into flash cards suitable for studying a language, for example.`,
}

func init() {
	extractCmd.PersistentFlags().StringVarP(&mediaFile, "mediafile", "m", "", "media file to decompose")
	extractCmd.PersistentFlags().StringVarP(&targetLang, "target", "t", "", "ISO-639-2 code of the target language to learn (ie. 'en', 'ja', 'es'...)")
	extractCmd.PersistentFlags().StringVarP(&separator, "separator", "s", "", "specifies which source separation library to use to isolate the voice's audio")
	extractCmd.PersistentFlags().Bool("stt", false, "transcribe audio using an online Speech-To-Text API")
	extractCmd.PersistentFlags().Bool("cc", false, "enforce treating the foreign subs as closed captions: strip it of its SDH material to keep only the dialog")
	extractCmd.PersistentFlags().Int("chan", 2, "prefer audiotracks with this number of channels")
	extractCmd.PersistentFlags().Int("timeout", 100*60, "timeout in seconds for the API request. Due to the upload and remote processing they should be set very high.")
	extractCmd.PersistentFlags().Int("offset", 250, "pad timings of audio clips with this offset in millisecond to compensate inaccurate sync between subs and voices")
	// uh? when using extractCmd.PersistentFlags().IntP <-- default negative int is reset to 0. maybe force signed integer?
	extractCmd.PersistentFlags().Int("a", -1, "force selection of the audiotrack at this index which ignores --chan and -t flag (indexing start at 1)")

	rootCmd.AddCommand(extractCmd)
}
