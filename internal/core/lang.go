package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	iso "github.com/barbashov/iso639-3"
	"github.com/gookit/color"
	"github.com/k0kubun/pp"
	"github.com/rs/zerolog"
)

// TODO May make sense to move some functions to translitkit/pkg

const (
	// for now the only ranking requirement is to have Sub at the bottom
	Sub         = iota // Regular subtitles (lowest priority)
	StrippedSDH        // Stripped SDH
	Dub                // Dubtitles
	CC                 // Closed captions
)

const unknownLang = "und" // = undetermined, special code part of the ISO639 spec

var refmatch = map[string]int{
	"closedcaptions": CC,
	"dubtitles":      Dub,
	"subtitles":      Sub,
	"dialog":         Sub,
	"stripped_sdh":   StrippedSDH,
}

type Lang struct {
	*iso.Language `json:"language"`
	Subtag string `json:"subtag"` // Typically a ISO 3166-1 region but can also be a ISO 15924 script
}

func (l *Lang) String() string {
	return Str(l.Language)
}

func Str(l *iso.Language) string {
	if l == nil {
		return unknownLang
	}
	switch {
	case l.Part1 != "":
		return l.Part1
	case l.Part3 != "":
		return l.Part3
	case l.Part2T != "":
		return l.Part2T
	case l.Part2B != "":
		return l.Part2B
	}
	return unknownLang
}


func TagsStr2TagsArr(tagsString string) []string {
	tags := strings.Split(tagsString, ",")
	for i := range tags {
		tags[i] = strings.TrimSpace(tags[i])
	}
	return tags
}


func (tsk *Task) PrepareLangs() *ProcessingError {
	if len(tsk.Langs) > 0 {
		tmp, err := ParseLanguageTags([]string{tsk.Langs[0]})
		if err != nil {
			return tsk.Handler.LogErr(err, AbortTask, "Language parsing error on index 0")
		}
		tsk.Targ = tmp[0]
	}
	if len(tsk.Langs) > 1 {
		tmp, err := ParseLanguageTags(tsk.Langs[1:])
		if err != nil {
			return tsk.Handler.LogErr(err, AbortTask, "Language parsing error")
		}
		tsk.RefLangs = tmp
	}
	return nil
}

// Exemple of input slice: []string{"pt-BR", "yue", "zh-Hant"}
func ParseLanguageTags(arr []string) (langs []Lang, err error) {
	if len(arr) == 0 {
		return langs, fmt.Errorf("empty slice passed to ParseLanguageTags")
	}

	for _, tmp := range arr {
		var lang Lang
		parts := strings.Split(tmp, "-")
		// Convert to lowercase for case-insensitive matching
		langCode := strings.ToLower(parts[0])
		lang.Language = iso.FromAnyCode(langCode)
		if lang.Language == nil {
			// Only try name matching for strings longer than 3 chars (ISO codes are 2-3 chars)
			// This prevents false positives like "e" matching the "E" language
			if len(parts[0]) > 3 {
				// Try to match by language name as fallback
				lang.Language = iso.FromName(parts[0])
				if lang.Language == nil {
					// Try with title case for each word (handles "english" -> "English", "ancient greek" -> "Ancient Greek")
					titleCased := strings.Title(strings.ToLower(parts[0]))
					lang.Language = iso.FromName(titleCased)
				}
			}
			if lang.Language == nil {
				// because everybody confuses the domain name .jp with the ISO language code
				if langCode == "jp" {
					return nil, fmt.Errorf("'%s' is not a valid ISO-639 code, "+
						"for Japanese the code to use is either 'ja' or 'jpn'", parts[0])
				}
				return nil, fmt.Errorf("an invalid language code or name was passed: '%s'", parts[0])
			}
		}
		if len(parts) > 1 {
			lang.Subtag = strings.ToLower(parts[1])
		}
		langs = append(langs, lang)
	}
	return
}

func (tsk *Task) SetPreferred(langs []Lang, l, atm Lang, filename string, out *string, Native *Lang) bool {
	logger := tsk.Handler.ZeroLog()
	for idx, lang := range langs {
		if lang.Language == nil {
			err := fmt.Errorf("lang at index %d is nil", idx)
			msg := "BUG: SetPreferred received nil pointer among []Lang."
			tsk.Handler.LogErrWithLevel(Error, err, AbortAllTasks, msg)
			time.Sleep(1 * time.Second) // make sure throttler has time to flush
			logger.Fatal().Err(err).Interface("languages", langs).Msg(msg)
		}
	}
	
	isPreferredLang := setPreferredLang(langs, l, atm, logger)
	isPreferredSubtype := isPreferredSubtypeOver(*out, filename, logger)
	isPreferred := isPreferredLang && isPreferredSubtype
	
	logger.Trace().
		Str("File", filename).
		Str("lang_currently_selected", atm.Part3).
		Bool("isPreferredLang", isPreferredLang).
		Bool("isPreferredSubtype", isPreferredSubtype).
		Msgf("candidate subs's lang '%s' should be preferred? %t", l.Part3, isPreferred)
		
	if isPreferred {
		*out = filename
		*Native = atm
		return true
	}
	return false
}

// Only 1st subtag found is considered, the others are ignored
func GuessLangFromFilename(name string) (lang Lang, err error) {
	// this int was in the original algo, not sure if I need it or not at some point
	var fn_start int
	l := guessLangFromFilename(name, &fn_start)
	if arr := strings.Split(l, "-"); strings.Contains(l, "-") {
		lang.Subtag = strings.ToLower(arr[1])
		l = arr[0]
	}
	lang.Language = iso.FromAnyCode(l)
	if lang.Language == nil {
		err = fmt.Errorf("No language could be identified: lang='%s'", l)
	}
	return
}

/*
This is a slightly improved version of mpv's algorithm to parse langs from filename available here:
https://github.com/mpv-player/mpv/blob/a6c363e8dac1e0fd932b1a17838e0da3b13f3b4c/misc/language.c#L300

While none of their code was directly reused, I'd rather give attribution to the authors nonetheless.
The C code they provide is under GNU Lesser General Public License LGPL version 2 or later:
https://github.com/mpv-player/mpv/blob/master/LICENSE.LGPL

The implementation here is provided under GPL3 as the rest of this project.
You can test if your subs are found by their algo with "mpv --sub-auto=fuzzy --msg-level=find_files=trace video.mp4"
*/
func guessLangFromFilename(name string, langStart *int) string {
	stripname := stripCommonSubsMention(filepath.Base(name))
	var ok bool
	var i, langLength int
	// this iter decorticates some more in case lang isn't located at the end of the name
	for x := 0; x < 3; x++ {
		//fmt.Printf("stripname_%d=\"%s\"\n", x, stripname)
		// Trim ext during 1st loop and then in further loops, attempt to
		// decorticate any potential dot-separated irrelevant info such as:
		// movie version (director's cut...), video quality, rip method used or whatnot
		stripname = strings.TrimSuffix(stripname, filepath.Ext(stripname))
		stripname = strings.TrimSpace(stripname)

		if len(stripname) < 2 {
			return ""
		}

		langLength = 0
		i = len(stripname) - 1
		suffixesLength := 0

		delimiter := '.'
		if stripname[i] == ')' {
			delimiter = '('
			i--
		}
		if stripname[i] == ']' {
			delimiter = '['
			i--
		}
		ok = true
		for {
			// Identify alphabetic characters for the language tag
			for i >= 0 && unicode.IsLetter(rune(stripname[i])) {
				langLength++
				i--
			}

			// Validate the length of subtags
			if langLength < suffixesLength+1 || langLength > suffixesLength+8 {
				ok = false // invalid length, return empty
			}

			// Check for subtag separator '-'
			if i >= 0 && stripname[i] == '-' {
				langLength++
				i--
				suffixesLength = langLength
			} else {
				//println(stripname[i+1 : i+1+langLength])
				break
			}
		}
		// Validate the primary subtag's length (2-3 letters)
		if langLength < suffixesLength+2 || langLength > suffixesLength+3 || i <= 0 || rune(stripname[i]) != delimiter {
			//color.Yellowln("langLength=", langLength, stripname[i+1 : i+1+langLength])
			//color.Yellowln("rune(stripname[i])=", string(stripname[i]))
			//color.Redln(langLength < suffixesLength+2, langLength > suffixesLength+3, i <= 0, rune(name[i]) != delimiter)
			ok = false
		}
		// if longer than 3 letters, must have hyphen and subtag
		if langLength > suffixesLength+3 && !strings.Contains(stripname[i+1:i+1+langLength], "-") {
			//color.Yellowln(string(stripname[i+1 : i+1+langLength]), "contains no '-'!")
			ok = false
		}
		if ok {
			break
		}
	}
	if !ok {
		return ""
	}

	// Set the starting position of the language tag
	if langStart != nil {
		*langStart = i
	}

	// Return the detected language tag as a substring
	return stripname[i+1 : i+1+langLength]
}


func stripCommonSubsMention(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "closedcaptions", "")
	s = strings.ReplaceAll(s, "subtitles", "")
	s = strings.ReplaceAll(s, "subtitle", "")
	s = strings.ReplaceAll(s, "dubtitles", "")
	s = strings.ReplaceAll(s, "dialog", "")
	s = strings.ReplaceAll(s, "stripped_sdh.subtitles", "")
	s = strings.ReplaceAll(s, "dubtitles.subtitles", "")
	//s = strings.ReplaceAll(s, "", "")
	return s
}


func setPreferredLang(langs []Lang, l, atm Lang, logger *zerolog.Logger) (b bool) {
	langIdx, langIsDesired := getIdx(langs, l)
	atmIdx, _ := getIdx(langs, atm)
	
	logger.Trace().
		Bool("lang_is_desired", langIsDesired).
		Bool("is_preferred_over_current", langIdx <= atmIdx).
		Msgf("evaluating candidate '%s' against current '%s'", l.Part3, atm.Part3)
	
	// idx = idx in the row of lang sorted by preference the user has passed
	if langIsDesired && langIdx <= atmIdx {
		b = true
	}
	return
}

func getIdx(langs []Lang, candidate Lang) (int, bool) {
	for i, l := range langs {
		// support redundant composition implicitly i.e. de-DE or th-TH
		var isRedundantSubtag bool
		if match := iso.FromAnyCode(candidate.Subtag); match != nil {
			isRedundantSubtag = *match == *l.Language
		}
		// pp.Println(candidate)
		// color.Blueln("candidateSubtag", candidate.Subtag)
		// color.Blueln("sameLangAsAsked?", l.Part3 == candidate.Part3, "subtagCheckOK?", (l.Subtag == candidate.Subtag || candidate.Subtag == l.Part3), "redundant?", isRedundantSubtag)
		// color.Yellowln("passing?", l.Part3 == candidate.Part3 && (l.Subtag == candidate.Subtag || isRedundantSubtag))
		if l.Part3 == candidate.Part3 && (l.Subtag == candidate.Subtag || l.Subtag == "" && isRedundantSubtag) {
			return i, true
		}
	}
	return 0, false
}

// compare subtitles filenames
func isPreferredSubtypeOver(curr, candidate string, logger *zerolog.Logger) bool {
	currVal := subtypeMatcher(curr)
	candidateVal := subtypeMatcher(candidate)
	
	eval := candidateVal >= currVal
	
	logger.Trace().
		Str("currently_select_filename", curr).
		Int("currently_select_subtype", currVal).
		Str("candidate_filename", candidate).
		Int("candidate_subtype", candidateVal).
		Msgf("candidate subs's subtype is preferred? %t", eval)
	
	return eval
}

func subtypeMatcher(s string) int {
	if s == "" {
		return 0
	}
	
	for subtype, v := range refmatch {
		if strings.Contains(strings.ToLower(s), subtype) {
			return v
		}
	}
	// If nothing is specified in the subtitle file name then it's probably a regular Sub file
	return Sub
}


func placeholder34564() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
