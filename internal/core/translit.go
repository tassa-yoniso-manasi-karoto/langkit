package core

import (
	"fmt"
	"strings"
	"regexp"
	"context"
	"os"
	"errors"
	"path/filepath"
	"time"
	"math"
	"unicode/utf8"
	"slices"
	
	"github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	//"github.com/schollz/progressbar/v3"
	"github.com/gookit/color"
	
	_ "github.com/tassa-yoniso-manasi-karoto/translitkit"
	common "github.com/tassa-yoniso-manasi-karoto/translitkit/common"
	"github.com/tassa-yoniso-manasi-karoto/go-ichiran"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/profiling"
)

var (
	Splitter = common.DefaultSplitter // All providers must accept and return UTF-8.
	reSpacingInARow = regexp.MustCompile(`\s*(.*)\s*`)
)

// CreateMockTransliterationFiles creates mock transliteration files for testing
func CreateMockTransliterationFiles(subsFilepath string, types []TranslitType) ([]string, error) {
	// Only run in test mode with mock providers
	if os.Getenv("LANGKIT_USE_MOCK_PROVIDERS") != "true" {
		return nil, nil
	}
	
	outputs := []string{}
	baseDir := filepath.Dir(subsFilepath)
	baseName := strings.TrimSuffix(filepath.Base(subsFilepath), filepath.Ext(subsFilepath))
	
	// Create mock files for each requested type
	for _, tlitType := range types {
		outputPath := filepath.Join(baseDir, baseName + tlitType.ToSuffix())
		
		// Create a simple mock subtitle file
		srtContent := `1
00:00:01,000 --> 00:00:04,000
Mock ` + tlitType.String() + ` content line 1

2
00:00:05,000 --> 00:00:08,000
Mock ` + tlitType.String() + ` content line 2

3
00:00:09,000 --> 00:00:12,000
[Mock ` + tlitType.String() + ` of ` + filepath.Base(subsFilepath) + `]
`
		err := os.WriteFile(outputPath, []byte(srtContent), 0644)
		if err != nil {
			return outputs, fmt.Errorf("failed to write %s file: %w", tlitType.String(), err)
		}
		
		outputs = append(outputs, outputPath)
		fmt.Printf("Created mock transliteration file: %s\n", outputPath)
	}
	
	return outputs, nil
}

var (
	reMultipleSpacesSeq = regexp.MustCompile(`\s+`)
)


type TranslitType int

const (
	Tokenize  TranslitType = iota
	Romanize
	Selective
)

func (m TranslitType) String() string{
	return []string{"tokenized", "romanized", "selective"}[m]
}

func (m TranslitType) ToSuffix() string {
	return "_" + m.String() + ".srt"
}

// TranslitProvider defines an interface for transliteration providers
// translitkit already acts a layer of abstraction but for selective transliteration
// it is better to access the dedicated lib for a given language directly.
type TranslitProvider interface {
	Initialize(ctx context.Context, tsk *Task) error
	GetTokens(ctx context.Context, text string, handler MessageHandler) (tokenized []string, transliterated []string, err error)
	GetSelectiveTranslit(ctx context.Context, threshold int) (string, error)
	PostProcess(text string) string
	ProviderName() string
}


func (tsk *Task) Transliterate(ctx context.Context, subsFilepath string) *ProcessingError {
	langCode := tsk.Targ.Language.Part3
	
	// Check if we're in test mode with mock providers - create mock files directly
	if os.Getenv("LANGKIT_USE_MOCK_PROVIDERS") == "true" {
		tsk.Handler.ZeroLog().Info().Msg("Using mock providers for transliteration in test mode")
		mockFiles, err := CreateMockTransliterationFiles(subsFilepath, tsk.TranslitTypes)
		if err != nil {
			return tsk.Handler.LogErr(err, AbortAllTasks, "Failed to create mock transliteration files")
		}
		
		// Register the mock files with the output registry
		for _, path := range mockFiles {
			tsk.Handler.ZeroLog().Info().Str("mock_file", path).Msg("Created mock transliteration file")
			// Add to output registry if available
			if tsk.MergeOutputFiles {
				featType := "transliteration"
				if strings.HasSuffix(path, Tokenize.ToSuffix()) {
					featType = "tokenization"
				} else if strings.HasSuffix(path, Romanize.ToSuffix()) {
					featType = "romanization"
				} else if strings.HasSuffix(path, Selective.ToSuffix()) {
					featType = "selective_transliteration"
				}
				tsk.RegisterOutputFile(path, "subtitle", tsk.Targ, featType, 0)
			}
		}
		
		return nil
	}
	
	// Start CPU profiling if enabled via environment variable
	var profileFile *os.File
	if WantCPUProfiling() {
		var err error
		profileFile, err = profiling.StartCPUProfile("translit_" + langCode)
		if err != nil {
			// Log error but continue with transliteration
			tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to start CPU profiling for transliteration")
		} else if profileFile != nil {
			tsk.Handler.ZeroLog().Info().Msg("CPU profiling enabled for transliteration")
			defer profiling.StopCPUProfile(profileFile)
		}
	}
	
	// Record overall timing - we always collect timings, but only write the profile/summary if profiling is enabled
	startTime := time.Now()
	
	common.BrowserAccessURL = tsk.BrowserAccessURL
	base := strings.TrimSuffix(subsFilepath, ".srt")

	subsFilepathTokenized := base + Tokenize.ToSuffix()
	subsFilepathTranslit  := base + Romanize.ToSuffix()
	subsFilepathSelective := base + Selective.ToSuffix()
	
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
	
	tsk.Handler.ZeroLog().Warn().Msgf("requesting tokens from %s, please wait...", provider.ProviderName())
	tokenizeds, translits, err := provider.GetTokens(ctx, mergedSubsStr, tsk.Handler)
	tsk.Handler.ZeroLog().Info().Msgf("tokens received from %s", provider.ProviderName())
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
		
		selectiveStartTime := time.Now()
		mergedSubsStrSelective, err = provider.GetSelectiveTranslit(ctx, tsk.KanjiThreshold)
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
	
	for _, out := range []struct {
		ttype TranslitType
		subs  *astisub.Subtitles
		path  string
		outputType MediaOutputType
		priority int
	}{
		{Tokenize,  SubTokenized, subsFilepathTokenized, OutputTokenized, 70},
		{Romanize,  SubTranslit,  subsFilepathTranslit,  OutputRomanized, 80},
		{Selective, SubSelective, subsFilepathSelective, OutputTranslit,  75},
	} {
		// Only proceed if user selected this translit type
		if !slices.Contains(tsk.TranslitTypes, out.ttype) {
			continue
		}

		// For selective transliteration, skip if not Japanese or threshold < 0
		if out.ttype == Selective && (langCode != "jpn" || tsk.KanjiThreshold < 0) {
			continue
		}

		// Attempt writing
		if err := out.subs.Write(out.path); err != nil {
			tsk.Handler.ZeroLog().
				Error().
				Err(err).
				Msgf("Failed to write %s subtitles", out.ttype.String())
		} else {
			tsk.Handler.ZeroLog().
				Info().
				Msgf("Created %s subtitles", out.ttype.String())
			
			// Register the subtitle file for final output merging if merging is enabled
			if tsk.MergeOutputFiles {
				feature := "subtitle" + strings.Title(out.ttype.String())
				tsk.RegisterOutputFile(out.path, out.outputType, tsk.Targ, feature, out.priority)
			}
		}
	}

	writeDuration := time.Since(writeStartTime)
	
	// Log total performance statistics
	totalDuration := time.Since(startTime)
	tsk.Handler.ZeroLog().Debug().
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
	if WantCPUProfiling() || profiling.IsCPUProfilingEnabled() {
		pprofDir, err := profiling.GetPprofDir()
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
		
		// Also write a memory profile if memory profiling is enabled
		if profiling.IsMemoryProfilingEnabled() {
			if err := profiling.WriteMemoryProfile("translit_" + langCode); err != nil {
				tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write memory profile")
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
	module *common.Module
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

func (p *GenericProvider) GetTokens(ctx context.Context, text string, handler MessageHandler) ([]string, []string, error) {
	// Generate a unique task ID for this operation
	taskID := fmt.Sprintf("transliteration-%d", time.Now().UnixNano())
	m := p.module
	nativelyUsesChunks := m.SupportsProgress()

	handler.ZeroLog().Debug().
		Bool("nativelyUsesChunks", nativelyUsesChunks).
		Msg("")
	
	// Create a progress callback function
	progressCallback := func(current, total int) {
		handler.IncrementProgress(
			taskID,
			0,
			total,
			30,
			"Transliterating",
			fmt.Sprintf("Processing text (%d/%d)", current+1, total),
			"h-2",
		)
	}
	
	// Determine whether to use native progress tracking or custom chunkifier
	if !nativelyUsesChunks {
		// For modules without native progress support, use a custom chunkifier
		// Calculate optimal chunk size based on text length
		runeCount := utf8.RuneCountInString(text)
		maxChunkSize, numChunks := calculateChunkSize(runeCount)
	
		handler.ZeroLog().Debug().
			Int("runeCount", runeCount).
			Int("maxChunkSize", maxChunkSize).
			Int("numChunks", numChunks).
			Msgf("using %s with custom chunkifier", p.ProviderName())

		m.WithCustomChunkifier(common.NewChunkifier(maxChunkSize))
	}
	
	m.WithProgressCallback(progressCallback)
	
	
	tokens, err := m.Tokens(text)
	
	if err != nil {
		return nil, nil, fmt.Errorf("error processing text: %w", err)
	}
	
	return tokens.TokenizedParts(), tokens.RomanParts(), nil
}


func (p *GenericProvider) GetSelectiveTranslit(ctx context.Context, threshold int) (string, error) {
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
type JapaneseProvider struct {
	// Cache the tokens to avoid redundant Analyze calls
	tokensSlice  []*ichiran.JSONTokens
}

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


func (p *JapaneseProvider) GetTokens(ctx context.Context, text string, handler MessageHandler) ([]string, []string, error) {
	// Generate a unique task ID for this operation
	taskID := fmt.Sprintf("jp-transliteration-%d", time.Now().UnixNano())
	
	// Calculate optimal chunk size based on text length
	runeCount := utf8.RuneCountInString(text)
	maxChunkSize, numChunks := calculateChunkSize(runeCount)
	
	handler.ZeroLog().Debug().
		Int("runeCount", runeCount).
		Int("maxChunkSize", maxChunkSize).
		Int("numChunks", numChunks).
		Msg("using ichiran with custom chunkifier")
	
	// Split text into chunks using Chunkify
	chunks, err := common.NewChunkifier(maxChunkSize).Chunkify(text)
	if err != nil {
		return nil, nil, fmt.Errorf("error splitting text into chunks: %w", err)
	}
	

	handler.ZeroLog().Trace().
		Int("actualNumChunks", len(chunks)).
		Msg("")
	
	// Initialize result variables
	var allTokenizedParts []string
	var allRomanParts []string
	
	// Process each chunk
	for i, chunk := range chunks {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
			// Continue processing
		}
		handler.ZeroLog().Trace().Msgf("Analyzing chunk %d/%d", i+1, len(chunks))
		tokens, err := p.analyzeText(chunk)
		if err != nil {
			return nil, nil, fmt.Errorf("error analyzing chunk %d: %w", i+1, err)
		}
		
		// Merge results
		allTokenizedParts = append(allTokenizedParts, tokens.TokenizedParts()...)
		allRomanParts = append(allRomanParts, tokens.RomanParts()...)
		
		// Update progress
		handler.IncrementProgress(
			taskID,
			1,
			len(chunks),
			30,
			"Transliterating",
			fmt.Sprintf("Processing Japanese text (%d/%d)", i+1, len(chunks)),
			"h-2",
		)
	}
	
	return allTokenizedParts, allRomanParts, nil
}

// analyzeText ensures we only call ichiran.Analyze once for the same text
func (p *JapaneseProvider) analyzeText(text string) (*ichiran.JSONTokens, error) {
	// Analyze the text and cache the results
	tokens, err := ichiran.Analyze(text)
	if err != nil {
		return nil, err
	}
	
	// Cache the tokens for future use
	p.tokensSlice = append(p.tokensSlice, tokens)
	
	return tokens, nil
}


func (p *JapaneseProvider) GetSelectiveTranslit(ctx context.Context, threshold int) (string, error) {
	if threshold <= -1 || len(p.tokensSlice) == 0 {
		return "", nil
	}
	
	var result strings.Builder
	
	for _, tokens := range p.tokensSlice {
		if tokens == nil {
			continue
		}
		
		transliterated, err := tokens.SelectiveTranslit(threshold)
		if err != nil {
			return "", fmt.Errorf("error applying selective transliteration: %w", err)
		}
		
		result.WriteString(transliterated)
	}
	
	return result.String(), nil
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




// function to determine optimal number of chunks:
// 		-5.383057243541×10⁻⁹*x² + 0.000412359x + 0.405561
// this one is eyeballed using desired number of steps (y-axis value)
// of progressbar and typical rune count (x-axis) I have seen on string made from a subfile, 
// obtained using polynominal regression @ https://www.dcode.fr/function-equation-finder
func calculateChunkSize(runeCount int) (int, int) {
	x := float64(runeCount)
	desiredChunks := -5.383057243541e-9*x*x + 0.000412359*x + 0.405561
	
	// Round to nearest integer and ensure at least 1 chunk
	numChunks := int(math.Round(desiredChunks))
	if numChunks < 1 {
		numChunks = 1
	}
	// plateau at 5 for performance
	if numChunks > 5 {
		numChunks = 5
	}
	
	// Calculate chunk size from desired number of chunks
	chunkSize := runeCount / numChunks
	if chunkSize < 1 {
		chunkSize = runeCount
	}
	
	return chunkSize, numChunks
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


// WantCPUProfiling returns true if CPU profiling is enabled for transliteration
func WantCPUProfiling() bool {
	return os.Getenv("LANGKIT_PROFILE_TRANSLIT") == "1" || 
	       os.Getenv("LANGKIT_PROFILE_TRANSLIT") == "true" ||
	       os.Getenv("LANGKIT_PROFILE_TRANSLIT") == "yes"
}


func clean(s string) string{
	return reMultipleSpacesSeq.ReplaceAllString(strings.TrimSpace(s), " ")
}

func placeholder2345432() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
