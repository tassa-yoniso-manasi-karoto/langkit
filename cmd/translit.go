
package cmd

import (
	"net/url"
	"fmt"
	"strings"
	"regexp"
	"os"
	"time"
	"unicode/utf8"
	
	"github.com/spf13/cobra"
	"github.com/go-rod/rod"
	"github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	"github.com/schollz/progressbar/v3"
	iso "github.com/barbashov/iso639-3"
	"github.com/gookit/color"
	
	"local.host/lib/ichiran"
)

var translitCmd = &cobra.Command{
	Use:   "translit <foreign-subs>",
	Short: "transliterate and tokenize a subtitle file",

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(1)),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			logger.Fatal().Msg("this command requires one argument: the path to the subtitle file to be processed")
		}
		tsk := DefaultTask(cmd)
		tsk.TargSubFile = args[0]
		
		tsk.WantTranslit = true
		tsk.TimeoutTranslit, _ = cmd.Flags().GetInt("translit-to")
		BrowserAccessURL, _ = cmd.Flags().GetString("browser-access-url")
		
		tsk.Mode = Translit
		tsk.Execute() // TODO check if any initialization is missing
	},
}

var (
	SupportedTranslitLangs = []TranslitModule{Thai, Japanese}
	
	Splitter = "𓃰" // All providers must accept and return UTF-8.
	reSpacingInARow = regexp.MustCompile(`\s*(.*)\s*`)
	reMultipleSpacesSeq = regexp.MustCompile(`\s+`)
)

type TranslitModule struct {
	Init		func() error
	Lang		*iso.Language
	Provider	string
	MaxLenQuery	int
	Query		func([]string) ([]string, []string, error)
	PostProcess	func(s string) string
}

func SupportedTranslitLangsRaw() (arr []string) {
	for _, l := range SupportedTranslitLangs {
		arr = append(arr, l.Lang.Part3)
	}
	return
}

var Thai = TranslitModule {
	Init: func() error {
		defer func() error {
			if x := recover(); x != nil {
				return fmt.Errorf("rod couldn't initialize and panicked")
			}
			return nil
		}()
		rod.New().ControlURL(BrowserAccessURL).MustConnect()	
		return nil
	},
	Lang: iso.FromAnyCode("th"),
	Provider: "thai2english.com",
	MaxLenQuery: 1000,
	Query: Thai2EnglishScraper,
	PostProcess: func(s string) string {
		return strings.ReplaceAll(clean(s), "pp", "bp")
	},
}

var Japanese = TranslitModule {
	Init: func() error {
		client, err := ichiran.NewClient(ichiran.DefaultConfig())
		if err != nil {
			return err
		}
		defer client.Close()

		_, err = client.Analyze("ありがとう")
		return err
	},
	Lang: iso.FromAnyCode("ja"),
	Provider: "Ichiran",
	MaxLenQuery: 5000, // TODO TBD
	Query: QueryIchiran,
	PostProcess: func(s string) string {
		return s
	},
}

func (tsk *Task) Translit(subsFilepath string) {
	var module TranslitModule
	for _, m := range SupportedTranslitLangs {
		if *m.Lang == *tsk.Targ.Language {
			module = m
			tsk.Log.Trace().Msg("Transliteration module found")
		}
	}
	if err := module.Init(); err != nil {
		tsk.Log.Fatal().Err(err).Str("module", pp.Sprint(module)).Msg("failed to initialize module")
	}
	tsk.Log.Trace().Msgf("Transliteration module %s-%s successfully initialized", module.Lang.Part3, module.Provider)
	SubTranslit, _ := astisub.OpenFile(subsFilepath)
	SubTokenized, _ := astisub.OpenFile(subsFilepath)
	mergedSubsStr, subSlice := Subs2StringBlock(SubTranslit)
	fmt.Println("Len=", len(subSlice))
	// CAVEAT: SlicedQuery doesn't correspond to our sentences but rather
	// to the bulk of sentences splitted in pieces of the maximum length accepted
	SlicedQuery := prepare(subSlice, module.MaxLenQuery)
	tsk.Log.Trace().Int("lenSlicedQuery", len(SlicedQuery)).Msg("Query to send to transliterator")
	// module.Query returns array of isolated words (=token) and isolated word transliteration that
	// we can replace directly of mergedSubsStr, before splitting mergedSubsStr to recover subtitles
	tokens, translits, err := module.Query(SlicedQuery)
	if err != nil {
		tsk.Log.Fatal().Err(err).Msg("Transliteration queries to provider failed")
	}
	tsk.Log.Trace().Msg("Transliteration queries finished")
	mergedSubsStrTranslit := mergedSubsStr
	for i, token := range tokens {
		translit := translits[i]
		/*if !strings.Contains(mergedSubsStr, token) {
			color.Redln("ignoring following token as not in japanese sub: ", token, " –> ", translit)
		}*/
		mergedSubsStrTranslit = strings.Replace(mergedSubsStrTranslit, token, translit+" ", 1)
	}
	// Replace from translit this time because if we replace thai by thai-tokenize we will endup replacing FALSE POSITIVES!
	mergedSubsStrTokenized := mergedSubsStrTranslit
	for i, token := range tokens {
		translit := translits[i]
		mergedSubsStrTokenized = strings.Replace(mergedSubsStrTokenized, translit, token, 1)
	}
	
	idx := 0
	subSliceTranslit := strings.Split(mergedSubsStrTranslit, Splitter)
	subSliceTokenized := strings.Split(mergedSubsStrTokenized, Splitter)
	for i, _ := range (*SubTranslit).Items {
		for j, _ := range (*SubTranslit).Items[i].Lines {
			for k, _ := range (*SubTranslit).Items[i].Lines[j].Items {
				// FIXME: Trimmed closed captions have some sublines removed, hence must adjust idx
				target := subSliceTranslit[idx]
				(*SubTokenized).Items[i].Lines[j].Items[k].Text = clean(subSliceTokenized[idx])
				(*SubTranslit).Items[i].Lines[j].Items[k].Text = module.PostProcess(target)
				idx += 1
			}
		}
	}
	SubTokenized.Write(strings.TrimSuffix(subsFilepath, ".srt") + "_tokenized.srt")
	SubTranslit.Write(strings.TrimSuffix(subsFilepath, ".srt") + "_translit.srt")
	tsk.Log.Debug().Msg("Foreign subs were transliterated")
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


func prepare(subSlice []string, max int) (QuerySliced []string) {
	var Query string
	for _, element := range subSlice {
		if max > 0 && utf8.RuneCountInString(Query+element) > max {
			QuerySliced = append(QuerySliced, Query)
			Query = ""
		}
		Query += element
	}
	return append(QuerySliced, Query)
}



func QueryIchiran(QuerySliced []string) (tokens, translits []string, err error) {
	config := ichiran.DefaultConfig()
	config.Timeout = 1 * time.Hour
	client, err := ichiran.NewClient(config)
	if err != nil {
		return []string{}, []string{}, err
	}
	defer client.Close()
	bar := progressbar.Default(int64(len(QuerySliced)))
	for _, Query := range QuerySliced {
		text, err := client.Analyze(Query)
		if err != nil {
			return []string{}, []string{}, err
		}
		tokens = append(tokens, text.TokenParts()...)
		translits = append(translits, text.RomanParts()...)
		bar.Add(1)
	}
	return
}

func Thai2EnglishScraper(QuerySliced []string) (ths, tlits []string, err error) {
	browser := rod.New().ControlURL(BrowserAccessURL).MustConnect()
	//defer browser.MustClose()
	bar := progressbar.Default(int64(len(QuerySliced)))
	for idx, Query := range QuerySliced {
		page := browser.MustPage(fmt.Sprintf("https://www.thai2english.com/?q=%s", url.QueryEscape(Query))).MustWaitLoad()
		page.MustWaitRequestIdle()
		// Wait for results to load
		page.MustElement(".word-breakdown_line-meanings__1RADe")
		elements := page.MustElements(".word-breakdown_line-meaning__NARMM")
		if len(elements) == 0 {
			return []string{}, []string{}, fmt.Errorf("elements are empty. idx=%d", idx)
		}
		for _, element := range elements {
			thNode, err       :=  element.Element(".thai")
			if err != nil {
				continue
			}
			th := thNode.MustText()
			
			tlitNode, err := element.Element(".tlit")
			if err != nil {
				color.Redln("no transliteration element exists")
				continue
			}
			tlit := tlitNode.MustText()
			ths = append(ths, th)
			tlits = append(tlits, tlit+" ")
		}
		page.MustClose()
		bar.Add(1)
	}
	return
}

func clean(s string) string{
	return reMultipleSpacesSeq.ReplaceAllString(strings.TrimSpace(s), " ")
}


func containsWithIdx(arr []string, i string) (bool, int) {
	for idx, a := range arr {
		if a == i {
			return true, idx
		}
	}
	return false, -1
}


// FIXME transcoding srt into ass causes astisub runtime panic, no sure if supported or not
func WriteASS(filepath string, subtitles *astisub.Subtitles) error {
	// Create the output file
	outputFile, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer outputFile.Close()

	// Write the subtitles to ASS format
	if err := subtitles.WriteToSSA(outputFile); err != nil {
		return fmt.Errorf("failed to write subtitles to ASS format: %v", err)
	}

	return nil
}


func placeholder2345432() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
