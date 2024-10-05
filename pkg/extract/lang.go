package extract

import (
	"path"
	"strings"
	"unicode"

	"github.com/gookit/color"
	"github.com/k0kubun/pp"
)

type Lang struct {
	Str, Region string
}

const (
	StrippedSDH = iota
	Sub
	Dub
	CC
)

var refmatch = map[string]int{
	"closedcaptions": CC,
	"dubtitles":      Dub,
	"subtitles":      Sub,
	"dialog":         Sub,
	"stripped_sdh":   StrippedSDH,
}

func IsPrefered(langs []Lang, l, atm Lang, name string, out *string) bool {
	if isPreferedLang(langs, l, atm) && isPreferedSubtypeOver(*out, name) {
		*out = name
		//color.Yellowln(isPreferedLang(langs, l, atm), isPreferedSubtypeOver(*out, name), "out becomes", name)
		return true
	}
	return false
}

func isPreferedLang(langs []Lang, l, atm Lang) (b bool) {
	//println(fmt.Sprintf("%#v", l), "l idx", getIdx(langs, l))
	//println(fmt.Sprintf("%#v", atm), "atm idx", getIdx(langs, atm))
	if getIdx(langs, l) <= getIdx(langs, atm) {
		b = true
	}
	//println("is prefered lang?", b)
	return
}

func getIdx(langs []Lang, ref Lang) int {
	for i, l := range langs {
		if l.Str == ref.Str && l.Region == ref.Region {
			return i
		}
	}
	return 1000 //FIXME
}

func isPreferedSubtypeOver(curr, test string) bool {
	currval := subtypeMatcher(curr)
	testval := subtypeMatcher(test)
	//println(testval, ">", currval, "IS", testval > currval)
	return testval > currval
}

func subtypeMatcher(s string) int {
	if s == "" {
		return 0
	}
	s = strings.ToLower(s)
	for subtype, v := range refmatch {
		if strings.Contains(s, subtype) {
			return v
		}
	}
	// If nothing is specified in the subtitle file name then it's probably a regular Sub file
	return Sub
}

// The 1st subtag found is considered the region, the others are ignored
func GuessLangFromFilename(name string) (lang, region string) {
	// this int was in the original algo, not sure if I need it or not at some point
	var fn_start int
	lang = guessLangFromFilename(name, &fn_start)
	if strings.Contains(lang, "-") {
		region = strings.Split(lang, "-")[1]
		lang = strings.Split(lang, "-")[0]
		return
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
You can test if your subs are found by their algo with "mpv --msg-level=find_files=trace video.mp4"
*/
func guessLangFromFilename(name string, langStart *int) string {
	stripname := name
	var ok bool
	var i, langLength int
	for x := 0; x < 2; x++ {
		// Strip the extension and any unwanted characters
		stripname = strings.TrimSuffix(path.Base(stripname), path.Ext(stripname))
		stripname = strings.TrimSpace(stripname)

		if len(stripname) < 2 {
			return "" // return an empty string
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
			ok = false // Invalid language tag format
		}
		// if longer than 3 letters, a de
		if langLength > suffixesLength+3 && !strings.Contains(stripname[i+1:i+1+langLength], "-") {
			//color.Yellowln(string(stripname[i+1 : i+1+langLength]), "contains no '-'!")
			ok = false // Invalid language tag format
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
	return name[i+1 : i+1+langLength]
}

func placeholder34564() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
