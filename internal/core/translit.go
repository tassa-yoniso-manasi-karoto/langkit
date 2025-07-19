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
	
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	_ "github.com/tassa-yoniso-manasi-karoto/translitkit"
	common "github.com/tassa-yoniso-manasi-karoto/translitkit/common"
	"github.com/tassa-yoniso-manasi-karoto/go-ichiran"
	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)


// FIXME transcoding srt into ass causes astisub runtime panic, no sure if supported or not

var (
	Splitter = common.DefaultSplitter // All providers must accept and return UTF-8.
	reSpacingInARow = regexp.MustCompile(`\s*(.*)\s*`)
)

var (
	reMultipleSpacesSeq = regexp.MustCompile(`\s+`)
)


type TranslitType int

const (
	Tokenize  TranslitType = iota
	Romanize
	Selective
	TokenizedSelective
)

type contextKey string

const (
	kanjiThresholdKey     contextKey = "kanjiThreshold"
	dockerRecreateKey     contextKey = "dockerRecreate"
	userProvidedBrowserKey contextKey = "userProvidedBrowser"
)

func (m TranslitType) String() string {
	return []string{"tokenized", "romanized", "selective", "selective_tokenized"}[m]
}

func (m TranslitType) ToSuffix() string {
	return "_" + m.String() + ".srt"
}


type StringResult struct {
	Tokenized         string // Complete tokenized text
	Romanized         string // Complete romanized text
	Selective         string // Complete selective transliteration
	TokenizedSelective string // Complete tokenized selective transliteration
}

type TranslitOutputType struct {
	ttype      TranslitType
	text       string
	path       string
	outputType MediaOutputType
	priority   int
	feature    string
}


// TranslitProvider defines an interface for transliteration providers
type TranslitProvider interface {
	Initialize(ctx context.Context, handler MessageHandler) error
	ProcessText(ctx context.Context, text string, handler MessageHandler) (StringResult, error)
	Close(ctx context.Context, langCode, RomanizationStyle string) error
	ProviderName() string
}


// Previously, a complex word-by-word replacement system was used to handle how NLP providers
// often trimmed non-lexical elements like punctuation, which deformed the original text format.
// Now translitkit handles this already so we've simplified by obtaining complete processed
// strings directly and applying them to the subtitle structure.

// this convoluted replacement system of processed word on the original subtitle line was
// designed to workaround the fact that some NLP providers trim non-lexical elements such as
// punctuation and therefore deformed the original string's format but after recent updates on
// translitkit it is not needed anymore. However for japanese go-ichiran is used directly
// because I didn't bother to reimplement selective transliteration through translitkit.
// TODO: Implement selective transliteration through translitkit

func (tsk *Task) Transliterate(ctx context.Context) *ProcessingError {
	langCode := tsk.Targ.Language.Part3
	
	reporter := crash.Reporter
	reporter.SaveSnapshot("Starting transliteration", tsk.DebugVals()) // necessity: high
	reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
		es.TransliterationType = pp.Sprint(tsk.TranslitTypes)
		es.TransliterationLanguage = langCode
	}) // necessity: high
	
	// Check if we're in test mode with mock providers - create mock files directly
	if os.Getenv("LANGKIT_USE_MOCK_PROVIDERS") == "true" {
		tsk.Handler.ZeroLog().Info().Msg("Using mock providers for transliteration in test mode")
		mockFiles, err := CreateMockTransliterationFiles(tsk.TargSubFile, tsk.TranslitTypes)
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
	
	// Record overall timing
	startTime := time.Now()
	
	base := strings.TrimSuffix(tsk.TargSubFile, ".srt")

	subsFilepathTokenized := base + Tokenize.ToSuffix()
	subsFilepathTranslit  := base + Romanize.ToSuffix()
	subsFilepathSelective := base + Selective.ToSuffix()
	subsFilepathTokenizedSelective := base + TokenizedSelective.ToSuffix()
	
	outputTypes := []TranslitOutputType{
		TranslitOutputType{Tokenize, "", subsFilepathTokenized, OutputTokenized, 70, "tokenization"},
		TranslitOutputType{Romanize, "", subsFilepathTranslit, OutputRomanized, 80, "romanization"},
		TranslitOutputType{Selective, "", subsFilepathSelective, OutputTranslit, 75, "selective_transliteration"},
		TranslitOutputType{TokenizedSelective, "", subsFilepathTokenizedSelective, OutputTranslit, 76, "tokenized_selective_transliteration"},
	}
	
	// Check if all requested transliteration types already exist
	allRequiredOutputsExist := true
	missingOutputs := []string{}
	existingOutputs := []string{}
	
	// Check each requested transliteration type
	for _, output := range outputTypes {
		if !slices.Contains(tsk.TranslitTypes, output.ttype) {
			// Skip if this type is not requested
			continue
		}
		
		// Skip selective transliteration checks if not applicable
		if (output.ttype == Selective || output.ttype == TokenizedSelective) && 
		   (langCode != "jpn" || tsk.KanjiThreshold <= -1) {
			continue
		}
		
		// Skip tokenized selective if not requested
		if output.ttype == TokenizedSelective && !tsk.TokenizeSelectiveTranslit {
			continue
		}
		
		// Skip selective if tokenized selective is requested instead
		if output.ttype == Selective && tsk.TokenizeSelectiveTranslit {
			continue
		}
		
		// Check if this output file exists and is not empty
		exists, err := fileExistsAndNotEmpty(output.path)
		if err != nil {
			return tsk.Handler.LogErr(err, AbortAllTasks,
				fmt.Sprintf("translit: error checking destination file %s", output.path))
		}
		
		if exists {
			existingOutputs = append(existingOutputs, output.ttype.String())
		} else {
			missingOutputs = append(missingOutputs, output.ttype.String())
			allRequiredOutputsExist = false
		}
	}
	
	// If all requested output types exist, we can skip processing
	if allRequiredOutputsExist && len(existingOutputs) > 0 {
		tsk.Handler.ZeroLog().Info().
			Strs("existing_outputs", existingOutputs).
			Msg("All requested transliteration outputs already exist, continuing...")
			
		// Register all existing output files for merging
		if tsk.MergeOutputFiles {
			for _, output := range outputTypes {
				if slices.Contains(tsk.TranslitTypes, output.ttype) {
					// We already checked that these files exist above
					tsk.RegisterOutputFile(output.path, output.outputType, tsk.Targ, output.feature, output.priority)
				}
			}
		}
		return nil
	}
	
	// Some or all outputs are missing, log which ones
	if len(existingOutputs) > 0 {
		tsk.Handler.ZeroLog().Info().
			Strs("existing_outputs", existingOutputs).
			Strs("missing_outputs", missingOutputs).
			Msg("Some transliteration outputs exist but others need to be generated")
	}

	// Get complete subtitle text
	subtitleText := GetCompleteSubtitleText(tsk.TargSubs)
	tsk.Handler.ZeroLog().Trace().Msgf("translit: subtitleText: len=%d", len(subtitleText))
	
	// Use the provider manager to get a provider and process the text
	// This handles initialization, processing, and releasing the provider
	processStartTime := time.Now()
	
	// Ensure provider manager is initialized
	if DefaultProviderManager == nil {
		InitTranslitService(tsk.Handler.ZeroLog().With().Logger())
	}
	
	// Record the provider name for crash reporting
	// Use a temporary provider to get the name without initializing
	tmpProvider, err := GetTranslitProvider(langCode, tsk.RomanizationStyle)
	if err == nil {
		reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
			es.CurrentTranslitProvider = tmpProvider.ProviderName()
		}) // necessity: high
	}
	
	tsk.Handler.ZeroLog().Warn().Msgf("translit: using managed provider for language %s, please wait...", langCode)
	
	ctxWithValues := context.WithValue(ctx, kanjiThresholdKey, tsk.KanjiThreshold)
	ctxWithValues = context.WithValue(ctxWithValues, dockerRecreateKey, tsk.DockerRecreate)
	ctxWithValues = context.WithValue(ctxWithValues, userProvidedBrowserKey, tsk.BrowserAccessURL != "")
	
	// Set user-provided browser URL if available
	if tsk.BrowserAccessURL != "" {
		tsk.Handler.ZeroLog().Info().
			Msgf("Setting user-provided browser access URL: %s", tsk.BrowserAccessURL)
		common.BrowserAccessURL = tsk.BrowserAccessURL
	}
	
	result, err := ProcessWithManagedProvider(ctxWithValues, langCode, tsk.RomanizationStyle, subtitleText, tsk.Handler, DefaultProviderManager)
	
	processDuration := time.Since(processStartTime)
	
	if err != nil {
		reporter.SaveSnapshot("Text processing failed", tsk.DebugVals()) // necessity: high
		if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: process: operation canceled by user")
		} else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: process: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks, "couldn't process text with managed provider")
	}
	
	// Assign result to output types
	outputTypes[0].text = result.Tokenized
	outputTypes[1].text = result.Romanized
	outputTypes[2].text = result.Selective
	outputTypes[3].text = result.TokenizedSelective
	
	tsk.Handler.ZeroLog().Info().
		Dur("process_duration", processDuration).
		Msgf("translit: finished processing text with managed provider")
	
	// Write output files - measure performance
	writeStartTime := time.Now()
	
	for _, output := range outputTypes {
		// Skip if not requested
		if !slices.Contains(tsk.TranslitTypes, output.ttype) {
			continue
		}
		
		if output.ttype == Selective || output.ttype == TokenizedSelective {
			if langCode != "jpn" || tsk.KanjiThreshold <= -1 {
				continue
			}
			
			if output.ttype == TokenizedSelective && !tsk.TokenizeSelectiveTranslit ||
				output.ttype == Selective && tsk.TokenizeSelectiveTranslit {
				continue
			}
		}
		
		// Skip if text is empty
		if output.text == "" {
			continue
		}
		
		// Create and write subtitle
		newSubs := CreateSubtitlesFromText(tsk.TargSubs, output.text)
		if err := newSubs.Write(output.path); err != nil {
			tsk.Handler.ZeroLog().Error().
				Err(err).
				Msgf("Failed to write %s subtitles", output.ttype.String())
		} else {
			tsk.Handler.ZeroLog().Info().
				Msgf("Created %s subtitles", output.ttype.String())
			
			if tsk.MergeOutputFiles {
				tsk.RegisterOutputFile(output.path, output.outputType, tsk.Targ, output.feature, output.priority)
			}
		}
	}

	writeDuration := time.Since(writeStartTime)
	
	// Log total performance statistics
	totalDuration := time.Since(startTime)
	tsk.Handler.ZeroLog().Debug().
		Str("language", langCode).
		Dur("total_duration", totalDuration).
		Dur("process_duration", processDuration).
		Dur("write_duration", writeDuration).
		Msg("Transliteration performance metrics")
	
	tsk.Handler.ZeroLog().Debug().Msg("Foreign subs were transliterated")
	return nil
}

// GetCompleteSubtitleText extracts all text from subtitles into a single string with splitters
func GetCompleteSubtitleText(subs *subs.Subtitles) string {
	var result strings.Builder
	
	for _, item := range (*subs).Subtitles.Items {
		for _, line := range item.Lines {
			for _, lineItem := range line.Items {
				result.WriteString(lineItem.Text)
				result.WriteString(Splitter)
			}
		}
	}
	
	return result.String()
}

// CreateSubtitlesFromText creates a new subtitle file from processed text
func CreateSubtitlesFromText(originalSubs *subs.Subtitles, processedText string) *subs.Subtitles {
	// Create a deep copy of the original subtitles
	newSubs := subs.DeepCopy(originalSubs)
	
	// Split the processed text into parts by splitter
	parts := strings.Split(processedText, Splitter)
	partIndex := 0
	
	// Apply processed parts to the subtitle structure
	for i := range (*newSubs).Items {
		for j := range (*newSubs).Items[i].Lines {
			for k := range (*newSubs).Items[i].Lines[j].Items {
				if partIndex < len(parts) {
					(*newSubs).Items[i].Lines[j].Items[k].Text = clean(parts[partIndex])
					partIndex++
				}
			}
		}
	}
	
	return newSubs
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

func (p *GenericProvider) Initialize(ctx context.Context, handler MessageHandler) error {
	recreate := false
	if val, ok := ctx.Value(dockerRecreateKey).(bool); ok {
		recreate = val
	}

	if !recreate {
		return p.module.InitWithContext(ctx)
	}
	return p.module.InitRecreateWithContext(ctx, true)
}

func (p *GenericProvider) ProcessText(ctx context.Context, text string, handler MessageHandler) (StringResult, error) {
	// Generate a unique task ID for this operation
	taskID := fmt.Sprintf("transliteration-%d", time.Now().UnixNano())
	m := p.module
	nativelyUsesChunks := m.SupportsProgress()

	handler.ZeroLog().Debug().
		Bool("nativelyUsesChunks", nativelyUsesChunks).
		Msg("")
	
	handler.IncrementProgress(
		taskID,
		0,
		1,
		30,
		"Starting transliteration...",
		"",
		"h-2",
	)
	
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
	
	m.WithProgressCallback(func(idx, length int) {
		handler.IncrementProgress(
			taskID,
			1,
			length,
			30,
			"Transliterating",
			fmt.Sprintf("Processing text (%d/%d)", idx+1, length),
			"h-2",
		)
	})
	
	tokens, err := m.TokensWithContext(ctx, text)
	if err != nil {
		return StringResult{}, fmt.Errorf("error processing text: %w", err)
	}
	
	// Get all formats from the tokens
	tokenized := tokens.Tokenized()
	romanized := tokens.Roman()
	
	romanized = p.module.RomanPostProcess(romanized, func(s string) string { return s })
	
	return StringResult{
		Tokenized: tokenized,
		Romanized: romanized,
	}, nil
}

func (p *GenericProvider) ProviderName() string {
	return fmt.Sprintf("%s-%s", p.module.Lang, p.module.ProviderNames())
}

func (p *GenericProvider) Close(ctx context.Context, languageCode, RomanizationStyle string) error {
	userProvidedBrowser := false
	if val, ok := ctx.Value(userProvidedBrowserKey).(bool); ok {
		userProvidedBrowser = val
	}
	
	schemes, err := common.GetSchemes(languageCode)
	if err != nil {
		if err == common.ErrNoSchemesRegistered {
			return fmt.Errorf("translit: close: no schemes for %s: %w", languageCode, err)
		} else {
			return fmt.Errorf("translit: close couldn't schemes for %s: %w", languageCode, err)
		}
	}
	
	needsScraper := false
	for _, scheme := range schemes {
		if RomanizationStyle == scheme.Name && scheme.NeedsScraper {
			needsScraper = true
			break
		}
	}
	
	// User-started instances should not be closed by Langkit
	if needsScraper && userProvidedBrowser {
		return nil
	}
	
	return p.module.CloseWithContext(ctx)
}













// JapaneseProvider handles Japanese-specific transliteration
type JapaneseProvider struct {
	tokensSlice    []*ichiran.JSONTokens
	kanjiThreshold int
}

func NewJapaneseProvider() *JapaneseProvider {
	return &JapaneseProvider{}
}

func (p *JapaneseProvider) Initialize(ctx context.Context, handler MessageHandler) error {
	p.kanjiThreshold = -1
	if val := ctx.Value(kanjiThresholdKey); val != nil {
		if threshold, ok := val.(int); ok {
			p.kanjiThreshold = threshold
		}
	}
	recreate := false
	if val, ok := ctx.Value(dockerRecreateKey).(bool); ok {
		recreate = val
	}

	// Create ichiran manager with progress handler
	options := []ichiran.ManagerOption{}
	if handler != nil {
		// Flag to track if this is the first progress callback
		isFirstCallback := true
		// Track last progress to calculate increments
		var lastProgress float64 = 0.0  // Start from 0%
		// Count checkpoints for progress calculation
		var checkpointCount int
		// Track if we're in initialization phase
		var initializationInProgress bool = false
		
		// Weighted progress points for each checkpoint based on actual durations
		checkpointProgress := []float64{
			10.0,  // Checkpoint 1: 5% → 10% (quick ~25s)
			15.0,  // Checkpoint 2: 10% → 15% (quick ~25s)
			25.0,  // Checkpoint 3: 15% → 25% (medium ~70s)
			35.0,  // Checkpoint 4: 25% → 35% (medium ~70s)
			85.0,  // Checkpoint 5: 35% → 85% (very long ~260s!)
			90.0,  // Checkpoint 6: 85% → 90% (trivial)
		}
		
		// Create a progress handler that uses the task's message handler
		progressHandler := func(progress float64, description string, logMessage string) {
			// Detect initialization start
			if progress == 0 {  // "Starting ichiran DB init!"
				initializationInProgress = true
				if isFirstCallback {
					isFirstCallback = false
					handler.ZeroLog().Info().
						Msg("First-time Ichiran initialization detected. "+
						"Langkit will download a compressed 200MB database dump and " +
						"use it to restore a local database that Ichiran depends on " +
						"(4.4GB on disk) (~10-20 minutes).")
				}
			}
			
			// If we're not in initialization, ignore checkpoint milestones
			if !initializationInProgress && progress == -1 {
				return  // Don't create progress bars for post-init checkpoints
			}
			
			// Handle dynamic checkpoint progress
			if progress == -1 {
				if checkpointCount < len(checkpointProgress) {
					progress = checkpointProgress[checkpointCount]
				} else {
					progress = 90.0  // Cap at 90% for any extra checkpoints
				}
				checkpointCount++
			}
			
			// Detect initialization complete
			if progress >= 95 {  // "All set, awaiting commands"
				initializationInProgress = false
			}
			
			// Calculate the INCREMENT from last progress
			increment := progress - lastProgress
			if increment < 0 {
				increment = 0  // Prevent negative increments
			}
			lastProgress = progress
			
			// Use a unique task ID for ichiran initialization
			taskID := "ichiran-init"
			// Pass the INCREMENT, not the absolute value
			handler.IncrementProgress(
				taskID,
				int(increment),  // increment since last update
				100,            // total
				25,             // priority (lower than main tasks)
				"Installing Ichiran",
				description,
				"h-3", // size hint
			)
		}
		options = append(options, ichiran.WithProgressHandler(dockerutil.ProgressHandler(progressHandler)))
	}

	// Create the manager with options
	mgr, err := ichiran.NewManager(ctx, options...)
	if err != nil {
		return fmt.Errorf("failed to create ichiran manager: %w", err)
	}
	
	// Use the manager's Init methods which will track progress
	var initErr error
	if !recreate {
		initErr = mgr.Init(ctx)
	} else {
		initErr = mgr.InitRecreate(ctx, true)
	}
	
	// Clean up progress bar whether we succeeded or failed
	if handler != nil {
		handler.RemoveProgressBar("ichiran-init")
	}
	
	return initErr
}

func (p *JapaneseProvider) ProcessText(ctx context.Context, text string, handler MessageHandler) (StringResult, error) {
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
		return StringResult{}, fmt.Errorf("error splitting text into chunks: %w", err)
	}
	totalChunks := len(chunks)
	
	handler.ZeroLog().Trace().
		Int("actualNumChunks", totalChunks).
		Msg("")
	
	handler.IncrementProgress(
		taskID,
		0,
		totalChunks,
		30,
		"Starting Japanese analysis...",
		"",
		"h-2",
	)
	
	var tokenizedResult, romanizedResult, selectiveResult, tokenizedSelectiveResult strings.Builder
	p.tokensSlice = make([]*ichiran.JSONTokens, 0, totalChunks)
	
	// Process each chunk
	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return StringResult{}, ctx.Err()
		default:
		}
		
		// Analyze each chunk only once
		tokens, err := ichiran.AnalyzeWithContext(ctx, chunk)
		if err != nil {
			return StringResult{}, fmt.Errorf("error analyzing chunk %d: %w", i+1, err)
		}
		
		// Cache tokens for potential future use
		p.tokensSlice = append(p.tokensSlice, tokens)
		
		// Extract all formats from the tokens
		tokenizedResult.WriteString(tokens.Tokenized())
		romanizedResult.WriteString(tokens.Roman())
		
		// Handle selective transliteration if enabled
		if p.kanjiThreshold > -1 {
			selective, err := tokens.SelectiveTranslit(p.kanjiThreshold)
			if err != nil {
				return StringResult{}, fmt.Errorf("error getting selective transliteration: %w", err)
			}
			selectiveResult.WriteString(selective)
			
			tokenizedSelective, err := tokens.SelectiveTranslitTokenized(p.kanjiThreshold)
			if err != nil {
				return StringResult{}, fmt.Errorf("error getting tokenized selective transliteration: %w", err)
			}
			tokenizedSelectiveResult.WriteString(tokenizedSelective)
		}
		
		// Update progress
		handler.IncrementProgress(
			taskID,
			1,
			totalChunks,
			30,
			"Analyzing Japanese",
			fmt.Sprintf("Processing chunk %d/%d", i+1, totalChunks),
			"h-2",
		)
	}
	
	return StringResult{
		Tokenized: tokenizedResult.String(),
		Romanized: romanizedResult.String(),
		Selective: selectiveResult.String(),
		TokenizedSelective: tokenizedSelectiveResult.String(),
	}, nil
}

func (p *JapaneseProvider) ProviderName() string {
	return "jpn-ichiran"
}

func (p *JapaneseProvider) Close(ctx context.Context, _, _ string) error {
	return ichiran.Close()
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


// fileExistsAndNotEmpty checks if a file exists and has content
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


func clean(s string) string{
	return reMultipleSpacesSeq.ReplaceAllString(strings.TrimSpace(s), " ")
}



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


func placeholder2345432() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓂𝓹𝓲𝓵𝓮𝓻")
}