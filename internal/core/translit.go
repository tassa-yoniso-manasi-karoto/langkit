package core

import (
	"fmt"
	"strings"
	"regexp"
	"context"
	"os"
	"errors"
	"path/filepath"
	"runtime/pprof"
	"time"
	"log"
	
	"github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	//"github.com/schollz/progressbar/v3"
	"github.com/gookit/color"
	
	_ "github.com/tassa-yoniso-manasi-karoto/translitkit"
	common "github.com/tassa-yoniso-manasi-karoto/translitkit/common"
	"github.com/tassa-yoniso-manasi-karoto/go-ichiran"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

var (
	Splitter = common.DefaultSplitter // All providers must accept and return UTF-8.
	reSpacingInARow = regexp.MustCompile(`\s*(.*)\s*`)
	reMultipleSpacesSeq = regexp.MustCompile(`\s+`)
)

// getPprofDir creates and returns a directory for storing pprof data
func getPprofDir() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}
	
	// Create a dedicated pprof directory
	pprofDir := filepath.Join(configDir, "pprof")
	if err := os.MkdirAll(pprofDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create pprof directory: %w", err)
	}
	
	return pprofDir, nil
}

// IsProfiling returns true if CPU profiling is enabled via environment variable
func IsProfiling() bool {
	return os.Getenv("LANGKIT_PROFILE_TRANSLIT") == "1" || 
	       os.Getenv("LANGKIT_PROFILE_TRANSLIT") == "true" ||
	       os.Getenv("LANGKIT_PROFILE_TRANSLIT") == "yes"
}

// startCPUProfile starts CPU profiling and returns the file and error
// CPU profiling only starts if LANGKIT_PROFILE_TRANSLIT environment variable is set
func startCPUProfile(langCode string) (*os.File, error) {
	// Check if profiling is enabled via environment variable
	if !IsProfiling() {
		return nil, nil
	}
	
	pprofDir, err := getPprofDir()
	if err != nil {
		return nil, err
	}
	
	// Create a timestamped filename for the profile
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(pprofDir, fmt.Sprintf("cpu_translit_%s_%s.pprof", langCode, timestamp))
	
	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create CPU profile file: %w", err)
	}
	
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to start CPU profile: %w", err)
	}
	
	return f, nil
}

// stopCPUProfile stops CPU profiling and closes the file
func stopCPUProfile(f *os.File) {
	if f != nil {
		pprof.StopCPUProfile()
		f.Close()
	}
}

// TranslitProvider defines an interface for transliteration providers
// translitkit already acts a layer of abstraction but for selective transliteration
// it is better to access the dedicated lib for a given language directly.
type TranslitProvider interface {
	Initialize(ctx context.Context, tsk *Task) error
	GetTokens(ctx context.Context, text string) (tokenized []string, transliterated []string, err error)
	GetSelectiveTranslit(ctx context.Context, text string, threshold int) (string, error)
	PostProcess(text string) string
	ProviderName() string
}

func (tsk *Task) Transliterate(ctx context.Context, subsFilepath string) *ProcessingError {
	langCode := tsk.Targ.Language.Part3
	
	// Start CPU profiling if enabled via environment variable
	var profileFile *os.File
	if IsProfiling() {
		var err error
		profileFile, err = startCPUProfile(langCode)
		if err != nil {
			// Log error but continue with transliteration
			tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to start CPU profiling for transliteration")
		} else if profileFile != nil {
			tsk.Handler.ZeroLog().Info().Msg("CPU profiling enabled for transliteration")
			defer stopCPUProfile(profileFile)
		}
	}
	
	// Record overall timing - we always collect timings, but only write the profile/summary if profiling is enabled
	startTime := time.Now()
	
	common.BrowserAccessURL = tsk.BrowserAccessURL
	subsFilepathTokenized := strings.TrimSuffix(subsFilepath, ".srt") + "_tokenized.srt"
	subsFilepathTranslit := strings.TrimSuffix(subsFilepath, ".srt") + "_translit.srt"
	
	// Check if transliteration already exists
	if alreadyDone, err := fileExistsAndNotEmpty(subsFilepathTranslit); err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: error checking destination file %s", subsFilepathTranslit))
	} else if alreadyDone {
		tsk.Handler.ZeroLog().Info().
			Bool("file_exists_and_not_empty", alreadyDone).
			Msg("Subtitle were already transliterated previously, continuing...")
		return nil
	}

	// Get the appropriate transliteration provider based on language
	provider, err := GetTranslitProvider(langCode, tsk.RomanizationStyle)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: couldn't get provider for language %s-%s", langCode, tsk.RomanizationStyle))
	}
	
	// Initialize provider - measure performance
	initStartTime := time.Now()
	tsk.Handler.ZeroLog().Warn().Msgf("translit: %s provider initialization starting, please wait...", provider.ProviderName())
	if err := provider.Initialize(ctx, tsk); err != nil {
		if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: init: operation canceled by user")
		} else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: init: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: failed to init provider for language %s", langCode))
	}
	initDuration := time.Since(initStartTime)
	tsk.Handler.ZeroLog().Info().
		Dur("init_duration", initDuration).
		Msgf("translit: %s successfully initialized", provider.ProviderName())
	
	// Open subtitle files
	SubTranslit, _ := astisub.OpenFile(subsFilepath)
	SubTokenized, _ := astisub.OpenFile(subsFilepath)
	mergedSubsStr, _ := Subs2StringBlock(SubTranslit)
	
	// Get tokens - measure performance
	tokenStartTime := time.Now()
	tokenizeds, translits, err := provider.GetTokens(ctx, mergedSubsStr)
	tokenDuration := time.Since(tokenStartTime)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: tkns: operation canceled by user")
		} else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: tkns: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks, "couldn't get tokens from provider")
	}
	tsk.Handler.ZeroLog().Trace().
		Dur("token_duration", tokenDuration).
		Int("token_count", len(tokenizeds)).
		Msgf("translit: %s returned tokens", provider.ProviderName())
	
	// Get selective transliteration if supported (for Japanese) - measure performance
	var SubSelective *astisub.Subtitles
	var mergedSubsStrSelective string
	var selectiveDuration time.Duration
	
	if langCode == "jpn" && tsk.KanjiThreshold > -1 {
		SubSelective, _ = astisub.OpenFile(subsFilepath)
		subsFilepathSelective := strings.TrimSuffix(subsFilepath, ".srt") + "_selective.srt"
		
		selectiveStartTime := time.Now()
		mergedSubsStrSelective, err = provider.GetSelectiveTranslit(ctx, mergedSubsStr, tsk.KanjiThreshold)
		selectiveDuration = time.Since(selectiveStartTime)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: selectiveTranslit: operation canceled by user")
			} else if errors.Is(err, context.DeadlineExceeded) {
				return tsk.Handler.LogErr(err, AbortTask, "translit: selectiveTranslit: operation timed out.")
			}
			return tsk.Handler.LogErr(err, AbortAllTasks, "couldn't get selective transliteration")
		}
		tsk.Handler.ZeroLog().Trace().
			Dur("selective_duration", selectiveDuration).
			Msg("Selective transliteration completed")
	}
	
	tsk.Handler.ZeroLog().Trace().Msg("Tokenization/transliteration query finished")
	
	// TODO this convoluted replacement system of processed word on the original subtitle line
	//  was designed to workaround the fact that some providers trimmed non-lexical elements such as
	// punctuation and therefore deformed the original string's format but after recent updates on
	// translitkit I am not sure whether it's still needed
	
	// Common replacement logic - measure performance
	replaceStartTime := time.Now()
	mergedSubsStrTranslit := mergedSubsStr
	for i, tokenized := range tokenizeds {
		translit := translits[i]
		mergedSubsStrTranslit = strings.Replace(mergedSubsStrTranslit, tokenized, translit+" ", 1)
	}
	
	mergedSubsStrTokenized := mergedSubsStrTranslit
	for i, tokenized := range tokenizeds {
		translit := translits[i]
		mergedSubsStrTokenized = strings.Replace(mergedSubsStrTokenized, translit, tokenized, 1)
	}
	replaceDuration := time.Since(replaceStartTime)
	tsk.Handler.ZeroLog().Trace().
		Dur("replace_duration", replaceDuration).
		Msg("Replacement operations completed")
	
	// Split results - measure performance
	splitStartTime := time.Now()
	idx := 0
	subSliceTranslit := strings.Split(mergedSubsStrTranslit, Splitter)
	subSliceTokenized := strings.Split(mergedSubsStrTokenized, Splitter)
	
	// Add selective slice for Japanese if available
	var subSliceSelective []string
	if langCode == "jpn" && tsk.KanjiThreshold > -1 && mergedSubsStrSelective != "" {
		subSliceSelective = strings.Split(mergedSubsStrSelective, Splitter)
		
		tsk.Handler.ZeroLog().Trace().
			Int("len(subSliceTranslit)", len(subSliceTranslit)).
			Int("len(subSliceTokenized)", len(subSliceTokenized)).
			Int("len(subSliceSelective)", len(subSliceSelective)).
			Dur("split_duration", time.Since(splitStartTime)).
			Msg("")
	} else {
		tsk.Handler.ZeroLog().Trace().
			Int("len(subSliceTranslit)", len(subSliceTranslit)).
			Int("len(subSliceTokenized)", len(subSliceTokenized)).
			Dur("split_duration", time.Since(splitStartTime)).
			Msg("")
	}
	splitDuration := time.Since(splitStartTime)
	
	// Apply changes to subtitles - measure performance
	applyStartTime := time.Now()
	for i := range (*SubTranslit).Items {
		for j := range (*SubTranslit).Items[i].Lines {
			for k := range (*SubTranslit).Items[i].Lines[j].Items {
				// FIXME: Trimmed closed captions have some sublines removed, hence must adjust idx
				(*SubTokenized).Items[i].Lines[j].Items[k].Text = clean(subSliceTokenized[idx])
				
				// Process transliteration
				if langCode == "jpn" {
					(*SubTranslit).Items[i].Lines[j].Items[k].Text = subSliceTranslit[idx]
				} else {
					(*SubTranslit).Items[i].Lines[j].Items[k].Text = provider.PostProcess(subSliceTranslit[idx])
				}
				
				// Add selective transliteration for Japanese
				if langCode == "jpn" && tsk.KanjiThreshold > -1 && SubSelective != nil {
					(*SubSelective).Items[i].Lines[j].Items[k].Text = subSliceSelective[idx]
				}
				
				idx++
			}
		}
	}
	applyDuration := time.Since(applyStartTime)
	tsk.Handler.ZeroLog().Trace().
		Dur("apply_duration", applyDuration).
		Int("len(SubTokenized.Items)", len(SubTokenized.Items)).
		Int("len(SubTranslit.Items)", len(SubTranslit.Items)).
		Msg("")
	
	// Write output files - measure performance
	writeStartTime := time.Now()
	if err := SubTokenized.Write(subsFilepathTokenized); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write tokenized subtitles")
	}
	if err := SubTranslit.Write(subsFilepathTranslit); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write transliterated subtitles")
	}
	
	// Write selective transliteration for Japanese if needed
	var subsFilepathSelective string
	if langCode == "jpn" && tsk.KanjiThreshold > -1 && SubSelective != nil {
		subsFilepathSelective = strings.TrimSuffix(subsFilepath, ".srt") + "_selective.srt"
		if err := SubSelective.Write(subsFilepathSelective); err != nil {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write selectively transliterated subtitles")
		}
	}
	writeDuration := time.Since(writeStartTime)
	
	// Log total performance statistics
	totalDuration := time.Since(startTime)
	tsk.Handler.ZeroLog().Info().
		Str("language", langCode).
		Dur("total_duration", totalDuration).
		Dur("init_duration", initDuration).
		Dur("token_duration", tokenDuration).
		Dur("replacement_duration", replaceDuration).
		Dur("split_duration", splitDuration).
		Dur("apply_duration", applyDuration).
		Dur("write_duration", writeDuration).
		Int("token_count", len(tokenizeds)).
		Msg("Transliteration performance metrics")
	
	// Create a performance summary text file if profiling is enabled
	if IsProfiling() {
		pprofDir, err := getPprofDir()
		if err == nil {
			timestamp := time.Now().Format("20060102-150405")
			summaryFile, err := os.Create(filepath.Join(pprofDir, fmt.Sprintf("translit_summary_%s_%s.txt", langCode, timestamp)))
			if err == nil {
				defer summaryFile.Close()
				fmt.Fprintf(summaryFile, "Transliteration Performance Summary\n")
				fmt.Fprintf(summaryFile, "==============================\n\n")
				fmt.Fprintf(summaryFile, "Language: %s\n", langCode)
				fmt.Fprintf(summaryFile, "Provider: %s\n", provider.ProviderName())
				fmt.Fprintf(summaryFile, "Input file: %s\n", subsFilepath)
				fmt.Fprintf(summaryFile, "Output files:\n")
				fmt.Fprintf(summaryFile, "  - %s\n", subsFilepathTokenized)
				fmt.Fprintf(summaryFile, "  - %s\n", subsFilepathTranslit)
				if langCode == "jpn" && tsk.KanjiThreshold > -1 {
					fmt.Fprintf(summaryFile, "  - %s\n", subsFilepathSelective)
				}
				fmt.Fprintf(summaryFile, "\nToken count: %d\n", len(tokenizeds))
				fmt.Fprintf(summaryFile, "\nPerformance Breakdown:\n")
				fmt.Fprintf(summaryFile, "---------------------\n")
				fmt.Fprintf(summaryFile, "Total duration:       %v\n", totalDuration)
				fmt.Fprintf(summaryFile, "Provider init:        %v (%.1f%%)\n", initDuration, float64(initDuration)/float64(totalDuration)*100)
				fmt.Fprintf(summaryFile, "Token extraction:     %v (%.1f%%)\n", tokenDuration, float64(tokenDuration)/float64(totalDuration)*100)
				if langCode == "jpn" && tsk.KanjiThreshold > -1 {
					fmt.Fprintf(summaryFile, "Selective translit:   %v (%.1f%%)\n", selectiveDuration, float64(selectiveDuration)/float64(totalDuration)*100)
				}
				fmt.Fprintf(summaryFile, "String replacements:  %v (%.1f%%)\n", replaceDuration, float64(replaceDuration)/float64(totalDuration)*100)
				fmt.Fprintf(summaryFile, "String splitting:     %v (%.1f%%)\n", splitDuration, float64(splitDuration)/float64(totalDuration)*100)
				fmt.Fprintf(summaryFile, "Applying text:        %v (%.1f%%)\n", applyDuration, float64(applyDuration)/float64(totalDuration)*100)
				fmt.Fprintf(summaryFile, "File writing:         %v (%.1f%%)\n", writeDuration, float64(writeDuration)/float64(totalDuration)*100)
				fmt.Fprintf(summaryFile, "\nToken processing rate: %.2f tokens/second\n", float64(len(tokenizeds))/totalDuration.Seconds())
			}
		}
	}
	
	tsk.Handler.ZeroLog().Debug().Msg("Foreign subs were transliterated")
	return nil
}

func Subs2StringBlock(subs *astisub.Subtitles) (mergedSubsStr string, subSlice []string) {
	for _, Item := range (*subs).Items {
		for _, Line := range Item.Lines {
			for _, LineItem := range Line.Items {
				subSlice = append(subSlice, LineItem.Text)
				mergedSubsStr += LineItem.Text +Splitter
			}
		}
	}
	return
}



// GenericProvider implements the transliteration for most languages
type GenericProvider struct {
	module *common.SchemeModule
}

func NewGenericProvider(lang string, style string) (*GenericProvider, error) {
	m, err := common.GetSchemeModule(lang, style)
	if err != nil {
		return nil, err
	}
	return &GenericProvider{module: m}, nil
}

func (p *GenericProvider) Initialize(ctx context.Context, tsk *Task) error {
	p.module.WithContext(ctx)
	
	if !tsk.DockerRecreate {
		return p.module.Init()
	} 
	return p.module.InitRecreate(true)
}

func (p *GenericProvider) GetTokens(ctx context.Context, text string) ([]string, []string, error) {
	tokens, err := p.module.Tokens(text)
	if err != nil {
		return nil, nil, err
	}
	return tokens.TokenizedParts(), tokens.RomanParts(), nil
}

func (p *GenericProvider) GetSelectiveTranslit(ctx context.Context, text string, threshold int) (string, error) {
	// Most languages don't support selective transliteration
	return "", nil
}

func (p *GenericProvider) PostProcess(text string) string {
	return p.module.RomanPostProcess(text, func(s string) string { return s })
}

func (p *GenericProvider) ProviderName() string {
	return fmt.Sprintf("%s-%s", p.module.Lang, p.module.ProviderNames())
}

// JapaneseProvider handles Japanese-specific transliteration
type JapaneseProvider struct{}

func NewJapaneseProvider() *JapaneseProvider {
	return &JapaneseProvider{}
}

func (p *JapaneseProvider) Initialize(ctx context.Context, tsk *Task) error {
	ichiran.Ctx = ctx
	
	if !tsk.DockerRecreate {
		return ichiran.Init()
	}
	return ichiran.InitRecreate(true)
}

func (p *JapaneseProvider) GetTokens(ctx context.Context, text string) ([]string, []string, error) {
	tokens, err := ichiran.Analyze(text)
	if err != nil {
		return nil, nil, err
	}
	return tokens.TokenizedParts(), tokens.RomanParts(), nil
}

func (p *JapaneseProvider) GetSelectiveTranslit(ctx context.Context, text string, threshold int) (string, error) {
	if threshold > -1 {
		tokens, err := ichiran.Analyze(text)
		if err != nil {
			return "", err
		}
		return tokens.SelectiveTranslit(threshold)
	}
	return "", nil
}

func (p *JapaneseProvider) PostProcess(text string) string {
	// Japanese doesn't need post-processing for transliteration
	return text
}

func (p *JapaneseProvider) ProviderName() string {
	return "jpn-ichiran"
}

// GetTranslitProvider returns the appropriate provider based on language
func GetTranslitProvider(lang string, style string) (TranslitProvider, error) {
	if lang == "jpn" {
		return NewJapaneseProvider(), nil
	}
	
	return NewGenericProvider(lang, style)
}



func fileExistsAndNotEmpty(filepath string) (bool, error) {
        fileInfo, err := os.Stat(filepath)
        if os.IsNotExist(err) {
                return false, nil // File does not exist, not an error
        }
        if err != nil {
                return false, err // Other errors (permissions, etc.)
        }

        return fileInfo.Size() > 0, nil
}



// // FIXME transcoding srt into ass causes astisub runtime panic, no sure if supported or not
// func WriteASS(filepath string, subtitles *astisub.Subtitles) error {
// 	// Create the output file
// 	outputFile, err := os.Create(filepath)
// 	if err != nil {
// 		return fmt.Errorf("failed to create file: %w", err)
// 	}
// 	defer outputFile.Close()

// 	// Write the subtitles to ASS format
// 	if err := subtitles.WriteToSSA(outputFile); err != nil {
// 		return fmt.Errorf("failed to write subtitles to ASS format: %w", err)
// 	}

// 	return nil
// }


func clean(s string) string{
	return reMultipleSpacesSeq.ReplaceAllString(strings.TrimSpace(s), " ")
}

func placeholder2345432() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
