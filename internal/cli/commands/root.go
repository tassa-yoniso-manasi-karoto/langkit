
package commands

import (
	"fmt"
	"os"
	"runtime"
	"context"
	
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	homedir "github.com/mitchellh/go-homedir" // FIXME migrate to XDG
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "langkit <command>",
	Long: `langkit's main purpose is to decompose subtitles and associated
media content into flash cards for an SRS application like Anki.

Example:
  langkit subs2cards media-content.mp4 foreign.srt native.srt`,
}

type RunFunc func(tsk *core.Task, ctx context.Context, cmd *cobra.Command, args []string) *core.ProcessingError

func RunWithExit(fn RunFunc) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		tsk := core.NewTask(core.NewCLIHandler())
		if err := fn(tsk, context.Background(), cmd, args); err != nil {
			os.Exit(1)
		}
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	
	RootCmd.PersistentFlags().StringP("config", "c", "", "config file (default is $HOME/.langkit.yaml)")
	RootCmd.PersistentFlags().StringSliceP("langs", "l", []string{},
		"ISO-639-1/3 codes of target language followed by\n" +
			"reference language(s) sorted by preference\n" +
				"(i.e. learning spanish from english → \"es,en\").\n\n" +
					"For each language one subtag can be specified\n" +
						"after a hyphen \"-\" (i.e. pt-BR or zh-Hant).",
	)
	RootCmd.PersistentFlags().Int("chan", 2, "prefer audiotracks with this number of channels\n")
	RootCmd.PersistentFlags().Int("a", -1,
		"force selection of the audiotrack at this index.\n" +
			"Useful for audiotracks missing a language tag.\n" +
				"Overrides --chan and -l flag.\n" +
					"Indexing of audiotracks start at 1.",
	)
	RootCmd.PersistentFlags().String("ffmpeg", "ffmpeg", "override for the path to FFmpeg binary\n")
	RootCmd.PersistentFlags().String("mediainfo", "mediainfo", "override for the path to Mediainfo binary\n")
	RootCmd.PersistentFlags().Int("workers", runtime.NumCPU()-1, "max concurrent workers to use for bulk processing")

	RootCmd.PersistentFlags().StringP("sep", "s", "", "separation API to use for voice isolation")
	RootCmd.PersistentFlags().Int("sep-to", 15*60, "timeout in seconds for the voice separation request")
	
	addSharedSTTflags(subs2cardsCmd)
	addSharedSTTflags(subs2dubsCmd)

	addSharedTranslitFlags(subs2cardsCmd, true)
	addSharedTranslitFlags(enhanceCmd, true)
	addSharedTranslitFlags(translitCmd, false)
	
	RootCmd.AddCommand(enhanceCmd)
	RootCmd.AddCommand(subs2dubsCmd)
	RootCmd.AddCommand(subs2cardsCmd)
	RootCmd.AddCommand(translitCmd)
}

// TODO FIXME initConfig reads in config file and ENV variables if set.
func initConfig() {
	cfgFile, _ := RootCmd.Flags().GetString("config")
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".langkit")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func addSharedSTTflags(cmd *cobra.Command) {
	cmd.PersistentFlags().Int("offset", 250, "pad before & after the timings of each audio clip\n"+
		"with this offset in millisecond")
	cmd.PersistentFlags().String("stt", "", "transcribe audio using specified\nonline Speech-To-Text API")
	cmd.PersistentFlags().Int("stt-to", 90, "timeout in seconds for the STT service request\n")
	cmd.PersistentFlags().Bool("stt-dub", true, "create dubtitles from STT transcriptions")
}

func addSharedTranslitFlags(cmd *cobra.Command, notTranslitSubcommand bool) {
	if notTranslitSubcommand {
		cmd.PersistentFlags().Bool("translit", false, "transliterate and tokenize the subtitle file or the newly created dubtitle file")
	}
	cmd.PersistentFlags().Int("translit-to", 90, "timeout in seconds for the transliteration service request\n")
	cmd.PersistentFlags().String("browser-access-url", "", "websocket URL for DevTools remote debugging")
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

