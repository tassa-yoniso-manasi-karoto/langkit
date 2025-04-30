
package commands

import (
	"fmt"
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"strings"
	
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
	if mainErr == nil {
		return
	}

	// Check for common ordinary errors that don't require crash reports
	if isOrdinaryError(mainErr) {
		tsk.Handler.ZeroLog().Trace().
			Err(mainErr).
			Msg("Operation failed with an ordinary error")
		
		color.Yellowf("Error: %v\n", mainErr)
		return
	}

	// Handle critical errors with crash reports
	tsk.Handler.ZeroLog().Warn().
		Err(mainErr).
		Msgf("An error occurred, creating a crash report at %s", crash.GetCrashDir())
	
	settings, err := config.LoadSettings()
	if err != nil {
		fmt.Printf("Warning: Failed to load settings: %v\n", err)
	}
	
	crashPath, err := crash.WriteReport(crash.ModeCrash, mainErr, settings, tsk.Handler.GetLogBuffer(), true)
	if err != nil {
		color.Redf("Failed to write crash report: %v\n", err)
	}
	
	tsk.Handler.ZeroLog().Fatal().
		Err(mainErr).
		Str("report_path", crashPath).
		Msg("A critical error occurred, exiting...")
}


// Early load the settings before flag initialization by cobra
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
	
	// Convert languages to string slice for flag default
	langs := []string{}
	if settings.TargetLanguage != "" {
		langs = append(langs, settings.TargetLanguage)
		if settings.NativeLanguages != "" {
			langs = append(langs, core.TagsStr2TagsArr(settings.NativeLanguages)...)
		}
	}
	
	// Set flags with proper defaults
	RootCmd.PersistentFlags().StringSliceP("langs", "l", langs,
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
	RootCmd.PersistentFlags().Int("workers", settings.MaxWorkers, "max concurrent workers to use for bulk processing")

	RootCmd.PersistentFlags().StringP("sep", "s", "", "separation API to use for voice isolation")
	RootCmd.PersistentFlags().Int("sep-to", 15*60, "timeout in seconds for the voice separation request")
	
	subs2cardsCmd.PersistentFlags().Int("w", 1000, "maximum width of screenshot")
	subs2cardsCmd.PersistentFlags().Int("h", 562, "maximum height of screenshot")
	
	addSharedSTTflags(subs2cardsCmd)
	addSharedSTTflags(subs2dubsCmd)

	addSharedTranslitFlags(subs2cardsCmd, true)
	addSharedTranslitFlags(translitCmd, false)
	
	RootCmd.AddCommand(enhanceCmd)
	RootCmd.AddCommand(subs2dubsCmd)
	RootCmd.AddCommand(subs2cardsCmd)
	RootCmd.AddCommand(translitCmd)
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
		"OPENAI_API_KEY": "api_keys.openai",
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



// isOrdinaryError checks if an error is a common non-critical error that doesn't warrant a crash report
func isOrdinaryError(err error) bool {
	// File not found errors
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
		return true
	}

	// Permission errors
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, os.ErrPermission) {
		return true
	}

	// Network-related errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		// DNS resolution errors, timeout errors, connection refused
		return netErr.Timeout() || strings.Contains(err.Error(), "no such host") || 
			strings.Contains(err.Error(), "connection refused") || 
			strings.Contains(err.Error(), "network is unreachable")
	}

	// IO errors that are common
	if strings.Contains(err.Error(), "broken pipe") || 
		strings.Contains(err.Error(), "connection reset by peer") {
		return true
	}

	// User input validation errors (assuming you have some pattern for these)
	if strings.Contains(err.Error(), "invalid input") || 
		strings.Contains(err.Error(), "validation failed") {
		return true
	}

	// Configuration errors
	// if strings.Contains(err.Error(), "configuration") || 
	// 	strings.Contains(err.Error(), "config file") {
	// 	return true
	// }

	// EOF errors (often normal in file processing)
	if errors.Is(err, io.EOF) {
		return true
	}

	return false
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

