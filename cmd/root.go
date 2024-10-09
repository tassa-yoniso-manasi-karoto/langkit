package cmd

import (
	"fmt"
	"os"
	"time"
	
	"github.com/rs/zerolog"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly}).With().Timestamp().Logger()
	cfgFile, mediafile, sep, STT string
	langs []string
	dubDescr = "Use the foreign subtitle file to create a dubtitle using\n" +
		"transcriptions made by the selected STT service"
	sepDescr = "Make a new audiotrack with voices louder using specified\n" +
		"separation library to isolate the voice's audio"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "langkit <command>",
	Long: `langkit' main purpose is to decompose subtitles and associated
media content into flash cards for an SRS application like Anki.

Example:
  langkit subs2cards media-content.mp4 foreign.srt native.srt`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}


func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel) 
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.langkit.yaml)")
	rootCmd.PersistentFlags().StringSliceVarP(&langs, "langs", "l", []string{},
		"ISO-639-1/3 codes of target language followed by\n" +
			"reference language(s) sorted by preference\n" +
				"(i.e. learning spanish from english → \"es,en\").\n\n" +
					"For each language one subtag can be specified\n" +
						"after a hyphen \"-\" (i.e. pt-BR or zh-Hant).",
	)
	rootCmd.PersistentFlags().Int("chan", 2, "prefer audiotracks with this number of channels\n")
	rootCmd.PersistentFlags().Int("a", -1,
		"force selection of the audiotrack at this index.\n" +
			"Useful for audiotracks missing a language tag.\n" +
				"Overrides --chan and -l flag.\n" +
					"Indexing of audiotracks start at 1.",
	)

	addSharedSTTflags(subs2cardsCmd)
	addSharedSTTflags(dubCmd)
	
	addSharedSepFlags(subs2cardsCmd)
	addSharedSepFlags(enhanceCmd)
	
	rootCmd.AddCommand(enhanceCmd)
	rootCmd.AddCommand(dubCmd)
}

func addSharedSTTflags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&STT, "stt", "", "transcribe audio using specified online Speech-To-Text API")
	cmd.PersistentFlags().Int("stt-to", 45, "timeout in seconds for the request to the STT service.\n")
	cmd.PersistentFlags().Bool("stt-dub", true, dubDescr)
	// FIXME subs2cardsCmd.PersistentFlags().Bool("cc", false, "enforce treating the foreign subs as closed captions: strip it of its SDH material to keep only the dialog")
}

func addSharedSepFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&sep, "sep", "s", "", sepDescr)
	cmd.PersistentFlags().Int("sep-to", 100*60, "timeout in seconds for the request to the voice separation\n" +
		"service. Due to the upload and remote processing it should\n be set very high.")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".subs2srs" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".langkit")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
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

