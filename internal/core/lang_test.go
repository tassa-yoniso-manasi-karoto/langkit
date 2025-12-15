package core

import (
	"testing"

	iso "github.com/barbashov/iso639-3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeLang creates a Lang struct from an ISO 639 code and optional subtag
func makeLang(code, subtag string) Lang {
	return Lang{
		Language: iso.FromAnyCode(code),
		Subtag:   subtag,
	}
}

func TestIsScriptSubtag(t *testing.T) {
	tests := []struct {
		name     string
		subtag   string
		expected bool
	}{
		{"hans is script (4 letters)", "hans", true},
		{"hant is script (4 letters)", "hant", true},
		{"latn is script (4 letters)", "latn", true},
		{"cyrl is script (4 letters)", "cyrl", true},
		{"us is not script (2 letters)", "us", false},
		{"yue is not script (3 letters)", "yue", false},
		{"empty is not script", "", false},
		{"single letter is not script", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isScriptSubtag(tt.subtag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRegionSubtag(t *testing.T) {
	tests := []struct {
		name     string
		subtag   string
		expected bool
	}{
		{"us is region (2 letters)", "us", true},
		{"gb is region (2 letters)", "gb", true},
		{"br is region (2 letters)", "br", true},
		{"in is region (2 letters)", "in", true},
		{"hans is not region (4 letters)", "hans", false},
		{"yue is not region (3 letters)", "yue", false},
		{"empty is not region", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRegionSubtag(tt.subtag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsExtlangSubtag(t *testing.T) {
	tests := []struct {
		name     string
		subtag   string
		expected bool
	}{
		{"yue is extlang (Cantonese)", "yue", true},
		{"cmn is extlang (Mandarin)", "cmn", true},
		{"eng is extlang (English)", "eng", true},
		{"jpn is extlang (Japanese)", "jpn", true},
		{"xyz is not extlang (invalid ISO)", "xyz", false},
		{"qqq is not extlang (invalid ISO)", "qqq", false},
		{"us is not extlang (2 letters)", "us", false},
		{"hans is not extlang (4 letters)", "hans", false},
		{"empty is not extlang", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExtlangSubtag(tt.subtag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubtagQuality(t *testing.T) {
	tests := []struct {
		name             string
		requestedSubtag  string
		candidateSubtag  string
		langCode         string
		expectedQuality  int
	}{
		// Exact matches
		{"exact match hans", "hans", "hans", "zho", 100},
		{"exact match us", "us", "us", "eng", 100},
		{"exact match empty", "", "", "zho", 100},

		// User didn't specify subtag (requestedSubtag == "")
		{"no request, generic candidate", "", "", "zho", 100}, // Same as exact match
		{"no request, default script hans for zho", "", "hans", "zho", 85},
		{"no request, non-default script hant for zho", "", "hant", "zho", 0},
		{"no request, default script hans for yue", "", "hans", "yue", 85},
		{"no request, preferred region us for eng", "", "us", "eng", 85},
		{"no request, secondary region gb for eng", "", "gb", "eng", 80},
		{"no request, other region in for eng", "", "in", "eng", 50},
		{"no request, other region br for por", "", "br", "por", 50},

		// User specified subtag but candidate is generic
		{"requested hans, candidate generic", "hans", "", "zho", 70},
		{"requested us, candidate generic", "us", "", "eng", 70},

		// Mismatched subtags
		{"hans vs hant mismatch", "hans", "hant", "zho", 0},
		{"us vs gb mismatch", "us", "gb", "eng", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := subtagQuality(tt.requestedSubtag, tt.candidateSubtag, tt.langCode)
			assert.Equal(t, tt.expectedQuality, result)
		})
	}
}

func TestParseLanguageTags(t *testing.T) {
	tests := []struct {
		name            string
		input           []string
		expectedLang    string // ISO 639-3 code
		expectedSubtag  string
	}{
		{"zh-Hans parses to zho with hans subtag", []string{"zh-Hans"}, "zho", "hans"},
		{"zh-Hant parses to zho with hant subtag", []string{"zh-Hant"}, "zho", "hant"},
		{"zh parses to zho with no subtag", []string{"zh"}, "zho", ""},
		{"zh-yue resolves to yue (extlang)", []string{"zh-yue"}, "yue", ""},
		{"zh-yue-Hans resolves to yue with hans subtag", []string{"zh-yue-Hans"}, "yue", "hans"},
		{"en-US parses to eng with us subtag", []string{"en-US"}, "eng", "us"},
		{"en-GB parses to eng with gb subtag", []string{"en-GB"}, "eng", "gb"},
		{"en parses to eng with no subtag", []string{"en"}, "eng", ""},
		{"pt-BR parses to por with br subtag", []string{"pt-BR"}, "por", "br"},
		{"ja parses to jpn with no subtag", []string{"ja"}, "jpn", ""},
		{"jpn parses to jpn with no subtag", []string{"jpn"}, "jpn", ""},
		{"yue directly parses to yue", []string{"yue"}, "yue", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			langs, err := ParseLanguageTags(tt.input)
			require.NoError(t, err)
			require.Len(t, langs, 1)

			assert.Equal(t, tt.expectedLang, langs[0].Part3, "language code mismatch")
			assert.Equal(t, tt.expectedSubtag, langs[0].Subtag, "subtag mismatch")
		})
	}
}

func TestParseLanguageTags_Multiple(t *testing.T) {
	// Test parsing multiple language tags at once
	input := []string{"zh-Hans", "en-US", "ja"}
	langs, err := ParseLanguageTags(input)
	require.NoError(t, err)
	require.Len(t, langs, 3)

	assert.Equal(t, "zho", langs[0].Part3)
	assert.Equal(t, "hans", langs[0].Subtag)

	assert.Equal(t, "eng", langs[1].Part3)
	assert.Equal(t, "us", langs[1].Subtag)

	assert.Equal(t, "jpn", langs[2].Part3)
	assert.Equal(t, "", langs[2].Subtag)
}

func TestParseLanguageTags_Errors(t *testing.T) {
	// Test error cases
	_, err := ParseLanguageTags([]string{})
	assert.Error(t, err, "empty slice should return error")

	_, err = ParseLanguageTags([]string{"invalidlang123"})
	assert.Error(t, err, "invalid language code should return error")

	_, err = ParseLanguageTags([]string{"jp"})
	assert.Error(t, err, "jp (domain) should return helpful error about using ja/jpn")
}

func TestGuessLangFromFilename(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		expectedLang   string // ISO 639-3 code
		expectedSubtag string
		expectError    bool
	}{
		// Chinese variants
		{"zh-Hans subtitle", "Movie_S01E01.zh-Hans.subtitles.srt", "zho", "hans", false},
		{"zh-Hant subtitle", "Movie_S01E01.zh-Hant.subtitles.srt", "zho", "hant", false},
		{"zh generic subtitle", "Movie_S01E01.zh.subtitles.srt", "zho", "", false},
		{"zh-yue Cantonese subtitle", "Movie_S01E01.zh-yue.subtitles.srt", "yue", "", false},
		// Note: zh-yue-Hans (triple tag) is not tested as mpv algorithm may not support it

		// English variants
		{"en-US subtitle", "Movie_S01E01.en-US.subtitles.srt", "eng", "us", false},
		{"en-GB subtitle", "Movie_S01E01.en-GB.subtitles.srt", "eng", "gb", false},
		{"en-IN subtitle", "Movie_S01E01.en-IN.subtitles.srt", "eng", "in", false},
		{"en generic subtitle", "Movie_S01E01.en.subtitles.srt", "eng", "", false},

		// Other languages
		{"ja subtitle", "Movie_S01E01.ja.subtitles.srt", "jpn", "", false},
		{"pt-BR subtitle", "Movie_S01E01.pt-BR.subtitles.srt", "por", "br", false},
		{"ru subtitle", "Movie_S01E01.ru.subtitles.srt", "rus", "", false},

		// Different filename patterns
		{"brackets pattern", "Movie (2024) [zh-Hans].srt", "zho", "hans", false},
		{"parentheses pattern", "Movie (2024) (en-US).srt", "eng", "us", false},
		{"simple pattern", "movie.en.srt", "eng", "", false},
		{"dialog pattern", "Movie.zh.dialog.srt", "zho", "", false},

		// Error cases
		{"no language tag", "Movie_S01E01.srt", "", "", true},
		{"invalid tag", "Movie_S01E01.xyz123.srt", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang, err := GuessLangFromFilename(tt.filename)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedLang, lang.Part3, "language code mismatch for %s", tt.filename)
			assert.Equal(t, tt.expectedSubtag, lang.Subtag, "subtag mismatch for %s", tt.filename)
		})
	}
}

func TestGetIdx_ScriptAware(t *testing.T) {
	tests := []struct {
		name          string
		userLangs     []Lang // What user requested
		candidate     Lang   // File's language
		expectedMatch bool
		expectedIdx   int
	}{
		// Chinese with script subtags
		{
			"zho matches zh-hans (default script)",
			[]Lang{makeLang("zho", "")},
			makeLang("zho", "hans"),
			true, 0,
		},
		{
			"zho does NOT match zh-hant (non-default script)",
			[]Lang{makeLang("zho", "")},
			makeLang("zho", "hant"),
			false, 0,
		},
		{
			"zho matches zh (no subtag)",
			[]Lang{makeLang("zho", "")},
			makeLang("zho", ""),
			true, 0,
		},
		{
			"zh-hans matches zh-hans exactly",
			[]Lang{makeLang("zho", "hans")},
			makeLang("zho", "hans"),
			true, 0,
		},
		{
			"zh-hans does NOT match zh-hant",
			[]Lang{makeLang("zho", "hans")},
			makeLang("zho", "hant"),
			false, 0,
		},
		{
			"zh-hans matches zh (generic fallback)",
			[]Lang{makeLang("zho", "hans")},
			makeLang("zho", ""),
			true, 0,
		},

		// English with regional subtags
		{
			"en matches en-us (regional)",
			[]Lang{makeLang("eng", "")},
			makeLang("eng", "us"),
			true, 0,
		},
		{
			"en matches en-in (regional)",
			[]Lang{makeLang("eng", "")},
			makeLang("eng", "in"),
			true, 0,
		},
		{
			"en matches en (no subtag)",
			[]Lang{makeLang("eng", "")},
			makeLang("eng", ""),
			true, 0,
		},

		// Language priority (multiple user langs)
		{
			"first lang preferred over second",
			[]Lang{makeLang("zho", ""), makeLang("eng", "")},
			makeLang("zho", "hans"),
			true, 0,
		},
		{
			"second lang matches at index 1",
			[]Lang{makeLang("zho", ""), makeLang("eng", "")},
			makeLang("eng", "us"),
			true, 1,
		},

		// No match cases
		{
			"different language doesn't match",
			[]Lang{makeLang("zho", "")},
			makeLang("eng", "us"),
			false, 0,
		},
		{
			"nil candidate doesn't match",
			[]Lang{makeLang("zho", "")},
			Lang{Language: nil, Subtag: ""},
			false, 0,
		},

		// Strict subtag matching (user specified subtag)
		{
			"en-us does NOT match en-in (strict)",
			[]Lang{makeLang("eng", "us")},
			makeLang("eng", "in"),
			false, 0,
		},
		{
			"en-us does NOT match en-gb (strict)",
			[]Lang{makeLang("eng", "us")},
			makeLang("eng", "gb"),
			false, 0,
		},
		{
			"de-at does NOT match de-de (strict)",
			[]Lang{makeLang("deu", "at")},
			makeLang("deu", "de"),
			false, 0,
		},
		{
			"en-us matches en (generic fallback)",
			[]Lang{makeLang("eng", "us")},
			makeLang("eng", ""),
			true, 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, matched := getIdx(tt.userLangs, tt.candidate)
			assert.Equal(t, tt.expectedMatch, matched, "match status mismatch")
			if matched {
				assert.Equal(t, tt.expectedIdx, idx, "index mismatch")
			}
		})
	}
}

func TestSetPreferredLang_QualityComparison(t *testing.T) {
	// Create a no-op logger for tests
	logger := zerolog.Nop()

	tests := []struct {
		name           string
		userLangs      []Lang
		candidate      Lang
		current        Lang
		expectedResult bool // true if candidate should be preferred
	}{
		// Script quality comparison
		{
			"zh-hans beats zh-hant when user requests zho",
			[]Lang{makeLang("zho", "")},
			makeLang("zho", "hans"), // candidate: quality 85 (default script)
			makeLang("zho", "hant"), // current: quality 0 (non-default, doesn't match)
			true,
		},
		{
			"zh (generic) beats zh-hans when user requests zho",
			[]Lang{makeLang("zho", "")},
			makeLang("zho", ""),     // candidate: quality 90 (generic)
			makeLang("zho", "hans"), // current: quality 85 (default script)
			true,
		},
		{
			"zh-hans does NOT beat zh (generic) when user requests zho",
			[]Lang{makeLang("zho", "")},
			makeLang("zho", "hans"), // candidate: quality 85
			makeLang("zho", ""),     // current: quality 90
			false,
		},

		// Regional quality comparison
		{
			"en-us beats en-in when user requests en",
			[]Lang{makeLang("eng", "")},
			makeLang("eng", "us"), // candidate: quality 85 (preferred region)
			makeLang("eng", "in"), // current: quality 50 (other region)
			true,
		},
		{
			"en-gb beats en-in when user requests en",
			[]Lang{makeLang("eng", "")},
			makeLang("eng", "gb"), // candidate: quality 80
			makeLang("eng", "in"), // current: quality 50
			true,
		},
		{
			"en (generic) beats en-us when user requests en",
			[]Lang{makeLang("eng", "")},
			makeLang("eng", ""),   // candidate: quality 90 (generic)
			makeLang("eng", "us"), // current: quality 85
			true,
		},
		{
			"en-in does NOT beat en-us when user requests en",
			[]Lang{makeLang("eng", "")},
			makeLang("eng", "in"), // candidate: quality 50
			makeLang("eng", "us"), // current: quality 85
			false,
		},

		// First valid match wins over non-matching current
		{
			"candidate wins when current doesn't match user request",
			[]Lang{makeLang("zho", "")},
			makeLang("zho", "hans"),        // candidate matches user request
			makeLang("eng", "us"),           // current is different language (doesn't match)
			true,
		},

		// Language priority overrides quality
		{
			"first language always beats second language regardless of quality",
			[]Lang{makeLang("zho", ""), makeLang("eng", "")},
			makeLang("zho", "hans"), // candidate: first lang, quality 85
			makeLang("eng", ""),     // current: second lang, quality 90
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setPreferredLang(tt.userLangs, tt.candidate, tt.current, &logger)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestDefaultScripts(t *testing.T) {
	// Verify the defaultScripts map contains expected entries
	assert.Equal(t, "hans", defaultScripts["zho"], "Chinese should default to Simplified")
	assert.Equal(t, "hans", defaultScripts["yue"], "Cantonese should default to Simplified")
	assert.Equal(t, "hans", defaultScripts["cmn"], "Mandarin should default to Simplified")
}
