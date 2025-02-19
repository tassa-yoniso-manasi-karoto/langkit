package core

import (
	"fmt"
	"strings"
	"regexp"
	"context"
	
	"github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	//"github.com/schollz/progressbar/v3"
	"github.com/gookit/color"
	
	_ "github.com/tassa-yoniso-manasi-karoto/translitkit"
	translitkit "github.com/tassa-yoniso-manasi-karoto/translitkit/common"
)

var (
	Splitter = "𓃰" // All providers must accept and return UTF-8.
	reSpacingInARow = regexp.MustCompile(`\s*(.*)\s*`)
	reMultipleSpacesSeq = regexp.MustCompile(`\s+`)
)

func (tsk *Task) Translit(ctx context.Context, subsFilepath string) *ProcessingError {
	translitkit.BrowserAccessURL = tsk.BrowserAccessURL
	m, err := translitkit.GetSchemeModule(tsk.Targ.Language.Part3, tsk.RomanizationStyle)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: couldn't get default provider for language %s-%s", tsk.Targ.Language.Part3, tsk.RomanizationStyle))
	}
	tsk.Handler.ZeroLog().Trace().Msg("translit: successfully retrived default module for lang: " + m.Lang)
	
	m.WithContext(ctx)
	
	tsk.Handler.ZeroLog().Warn().Msgf("translit: %s-%s-%s provider initialization starting, please wait...", m.Lang, m.ProviderNames(), tsk.RomanizationStyle)
	if !tsk.DockerRecreate {
		err = m.Init()
	} else {
		err = m.InitRecreate(true)
	}
	if err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("translit: failed to init default provider for language %s", tsk.Targ.Language.Part3))
	}
	tsk.Handler.ZeroLog().Info().Msgf("translit: %s-%s-%s successfully initialized", m.Lang, m.ProviderNames(), tsk.RomanizationStyle)
	
	SubTranslit, _ := astisub.OpenFile(subsFilepath)
	SubTokenized, _ := astisub.OpenFile(subsFilepath)
	mergedSubsStr, subSlice := Subs2StringBlock(SubTranslit)
	fmt.Println("Len=", len(subSlice)) // FIXME
	
	// module returns array of all tokenized and all tokenized+transliteration that
	// we can replace directly of mergedSubsStr, before splitting mergedSubsStr to recover subtitles
	tokens, err := m.Tokens(mergedSubsStr)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks,
			fmt.Sprintf("couldn't get tokens from default provider for language %s", tsk.Targ.Language.Part3))
			//Str("module-lang", m.Lang).
			//Str("module-provider", m.ProviderNames()).
	}
	tsk.Handler.ZeroLog().Trace().Msgf("translit: %s-%s returned tokens", m.Lang, m.ProviderNames())
	tokenizeds := tokens.TokenizedParts()
	translits  := tokens.RomanParts()
	tsk.Handler.ZeroLog().Trace().Msg("Tokenization/transliteration query finished")
	
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
	if err := SubTokenized.Write(strings.TrimSuffix(subsFilepath, ".srt") + "_tokenized.srt"); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write tokenized subtitles")
	}
	if err := SubTranslit.Write(strings.TrimSuffix(subsFilepath, ".srt") + "_translit.srt"); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to write transliterated subtitles")
	}
	tsk.Handler.ZeroLog().Debug().Msg("Foreign subs were transliterated")
	return nil
}


// pretty much the same code as above as I haven't found a simple way to add 
//  a custom routing for a specific language inside the general purpose func.
// func (tsk *Task) TranslitJPN(subsFilepath string) *ProcessingError {

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






// func prepare(subSlice []string, max int) (QuerySliced []string) {
// 	var Query string
// 	for _, element := range subSlice {
// 		if max > 0 && utf8.RuneCountInString(Query+element) > max {
// 			QuerySliced = append(QuerySliced, Query)
// 			Query = ""
// 		}
// 		Query += element
// 	}
// 	return append(QuerySliced, Query)
// }



// // FIXME transcoding srt into ass causes astisub runtime panic, no sure if supported or not
// func WriteASS(filepath string, subtitles *astisub.Subtitles) error {
// 	// Create the output file
// 	outputFile, err := os.Create(filepath)
// 	if err != nil {
// 		return fmt.Errorf("failed to create file: %v", err)
// 	}
// 	defer outputFile.Close()

// 	// Write the subtitles to ASS format
// 	if err := subtitles.WriteToSSA(outputFile); err != nil {
// 		return fmt.Errorf("failed to write subtitles to ASS format: %v", err)
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
