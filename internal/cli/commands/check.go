package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
)

var checkCmd = &cobra.Command{
	Use:   "check <path>",
	Short: "Validate media library against expectations before processing",
	Long: `Scan a file or directory and check for problems: corrupted files,
missing audio/subtitle languages, duration mismatches, and more.

Use --auto to detect anomalies via internal consistency (no expectations needed).
Use --audio-langs/--sub-langs to verify specific language expectations.
Use --profile to load a saved expectation profile by name.
Both modes can be combined.

Examples:
  langkit check /media/anime/series/ --auto
  langkit check /media/anime/series/ --profile "Thai anime"
  langkit check /media/anime/series/ --audio-langs ja --sub-langs ja,en
  langkit check /media/anime/ --auto --profile "JP drama"
  langkit check /media/anime/ --auto --quorum 80 --json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

		profileName, _ := cmd.Flags().GetString("profile")
		audioLangs, _ := cmd.Flags().GetStringSlice("audio-langs")
		subLangs, _ := cmd.Flags().GetStringSlice("sub-langs")
		durTolerance, _ := cmd.Flags().GetFloat64("duration-tolerance")
		requireTags, _ := cmd.Flags().GetBool("require-tags")
		autoMode, _ := cmd.Flags().GetBool("auto")
		quorum, _ := cmd.Flags().GetFloat64("quorum")
		softFloor, _ := cmd.Flags().GetFloat64("soft-floor")
		jsonOutput, _ := cmd.Flags().GetBool("json")
		failOn, _ := cmd.Flags().GetString("fail-on")
		decodeDepthStr, _ := cmd.Flags().GetString("decode-depth")

		// Build profile only when user explicitly requests one.
		// Without this, --auto alone would still run profile checks
		// against default settings, producing unwanted findings.
		hasExplicitProfile := profileName != "" ||
			len(audioLangs) > 0 || len(subLangs) > 0 ||
			cmd.Flags().Changed("duration-tolerance") ||
			cmd.Flags().Changed("require-tags")

		var profile *core.ExpectationProfile
		if profileName != "" {
			saved, err := core.GetProfile(profileName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading profile: %v\n", err)
				os.Exit(1)
			}
			if saved == nil {
				fmt.Fprintf(os.Stderr, "Error: profile %q not found\n", profileName)
				os.Exit(1)
			}
			p := *saved
			if len(audioLangs) > 0 {
				p.ExpectedAudioLangs = audioLangs
			}
			if len(subLangs) > 0 {
				p.ExpectedSubtitleLangs = subLangs
			}
			if cmd.Flags().Changed("duration-tolerance") {
				p.DurationTolerancePct = durTolerance
			}
			if cmd.Flags().Changed("require-tags") {
				p.RequireLanguageTags = requireTags
			}
			profile = &p
		} else if hasExplicitProfile {
			p := core.DefaultProfile()
			p.ExpectedAudioLangs = audioLangs
			p.ExpectedSubtitleLangs = subLangs
			if cmd.Flags().Changed("duration-tolerance") {
				p.DurationTolerancePct = durTolerance
			}
			if cmd.Flags().Changed("require-tags") {
				p.RequireLanguageTags = requireTags
			}
			profile = &p
		}

		// Auto mode config
		var autoConfig *core.AutoCheckConfig
		if autoMode {
			if quorum < 0 || quorum > 100 {
				fmt.Fprintf(os.Stderr, "Error: --quorum must be between 0 and 100\n")
				os.Exit(1)
			}
			if softFloor < 0 || softFloor > 100 {
				fmt.Fprintf(os.Stderr, "Error: --soft-floor must be between 0 and 100\n")
				os.Exit(1)
			}
			if softFloor > quorum {
				fmt.Fprintf(os.Stderr, "Error: --soft-floor (%.1f) must not exceed --quorum (%.1f)\n", softFloor, quorum)
				os.Exit(1)
			}
			ac := core.DefaultAutoConfig()
			ac.QuorumPct = quorum
			ac.SoftFloorPct = softFloor
			autoConfig = &ac
		}

		// Validate decode depth
		var decodeDepth media.IntegrityDepth
		if decodeDepthStr != "" {
			decodeDepth = media.IntegrityDepth(decodeDepthStr)
			if decodeDepth != media.IntegritySampled && decodeDepth != media.IntegrityFull {
				fmt.Fprintf(os.Stderr, "Error: --decode-depth must be 'sampled' or 'full'\n")
				os.Exit(1)
			}
		}

		ctx := context.Background()
		report, err := core.RunCheck(ctx, path, profile, autoConfig, decodeDepth)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			data, err := core.FormatReportJSON(report)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			fmt.Print(core.FormatReportCLI(report))
		}

		// Exit code based on --fail-on
		switch failOn {
		case "warning":
			if report.ErrorCount > 0 || report.WarningCount > 0 {
				os.Exit(1)
			}
		default: // "error" or unset
			if report.ErrorCount > 0 {
				os.Exit(1)
			}
		}
	},
}

// Profile management subcommands

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage expectation profiles",
}

var profilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved expectation profiles",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		profiles, err := core.LoadProfiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(profiles) == 0 {
			fmt.Println("No saved profiles.")
			return
		}
		for _, p := range profiles {
			fmt.Printf("  %s\n", p.Name)
			if len(p.ExpectedAudioLangs) > 0 {
				fmt.Printf("    audio: %s\n", strings.Join(p.ExpectedAudioLangs, ", "))
			}
			if len(p.ExpectedSubtitleLangs) > 0 {
				fmt.Printf("    subtitles: %s\n", strings.Join(p.ExpectedSubtitleLangs, ", "))
			}
		}
	},
}

var profilesSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save an expectation profile (creates or updates)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		audioLangs, _ := cmd.Flags().GetStringSlice("audio-langs")
		subLangs, _ := cmd.Flags().GetStringSlice("sub-langs")
		durTolerance, _ := cmd.Flags().GetFloat64("duration-tolerance")
		requireTags, _ := cmd.Flags().GetBool("require-tags")

		p := core.DefaultProfile()
		p.Name = name
		p.ExpectedAudioLangs = audioLangs
		p.ExpectedSubtitleLangs = subLangs
		p.DurationTolerancePct = durTolerance
		p.RequireLanguageTags = requireTags

		if err := core.SaveProfile(p); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Profile %q saved.\n", name)
	},
}

var profilesDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a saved expectation profile",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if err := core.DeleteProfile(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Profile %q deleted.\n", name)
	},
}

func init() {
	// Main check command flags
	checkCmd.Flags().String("profile", "",
		"Load a saved expectation profile by name")
	checkCmd.Flags().StringSlice("audio-langs", nil,
		"Expected audio languages (BCP 47 codes, e.g. ja,en)")
	checkCmd.Flags().StringSlice("sub-langs", nil,
		"Expected subtitle languages (BCP 47 codes, e.g. ja,en)")
	checkCmd.Flags().Float64("duration-tolerance", 2.0,
		"Allowed duration deviation between audio/video (%)")
	checkCmd.Flags().Bool("require-tags", true,
		"Warn about tracks without language tags")
	checkCmd.Flags().Bool("auto", false,
		"Enable auto mode: detect anomalies via internal consistency")
	checkCmd.Flags().Float64("quorum", 75.0,
		"Consensus threshold percentage for auto mode (default: 75)")
	checkCmd.Flags().Float64("soft-floor", 20.0,
		"Soft floor percentage for auto mode Info findings (default: 20)")
	checkCmd.Flags().Bool("json", false,
		"Output results as JSON")
	checkCmd.Flags().String("fail-on", "error",
		"Exit non-zero on: 'error' (default) or 'warning'")
	checkCmd.Flags().String("decode-depth", "",
		"Decode integrity depth: 'sampled' (default from settings) or 'full'")

	// Profile save subcommand flags
	profilesSaveCmd.Flags().StringSlice("audio-langs", nil,
		"Expected audio languages (BCP 47 codes, e.g. ja,en)")
	profilesSaveCmd.Flags().StringSlice("sub-langs", nil,
		"Expected subtitle languages (BCP 47 codes, e.g. ja,en)")
	profilesSaveCmd.Flags().Float64("duration-tolerance", 2.0,
		"Allowed duration deviation between audio/video (%)")
	profilesSaveCmd.Flags().Bool("require-tags", true,
		"Warn about tracks without language tags")

	// Wire subcommand hierarchy
	profilesCmd.AddCommand(profilesListCmd)
	profilesCmd.AddCommand(profilesSaveCmd)
	profilesCmd.AddCommand(profilesDeleteCmd)
	checkCmd.AddCommand(profilesCmd)

	RootCmd.AddCommand(checkCmd)
}
