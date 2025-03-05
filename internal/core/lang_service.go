package core

import (
	"fmt"
	"path"
	"strings"
	"unicode"

	iso "github.com/barbashov/iso639-3"
)

// DefaultLanguageDetector implements the LanguageDetector interface
type DefaultLanguageDetector struct{}

// NewLanguageDetector creates a new DefaultLanguageDetector
func NewLanguageDetector() LanguageDetector {
	return &DefaultLanguageDetector{}
}

// GuessLangFromFilename implements LanguageDetector interface
func (d *DefaultLanguageDetector) GuessLangFromFilename(name string) (Lang, error) {
	var lang Lang
	var fnStart int
	
	// Extract language code from filename
	langCode := d.guessLangFromFilename(name, &fnStart)
	
	// Handle subtags if present
	if arr := strings.Split(langCode, "-"); strings.Contains(langCode, "-") {
		lang.Subtag = strings.ToLower(arr[1])
		langCode = arr[0]
	}
	
	// Look up the language code
	lang.Language = iso.FromAnyCode(langCode)
	if lang.Language == nil {
		return lang, fmt.Errorf("no language could be identified: lang='%s'", langCode)
	}
	
	return lang, nil
}

// ParseLanguageTags implements LanguageDetector interface
func (d *DefaultLanguageDetector) ParseLanguageTags(langTagString string) []Lang {
	tags := strings.Split(langTagString, ",")
	for i := range tags {
		tags[i] = strings.TrimSpace(tags[i])
	}
	
	langs, _ := parseLanguageTagsInternal(tags) // Ignoring error for interface compatibility
	return langs
}

// guessLangFromFilename extracts language information from a filename
func (d *DefaultLanguageDetector) guessLangFromFilename(name string, langStart *int) string {
	stripname := stripCommonSubsMention(path.Base(name))
	var ok bool
	var i, langLength int
	
	// This loop attempts to extract language code by progressively trimming the filename
	for x := 0; x < 3; x++ {
		// Trim extension during 1st loop and then in further loops, attempt to
		// decorticate any potential dot-separated irrelevant info
		stripname = strings.TrimSuffix(stripname, path.Ext(stripname))
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
				break
			}
		}
		
		// Validate the primary subtag's length (2-3 letters)
		if langLength < suffixesLength+2 || langLength > suffixesLength+3 || i <= 0 || rune(stripname[i]) != delimiter {
			ok = false
		}
		
		// If longer than 3 letters, must have hyphen and subtag
		if langLength > suffixesLength+3 && !strings.Contains(stripname[i+1:i+1+langLength], "-") {
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

// Helper functions extracted for better testability

// parseLanguageTagsInternal parses an array of language tag strings into Lang objects
func parseLanguageTagsInternal(arr []string) (langs []Lang, err error) {
	if len(arr) == 0 {
		return langs, fmt.Errorf("empty slice passed to ParseLanguageTags")
	}
	
	for _, tmp := range arr {
		var lang Lang
		subTags := strings.Split(tmp, "-")
		lang.Language = iso.FromAnyCode(subTags[0])
		if lang.Language == nil {
			// Handle common confusion with Japanese code
			if subTags[0] == "jp" {
				return nil, fmt.Errorf("'%s' is not a valid ISO-639 code, " +
				"for Japanese the code to use is either 'ja' or 'jpn'", subTags[0])
			}
			return nil, fmt.Errorf("an invalid language code was passed: '%s'", subTags[0])
		}
		if len(subTags) > 1 {
			lang.Subtag = strings.ToLower(subTags[1])
		}
		langs = append(langs, lang)
	}
	return
}

// These functions are already declared in lang.go
// Removed to avoid redeclaration