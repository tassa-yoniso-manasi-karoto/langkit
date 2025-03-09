
package commands

import (
	"fmt"
	"strings"
	"context"
	
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
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
		defer func() {
			if r := recover(); r != nil {
				exitOnError(tsk, fmt.Errorf("panic: %v", r))
			}
		}()
		if err := fn(tsk, ctx, cmd, args); err != nil {
			exitOnError(tsk, err)
		}
	}
}


func exitOnError(tsk *core.Task, mainErr error) {
	tsk.Handler.ZeroLog().Warn().
		Err(mainErr).
		Msgf("An error occured, creating a crash report at %s", crash.GetCrashDir())

	settings, err := config.LoadSettings()
	if err != nil {
		// Continue with empty settings if loading fails
		fmt.Printf("Warning: Failed to load settings: %v\n", err)
	}
	
	crashPath, err := crash.WriteReport(crash.ModeCrash, mainErr, settings, tsk.Handler.GetLogBuffer(), true)
	if err != nil {
		color.Redf("failed to write crash report: %w", err)
	}
	tsk.Handler.ZeroLog().Fatal().
		Err(mainErr).
		Str("report_path", crashPath).
		Msg("An error occured, exiting...")
}


// Early load the settings before flag initialization
var settings config.Settings

func init() {
	// Load settings first, before initializing flags
	var err error
	// Initialize config with empty path (use default)
	if err := config.InitConfig(""); err != nil {
		fmt.Printf("Warning: Could not initialize config: %v\n", err)
	}
	
	// Load settings for flag defaults
	settings, err = config.LoadSettings()
	if err != nil {
		fmt.Printf("Warning: Could not load settings: %v\n", err)
	}
	
	// Initialize cobra with our loaded settings
	initCommandsWithSettings()
	
	// Setup config load during command execution
	cobra.OnInitialize(initConfig)
}

// initCommandsWithSettings initializes commands using settings from config
func initCommandsWithSettings() {
	// Initialize flags with values from config or defaults if config loading failed
	
	// TODO Convert languages to string slice for flag default
	/*langs := []string{}
	if settings.TargetLanguage != "" {
		langs = append(langs, settings.TargetLanguage)
		if settings.NativeLanguages != "" {
			langs = append(langs, convertLanguagesString(settings.NativeLanguages)...)
		}
	}*/
	
	// Set flags with proper defaults
	RootCmd.PersistentFlags().StringSliceP("langs", "l", /*TODO*/ nil,
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

// TODO Helper function to convert comma-separated languages string to slice
func convertLanguagesString(langsStr string) []string {
	if langsStr == "" {
		return []string{}
	}
	
	// This would use TagsStr2TagsArr from your existing code
	// For the sake of this example:
	return strings.Split(langsStr, ",")
}

func initConfig() {
	// Setup environment variables
	viper.SetEnvPrefix("LANGKIT")
	viper.AutomaticEnv()
	
	// Bind specific environment variables to their config counterparts
	envBindings := map[string]string{
		"REPLICATE_API_KEY": "api_keys.replicate",
		"ASSEMBLYAI_API_KEY": "api_keys.assemblyai",
		"ELEVENLABS_API_KEY": "api_keys.elevenlabs",
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

