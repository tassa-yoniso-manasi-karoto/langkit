package core

import (
	"testing"
	"io"
	
	"github.com/rs/zerolog"
	iso "github.com/barbashov/iso639-3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/google/go-cmp/cmp"
)

// LanguageDetectionSuite is a test suite for language detection functions
type LanguageDetectionSuite struct {
	suite.Suite
	detector LanguageDetector
}

func (suite *LanguageDetectionSuite) SetupTest() {
	suite.detector = NewLanguageDetector()
}

func TestLanguageDetectionSuite(t *testing.T) {
	suite.Run(t, new(LanguageDetectionSuite))
}

func (suite *LanguageDetectionSuite) TestGuessLangFromFilename() {
	tests := []struct {
		name           string
		filename       string
		expectedLang   string
		expectedSubtag string
		expectError    bool
	}{
		// Basic language code detection
		{
			name:           "Simple English",
			filename:       "movie.en.srt",
			expectedLang:   "en",
			expectedSubtag: "",
			expectError:    false,
		},
		{
			name:           "English with Region",
			filename:       "movie.en-US.srt",
			expectedLang:   "en",
			expectedSubtag: "us",
			expectError:    false,
		},
		{
			name:           "Japanese",
			filename:       "movie.ja.srt",
			expectedLang:   "ja",
			expectedSubtag: "",
			expectError:    false,
		},
		
		// Different separators and formats
		{
			name:           "Bracketed Code",
			filename:       "movie [fr].srt",
			expectedLang:   "fr",
			expectedSubtag: "",
			expectError:    false,
		},
		{
			name:           "Bracketed Code with Region",
			filename:       "movie [pt-BR].srt",
			expectedLang:   "pt",
			expectedSubtag: "br",
			expectError:    false,
		},
		{
			name:           "With Subtitle Keyword",
			filename:       "movie.subtitles.de.srt",
			expectedLang:   "de",
			expectedSubtag: "",
			expectError:    false,
		},
		{
			name:           "Parentheses Format",
			filename:       "movie (es).srt",
			expectedLang:   "es",
			expectedSubtag: "",
			expectError:    false,
		},
		
		// Complex filenames
		{
			name:           "Complex Filename with Dots",
			filename:       "The.Movie.Title.2023.1080p.WEB-DL.AAC5.1.x264-GROUP.ja.srt",
			expectedLang:   "ja",
			expectedSubtag: "",
			expectError:    false,
		},
		{
			name:           "Complex Filename with Multiple Codes",
			filename:       "Movie.Title.2023.1080p.BluRay.DTS-HD.MA.5.1.x264-GROUP.en-US.srt",
			expectedLang:   "en",
			expectedSubtag: "us",
			expectError:    false,
		},
		
		// Subtitle keywords
		{
			name:           "With closedcaptions Keyword",
			filename:       "movie.closedcaptions.en.srt",
			expectedLang:   "en",
			expectedSubtag: "",
			expectError:    false,
		},
		{
			name:           "With dialog Keyword",
			filename:       "movie.dialog.fr.srt",
			expectedLang:   "fr",
			expectedSubtag: "",
			expectError:    false,
		},
		{
			name:           "With dubtitles Keyword",
			filename:       "movie.dubtitles.de.srt",
			expectedLang:   "de",
			expectedSubtag: "",
			expectError:    false,
		},
		
		// Error cases
		{
			name:           "Invalid Language Code",
			filename:       "movie.xx.srt",
			expectedLang:   "",
			expectedSubtag: "",
			expectError:    true,
		},
		{
			name:           "No Language Code",
			filename:       "movie.srt",
			expectedLang:   "",
			expectedSubtag: "",
			expectError:    true,
		},
		{
			name:           "Very Short Filename",
			filename:       "m.srt",
			expectedLang:   "",
			expectedSubtag: "",
			expectError:    true,
		},
	}
	
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			lang, err := suite.detector.GuessLangFromFilename(tc.filename)
			
			if tc.expectError {
				suite.Error(err)
				return
			}
			
			suite.NoError(err)
			suite.Equal(tc.expectedLang, Str(lang.Language))
			suite.Equal(tc.expectedSubtag, lang.Subtag)
		})
	}
}

func (suite *LanguageDetectionSuite) TestParseLanguageTags() {
	tests := []struct {
		name           string
		tagString      string
		expectedCount  int
		expectedLangs  []string
		expectedSubtags []string
	}{
		// Basic parsing
		{
			name:           "Simple Language",
			tagString:      "en",
			expectedCount:  1,
			expectedLangs:  []string{"en"},
			expectedSubtags: []string{""},
		},
		{
			name:           "Multiple Languages",
			tagString:      "en, fr, de",
			expectedCount:  3,
			expectedLangs:  []string{"en", "fr", "de"},
			expectedSubtags: []string{"", "", ""},
		},
		
		// Regions and scripts
		{
			name:           "Languages with Regions",
			tagString:      "en-US, pt-BR",
			expectedCount:  2,
			expectedLangs:  []string{"en", "pt"},
			expectedSubtags: []string{"us", "br"},
		},
		{
			name:           "Languages with Scripts",
			tagString:      "zh-Hant, zh-Hans",
			expectedCount:  2,
			expectedLangs:  []string{"zh", "zh"},
			expectedSubtags: []string{"hant", "hans"},
		},
		
		// Mixed and complex
		{
			name:           "Mixed Tags",
			tagString:      "en, ja-JP, zh-Hans",
			expectedCount:  3,
			expectedLangs:  []string{"en", "ja", "zh"},
			expectedSubtags: []string{"", "jp", "hans"},
		},
		{
			name:           "Extra Whitespace",
			tagString:      "  en  ,  fr  ,  de  ",
			expectedCount:  3,
			expectedLangs:  []string{"en", "fr", "de"},
			expectedSubtags: []string{"", "", ""},
		},
		
		// Using 3-letter codes
		{
			name:           "Three-letter Codes",
			tagString:      "eng, fra, deu",
			expectedCount:  3,
			expectedLangs:  []string{"en", "fr", "de"},
			expectedSubtags: []string{"", "", ""},
		},
		
		// Mixed 2-letter and 3-letter codes
		{
			name:           "Mixed Code Lengths",
			tagString:      "en, fra, de",
			expectedCount:  3,
			expectedLangs:  []string{"en", "fr", "de"},
			expectedSubtags: []string{"", "", ""},
		},
	}
	
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			langs := suite.detector.ParseLanguageTags(tc.tagString)
			
			suite.Equal(tc.expectedCount, len(langs))
			
			for i, expectedLang := range tc.expectedLangs {
				if i < len(langs) {
					suite.Equal(expectedLang, Str(langs[i].Language))
					suite.Equal(tc.expectedSubtags[i], langs[i].Subtag)
				}
			}
		})
	}
}

func (suite *LanguageDetectionSuite) TestLanguageString() {
	tests := []struct {
		name     string
		language *iso.Language
		expected string
	}{
		{
			name:     "English with part1",
			language: iso.FromAnyCode("en"),
			expected: "en",
		},
		{
			name:     "Japanese with part1",
			language: iso.FromAnyCode("ja"),
			expected: "ja",
		},
		{
			name:     "French with part1",
			language: iso.FromAnyCode("fr"),
			expected: "fr",
		},
		{
			name:     "Language with only part3 (no part1)",
			language: iso.FromAnyCode("tlh"), // Klingon
			expected: "tlh",
		},
		{
			name:     "Null language",
			language: nil,
			expected: "und",
		},
	}
	
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			suite.Equal(tc.expected, Str(tc.language))
		})
	}
}

// MockLanguageDetector for testing code that uses LanguageDetector
type MockLanguageDetector struct {
	mock.Mock
}

func (m *MockLanguageDetector) GuessLangFromFilename(filename string) (Lang, error) {
	args := m.Called(filename)
	return args.Get(0).(Lang), args.Error(1)
}

func (m *MockLanguageDetector) ParseLanguageTags(langTag string) []Lang {
	args := m.Called(langTag)
	return args.Get(0).([]Lang)
}

// Additional test for setPreferredLang function
func TestSetPreferredLang(t *testing.T) {
	tests := []struct {
		name         string
		prefLangs    []Lang
		lang         Lang
		current      Lang
		expected     bool
	}{
		{
			name:      "First preferred language",
			prefLangs: []Lang{
				{Language: iso.FromAnyCode("en")},
				{Language: iso.FromAnyCode("fr")},
			},
			lang:     Lang{Language: iso.FromAnyCode("en")},
			current:  Lang{Language: iso.FromAnyCode("fr")},
			expected: true,
		},
		{
			name:      "Second preferred language vs third",
			prefLangs: []Lang{
				{Language: iso.FromAnyCode("en")},
				{Language: iso.FromAnyCode("fr")},
				{Language: iso.FromAnyCode("de")},
			},
			lang:     Lang{Language: iso.FromAnyCode("fr")},
			current:  Lang{Language: iso.FromAnyCode("de")},
			expected: true,
		},
		{
			name:      "Lower preference language",
			prefLangs: []Lang{
				{Language: iso.FromAnyCode("en")},
				{Language: iso.FromAnyCode("fr")},
			},
			lang:     Lang{Language: iso.FromAnyCode("fr")},
			current:  Lang{Language: iso.FromAnyCode("en")},
			expected: false,
		},
		{
			name:      "Lang not in preferences",
			prefLangs: []Lang{
				{Language: iso.FromAnyCode("en")},
				{Language: iso.FromAnyCode("fr")},
			},
			lang:     Lang{Language: iso.FromAnyCode("de")},
			current:  Lang{Language: iso.FromAnyCode("fr")},
			expected: false,
		},
		{
			name:      "Same language",
			prefLangs: []Lang{
				{Language: iso.FromAnyCode("en")},
				{Language: iso.FromAnyCode("fr")},
			},
			lang:     Lang{Language: iso.FromAnyCode("en")},
			current:  Lang{Language: iso.FromAnyCode("en")},
			expected: true, // Same position, so we keep the current (not strictly preferred)
		},
		{
			name:      "With subtags",
			prefLangs: []Lang{
				{Language: iso.FromAnyCode("en"), Subtag: "us"},
				{Language: iso.FromAnyCode("en"), Subtag: "gb"},
			},
			lang:     Lang{Language: iso.FromAnyCode("en"), Subtag: "us"},
			current:  Lang{Language: iso.FromAnyCode("en"), Subtag: "gb"},
			expected: true,
		},
	}
	logger := zerolog.New(io.Discard).With().Timestamp().Logger()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := setPreferredLang(tc.prefLangs, tc.lang, tc.current, &logger)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test for isPreferredSubtypeOver function
func TestIsPreferredSubtypeOver(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		candidate string
		expected  bool
	}{
		{
			name:      "Regular subtitle vs empty",
			current:   "",
			candidate: "movie.en.srt",
			expected:  false,
		},
		{
			name:      "Closed captions vs regular subtitles",
			current:   "movie.en.srt",
			candidate: "movie.closedcaptions.en.srt",
			expected:  true,
		},
		{
			name:      "Dubtitles vs regular subtitles",
			current:   "movie.en.srt",
			candidate: "movie.dubtitles.en.srt",
			expected:  true,
		},
		{
			name:      "Stripped SDH vs regular subtitles",
			current:   "movie.en.srt",
			candidate: "movie.stripped_sdh.en.srt",
			expected:  true,
		},
		{
			name:      "Regular subtitle vs better type",
			current:   "movie.closedcaptions.en.srt",
			candidate: "movie.en.srt",
			expected:  false,
		},
	}
	
	logger := zerolog.New(io.Discard).With().Timestamp().Logger()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isPreferredSubtypeOver(tc.current, tc.candidate, &logger)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test with go-cmp for comparing Lang objects 
func TestLangEquality(t *testing.T) {
	en1 := Lang{Language: iso.FromAnyCode("en")}
	en2 := Lang{Language: iso.FromAnyCode("en")}
	enUS := Lang{Language: iso.FromAnyCode("en"), Subtag: "us"}
	fr := Lang{Language: iso.FromAnyCode("fr")}
	
	// Using go-cmp for deep equality testing
	if diff := cmp.Diff(en1, en2); diff != "" {
		t.Errorf("Languages should be equal, but got diff: %s", diff)
	}
	
	if diff := cmp.Diff(en1, enUS); diff == "" {
		t.Errorf("Languages with different subtags should not be equal")
	}
	
	if diff := cmp.Diff(en1, fr); diff == "" {
		t.Errorf("Different languages should not be equal")
	}
}