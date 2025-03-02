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

func (tsk *Task) Translit(ctx context.Context, subsFilepath string) *ProcessingError {	
	common.BrowserAccessURL = tsk.BrowserAccessURL 
	// TODO rm hardcoded ext
	subsFilepathTokenized := strings.TrimSuffix(subsFilepath, ".srt") + "_tokenized.srt"
	subsFilepathTranslit := strings.TrimSuffix(subsFilepath, ".srt") + "_translit.srt"
	
	if alreadyDone, err := fileExistsAndNotEmpty(subsFilepathTranslit); err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: error checking destination file %s", subsFilepathTranslit))
	} else if alreadyDone {
		tsk.Handler.ZeroLog().Info().
			Bool("file_exists_and_not_empty", alreadyDone).
			Msg("Subtitle were already transliterated previously, continuing...")
		return nil
	}

	m, err := common.GetSchemeModule(tsk.Targ.Language.Part3, tsk.RomanizationStyle)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: couldn't get default provider for language %s-%s", tsk.Targ.Language.Part3, tsk.RomanizationStyle))
	}
	tsk.Handler.ZeroLog().Trace().Msg("translit: successfully retrived default module for lang: " + m.Lang)
	
	// TODO to derive or not to derive?
	m.WithContext(ctx)
	
	tsk.Handler.ZeroLog().Warn().Msgf("translit: %s-%s-%s provider initialization starting, please wait...", m.Lang, m.ProviderNames(), tsk.RomanizationStyle)
	if !tsk.DockerRecreate {
		err = m.Init()
	} else {
		err = m.InitRecreate(true)
	}
	if err != nil {
	        if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: init: operation canceled by user")
	        } else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: init: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: failed to init default provider for language %s", tsk.Targ.Language.Part3))
	}
	tsk.Handler.ZeroLog().Info().Msgf("translit: %s-%s-%s successfully initialized", m.Lang, m.ProviderNames(), tsk.RomanizationStyle)
	
	SubTranslit, _ := astisub.OpenFile(subsFilepath)
	SubTokenized, _ := astisub.OpenFile(subsFilepath)
	mergedSubsStr, _ := Subs2StringBlock(SubTranslit)
	
	// module returns array of all tokenized and all tokenized+transliteration that
	// we can replace directly of mergedSubsStr, before splitting mergedSubsStr to recover subtitles
	tokens, err := m.Tokens(mergedSubsStr)
	if err != nil {
	        if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: tkns: operation canceled by user")
	        } else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: tkns: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("couldn't get tokens from default provider for language %s", tsk.Targ.Language.Part3))
			//Str("module-lang", m.Lang).
			//Str("module-provider", m.ProviderNames()).
	}
	tsk.Handler.ZeroLog().Trace().Msgf("translit: %s-%s returned tokens", m.Lang, m.ProviderNames())
	tokenizeds := tokens.TokenizedParts()
	translits  := tokens.RomanParts()
	tsk.Handler.ZeroLog().Trace().Msg("Tokenization/transliteration query finished")
	
	// TODO this convoluted replacement system of processed word on the original subtitle line
	//  was designed to workaround the fact that some providers trimmed non-lexical elements such as
	// punctuation and therefore deformed the original string's format but after recent updates on
	// translitkit I am not sure whether it's still needed
	mergedSubsStrTranslit := mergedSubsStr
	for i, tokenized := range tokenizeds {
		translit := translits[i]
		//color.Redln("Replacing: ", tokenized, " –> ", translit, "\tisFound? ", strings.Contains(mergedSubsStrTranslit, tokenized))
		mergedSubsStrTranslit = strings.Replace(mergedSubsStrTranslit, tokenized, translit+" ", 1)
	}
	
	// Replace from translit this time because if we replace thai by thai-tokenize we will endup replacing FALSE POSITIVES!
	mergedSubsStrTokenized := mergedSubsStrTranslit
	for i, tokenized := range tokenizeds {
		translit := translits[i]
		//color.Redln("Replacing: ", translit, " –> ", tokenized, "\tisFound? ", strings.Contains(mergedSubsStrTokenized, translit))
		mergedSubsStrTokenized = strings.Replace(mergedSubsStrTokenized, translit, tokenized, 1)
	}
	
	idx := 0
	subSliceTranslit  := strings.Split(mergedSubsStrTranslit, Splitter)
	subSliceTokenized := strings.Split(mergedSubsStrTokenized, Splitter)
	tsk.Handler.ZeroLog().Trace().
		Int("len(subSliceTranslit)", len(subSliceTranslit)).
		Int("len(subSliceTokenized)", len(subSliceTokenized)).
		Msg("")
	for i, _ := range (*SubTranslit).Items {
		for j, _ := range (*SubTranslit).Items[i].Lines {
			for k, _ := range (*SubTranslit).Items[i].Lines[j].Items {
				// FIXME: Trimmed closed captions have some sublines removed, hence must adjust idx
				target := subSliceTranslit[idx]
				(*SubTokenized).Items[i].Lines[j].Items[k].Text = clean(subSliceTokenized[idx])
				(*SubTranslit).Items[i].Lines[j].Items[k].Text = m.RomanPostProcess(target, func(s string) string { return s })
				idx += 1
			}
		}
	}
	tsk.Handler.ZeroLog().Trace().
		Int("len(SubTokenized.Items)", len(SubTokenized.Items)).
		Int("len(SubTranslit.Items)", len(SubTranslit.Items)).
		Msg("")
	
	if err := SubTokenized.Write(subsFilepathTokenized); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write tokenized subtitles")
	}
	if err := SubTranslit.Write(subsFilepathTranslit); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write transliterated subtitles")
	}
	tsk.Handler.ZeroLog().Debug().Msg("Foreign subs were transliterated")
	return nil
}


// pretty much the same code as above as I haven't found a simple way to add 
//  a custom routing for a specific language inside the general purpose func.
func (tsk *Task) TranslitJPN(ctx context.Context, subsFilepath string) *ProcessingError {	
	common.BrowserAccessURL = tsk.BrowserAccessURL
	subsFilepathTokenized := strings.TrimSuffix(subsFilepath, ".srt") + "_tokenized.srt"
	subsFilepathTranslit := strings.TrimSuffix(subsFilepath, ".srt") + "_translit.srt"
	subsFilepathSelective := strings.TrimSuffix(subsFilepath, ".srt") + "_selective.srt"
	
	if alreadyDone, err := fileExistsAndNotEmpty(subsFilepathTranslit); err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: error checking destination file %s", subsFilepathTranslit))
	} else if alreadyDone {
		tsk.Handler.ZeroLog().Info().
			Bool("file_exists_and_not_empty", alreadyDone).
			Msg("Subtitle were already transliterated previously, continuing...")
		return nil
	}
	
	ichiran.Ctx = ctx
	
	tsk.Handler.ZeroLog().Warn().Msgf("translit: %s-%s provider initialization starting, please wait...", "jpn", "ichiran")
	var err error
	if !tsk.DockerRecreate {
		err = ichiran.Init()
	} else {
		err = ichiran.InitRecreate(true)
	}
	if err != nil {
	        if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: init: operation canceled by user")
	        } else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: init: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: failed to init default provider for language %s", tsk.Targ.Language.Part3))
	}
	tsk.Handler.ZeroLog().Info().Msgf("translit: %s-%s successfully initialized", "jpn", "ichiran")
	
	SubTranslit, _ := astisub.OpenFile(subsFilepath)
	SubTokenized, _ := astisub.OpenFile(subsFilepath)
	SubSelective, _ := astisub.OpenFile(subsFilepath)
	mergedSubsStr, _ := Subs2StringBlock(SubTranslit)
	
	// module returns array of all tokenized and all tokenized+transliteration that
	// we can replace directly of mergedSubsStr, before splitting mergedSubsStr to recover subtitles
	tokens, err := ichiran.Analyze(mergedSubsStr)
	if err != nil {
	        if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: tkns: operation canceled by user")
	        } else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: tkns: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks, "couldn't get tokens from ichiran")
	}
	tsk.Handler.ZeroLog().Trace().Msgf("translit: %s-%s returned tokens", "jpn", "ichiran")
	tokenizeds := tokens.TokenizedParts()
	translits  := tokens.RomanParts()
	mergedSubsStrSelective, err  := tokens.SelectiveTranslit(tsk.KanjiThreshold)
	if err != nil {
	        if errors.Is(err, context.Canceled) {
			return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "translit: selectiveTranslit: operation canceled by user")
	        } else if errors.Is(err, context.DeadlineExceeded) {
			return tsk.Handler.LogErr(err, AbortTask, "translit: selectiveTranslit: operation timed out.")
		}
		return tsk.Handler.LogErr(err, AbortAllTasks, "couldn't get selectiveTranslit from ichiran")
	}
	tsk.Handler.ZeroLog().Trace().Msg("Tokenization/transliteration query finished")
	
	// TODO this convoluted replacement system of processed word on the original subtitle line
	//  was designed to workaround the fact that some providers trimmed non-lexical elements such as
	// punctuation and therefore deformed the original string's format but after recent updates on
	// translitkit I am not sure whether it's still needed
	mergedSubsStrTranslit := mergedSubsStr
	for i, tokenized := range tokenizeds {
		translit := translits[i]
		//color.Redln("Replacing: ", tokenized, " –> ", translit, "\tisFound? ", strings.Contains(mergedSubsStrTranslit, tokenized))
		mergedSubsStrTranslit = strings.Replace(mergedSubsStrTranslit, tokenized, translit+" ", 1)
	}
	
	mergedSubsStrTokenized := mergedSubsStrTranslit
	for i, tokenized := range tokenizeds {
		translit := translits[i]
		//color.Redln("Replacing: ", translit, " –> ", tokenized, "\tisFound? ", strings.Contains(mergedSubsStrTokenized, translit))
		mergedSubsStrTokenized = strings.Replace(mergedSubsStrTokenized, translit, tokenized, 1)
	}
	
	idx := 0
	subSliceTranslit  := strings.Split(mergedSubsStrTranslit, Splitter)
	subSliceTokenized := strings.Split(mergedSubsStrTokenized, Splitter)
	subSliceSelective := strings.Split(mergedSubsStrSelective, Splitter)
	tsk.Handler.ZeroLog().Trace().
		Int("len(subSliceTranslit)", len(subSliceTranslit)).
		Int("len(subSliceTokenized)", len(subSliceTokenized)).
		Int("len(subSliceSelective)", len(subSliceSelective)).
		Msg("")
	for i, _ := range (*SubTranslit).Items {
		for j, _ := range (*SubTranslit).Items[i].Lines {
			for k, _ := range (*SubTranslit).Items[i].Lines[j].Items {
				// FIXME: Trimmed closed captions have some sublines removed, hence must adjust idx
				(*SubTokenized).Items[i].Lines[j].Items[k].Text = clean(subSliceTokenized[idx])
				(*SubTranslit).Items[i].Lines[j].Items[k].Text = subSliceTranslit[idx]
				if tsk.KanjiThreshold > -1 {
					(*SubSelective).Items[i].Lines[j].Items[k].Text = subSliceSelective[idx]
				}
				idx += 1
			}
		}
	}
	tsk.Handler.ZeroLog().Trace().
		Int("len(SubTokenized.Items)", len(SubTokenized.Items)).
		Int("len(SubTranslit.Items)", len(SubTranslit.Items)).
		Msg("")
	
	if err := SubTokenized.Write(subsFilepathTokenized); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write tokenized subtitles")
	}
	if err := SubTranslit.Write(subsFilepathTranslit); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write transliterated subtitles")
	}
	if tsk.KanjiThreshold > -1 {
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
