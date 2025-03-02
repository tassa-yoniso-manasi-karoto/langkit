package core

import (
	"fmt"
	"strings"
	"regexp"
	"context"
	"os"
	"errors"
	
	"github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	//"github.com/schollz/progressbar/v3"
	"github.com/gookit/color"
	
	_ "github.com/tassa-yoniso-manasi-karoto/translitkit"
	common "github.com/tassa-yoniso-manasi-karoto/translitkit/common"
	"github.com/tassa-yoniso-manasi-karoto/go-ichiran"
)

var (
	Splitter = common.DefaultSplitter // All providers must accept and return UTF-8.
	reSpacingInARow = regexp.MustCompile(`\s*(.*)\s*`)
	reMultipleSpacesSeq = regexp.MustCompile(`\s+`)
)

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
	provider, err := GetTranslitProvider(tsk.Targ.Language.Part3, tsk.RomanizationStyle)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: couldn't get provider for language %s-%s", tsk.Targ.Language.Part3, tsk.RomanizationStyle))
	}
	
	// Initialize provider
	tsk.Handler.ZeroLog().Warn().Msgf("translit: %s provider initialization starting, please wait...", provider.ProviderName())
	if err := provider.Initialize(ctx, tsk); err != nil {
		if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: init: operation canceled by user")
		} else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: init: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: failed to init provider for language %s", tsk.Targ.Language.Part3))
	}
	tsk.Handler.ZeroLog().Info().Msgf("translit: %s successfully initialized", provider.ProviderName())
	
	// Open subtitle files
	SubTranslit, _ := astisub.OpenFile(subsFilepath)
	SubTokenized, _ := astisub.OpenFile(subsFilepath)
	mergedSubsStr, _ := Subs2StringBlock(SubTranslit)
	
	// Get tokens
	// returns array of all tokenized and all tokenized+transliteration that
	// we can replace directly of mergedSubsStr, before splitting mergedSubsStr
	// to recover subtitles without altering their format: punctuation etc
	tokenizeds, translits, err := provider.GetTokens(ctx, mergedSubsStr)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: tkns: operation canceled by user")
		} else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: tkns: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks, "couldn't get tokens from provider")
	}
	tsk.Handler.ZeroLog().Trace().Msgf("translit: %s returned tokens", provider.ProviderName())
	
	// Get selective transliteration if supported (for Japanese)
	var SubSelective *astisub.Subtitles
	var mergedSubsStrSelective string
	
	if tsk.Targ.Language.Part3 == "jpn" && tsk.KanjiThreshold > -1 {
		SubSelective, _ = astisub.OpenFile(subsFilepath)
		subsFilepathSelective := strings.TrimSuffix(subsFilepath, ".srt") + "_selective.srt"
		
		mergedSubsStrSelective, err = provider.GetSelectiveTranslit(ctx, mergedSubsStr, tsk.KanjiThreshold)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: selectiveTranslit: operation canceled by user")
			} else if errors.Is(err, context.DeadlineExceeded) {
				return tsk.Handler.LogErr(err, AbortTask, "translit: selectiveTranslit: operation timed out.")
			}
			return tsk.Handler.LogErr(err, AbortAllTasks, "couldn't get selective transliteration")
		}
	}
	
	tsk.Handler.ZeroLog().Trace().Msg("Tokenization/transliteration query finished")
	
	// TODO this convoluted replacement system of processed word on the original subtitle line
	//  was designed to workaround the fact that some providers trimmed non-lexical elements such as
	// punctuation and therefore deformed the original string's format but after recent updates on
	// translitkit I am not sure whether it's still needed
	
	// Common replacement logic
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
	
	// Split results
	idx := 0
	subSliceTranslit := strings.Split(mergedSubsStrTranslit, Splitter)
	subSliceTokenized := strings.Split(mergedSubsStrTokenized, Splitter)
	
	// Add selective slice for Japanese if available
	var subSliceSelective []string
	if tsk.Targ.Language.Part3 == "jpn" && tsk.KanjiThreshold > -1 && mergedSubsStrSelective != "" {
		subSliceSelective = strings.Split(mergedSubsStrSelective, Splitter)
		
		tsk.Handler.ZeroLog().Trace().
			Int("len(subSliceTranslit)", len(subSliceTranslit)).
			Int("len(subSliceTokenized)", len(subSliceTokenized)).
			Int("len(subSliceSelective)", len(subSliceSelective)).
			Msg("")
	} else {
		tsk.Handler.ZeroLog().Trace().
			Int("len(subSliceTranslit)", len(subSliceTranslit)).
			Int("len(subSliceTokenized)", len(subSliceTokenized)).
			Msg("")
	}
	
	// Apply changes to subtitles
	for i := range (*SubTranslit).Items {
		for j := range (*SubTranslit).Items[i].Lines {
			for k := range (*SubTranslit).Items[i].Lines[j].Items {
				// FIXME: Trimmed closed captions have some sublines removed, hence must adjust idx
				(*SubTokenized).Items[i].Lines[j].Items[k].Text = clean(subSliceTokenized[idx])
				
				// Process transliteration
				if tsk.Targ.Language.Part3 == "jpn" {
					(*SubTranslit).Items[i].Lines[j].Items[k].Text = subSliceTranslit[idx]
				} else {
					(*SubTranslit).Items[i].Lines[j].Items[k].Text = provider.PostProcess(subSliceTranslit[idx])
				}
				
				// Add selective transliteration for Japanese
				if tsk.Targ.Language.Part3 == "jpn" && tsk.KanjiThreshold > -1 && SubSelective != nil {
					(*SubSelective).Items[i].Lines[j].Items[k].Text = subSliceSelective[idx]
				}
				
				idx++
			}
		}
	}
	
	tsk.Handler.ZeroLog().Trace().
		Int("len(SubTokenized.Items)", len(SubTokenized.Items)).
		Int("len(SubTranslit.Items)", len(SubTranslit.Items)).
		Msg("")
	
	// Write output files
	if err := SubTokenized.Write(subsFilepathTokenized); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write tokenized subtitles")
	}
	if err := SubTranslit.Write(subsFilepathTranslit); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write transliterated subtitles")
	}
	
	// Write selective transliteration for Japanese if needed
	if tsk.Targ.Language.Part3 == "jpn" && tsk.KanjiThreshold > -1 && SubSelective != nil {
		subsFilepathSelective := strings.TrimSuffix(subsFilepath, ".srt") + "_selective.srt"
		if err := SubSelective.Write(subsFilepathSelective); err != nil {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write selectively transliterated subtitles")
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
