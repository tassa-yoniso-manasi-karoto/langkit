
package commands

import (
	"fmt"
	"os"
	"runtime"
	"context"
	
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	
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
		ctx := context.Background()
		tsk := core.NewTask(core.NewCLIHandler(ctx))
		if err := fn(tsk, ctx, cmd, args); err != nil {
			os.Exit(1)
		}
	}
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)
	// FIXME ↓
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.config/langkit/config.yaml)")
	
	// TODO Keep existing flags but don't set defaults - they'll come from config
	RootCmd.PersistentFlags().StringSliceP("langs", "l", nil,
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
	
	subs2cardsCmd.PersistentFlags().Int("w", 1000, "maximum width of screenshot")
	subs2cardsCmd.PersistentFlags().Int("h", 562, "maximum height of screenshot")
	
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

func initConfig() {
	if err := config.InitConfig(cfgFile); err != nil {
		fmt.Println("Error initializing config:", err)
		os.Exit(1)
	}

	// Bind environment variables
	viper.SetEnvPrefix("LANGKIT")
	viper.AutomaticEnv()
	
	// Bind specific environment variables to their config counterparts
	envBindings := map[string]string{
		"REPLICATE_API_KEY": "api_keys.replicate",
		"ASSEMBLYAI_API_KEY": "api_keys.assemblyai",
		"ELEVENLABS_API_KEY": "api_keys.elevenlabs",
		"TARGET_LANG": "target_language",
		"NATIVE_LANG": "native_language",
	}

	for env, conf := range envBindings {
		if err := viper.BindEnv(conf, env); err != nil {
			fmt.Printf("Warning: failed to bind environment variable %s: %v\n", env, err)
		}
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

