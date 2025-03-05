package core

import (
	"testing"
	
	iso "github.com/barbashov/iso639-3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// We use the Mock types from test_helpers.go

// TrackSelectorSuite is a test suite for track selector functions
type TrackSelectorSuite struct {
	suite.Suite
	selector     TrackSelector
	mockHandler  *MockHandler
	mockLogger   *MockLogger
	mockLogEvent *MockLogEvent
}

func (suite *TrackSelectorSuite) SetupTest() {
	suite.mockHandler = NewMockHandler()
	suite.mockLogger = new(MockLogger)
	suite.mockLogEvent = new(MockLogEvent)
	
	// Setup default behavior
	suite.mockHandler.On("ZeroLog").Return(suite.mockLogger)
	suite.mockLogger.On("Debug").Return(suite.mockLogEvent)
	suite.mockLogger.On("Trace").Return(suite.mockLogEvent)
	suite.mockLogEvent.On("Int", mock.Anything, mock.Anything).Return(suite.mockLogEvent)
	suite.mockLogEvent.On("Str", mock.Anything, mock.Anything).Return(suite.mockLogEvent)
	suite.mockLogEvent.On("Msg", mock.Anything).Return()
	
	suite.selector = NewTrackSelector(suite.mockHandler)
}

func TestTrackSelectorSuite(t *testing.T) {
	suite.Run(t, new(TrackSelectorSuite))
}

func (suite *TrackSelectorSuite) TestSelectTrack_IdealMatch() {
	targetLang := Lang{Language: iso.FromAnyCode("en")}
	tracks := []AudioTrack{
		{Language: iso.FromAnyCode("fr"), Channels: "6"},
		{Language: iso.FromAnyCode("en"), Channels: "2"}, // Should select this (ideal)
		{Language: iso.FromAnyCode("en"), Channels: "6"},
	}
	
	idx, err := suite.selector.SelectTrack(tracks, targetLang)
	
	suite.NoError(err)
	suite.Equal(1, idx)
}

func (suite *TrackSelectorSuite) TestSelectTrack_LanguageMatch() {
	targetLang := Lang{Language: iso.FromAnyCode("fr")}
	tracks := []AudioTrack{
		{Language: iso.FromAnyCode("en"), Channels: "2"},
		{Language: iso.FromAnyCode("fr"), Channels: "6"}, // Should select this (language match)
		{Language: iso.FromAnyCode("de"), Channels: "2"},
	}
	
	idx, err := suite.selector.SelectTrack(tracks, targetLang)
	
	suite.NoError(err)
	suite.Equal(1, idx)
}

func (suite *TrackSelectorSuite) TestSelectTrack_FallbackWithMismatch() {
	targetLang := Lang{Language: iso.FromAnyCode("de")}
	tracks := []AudioTrack{
		{Language: iso.FromAnyCode("en"), Channels: "2"}, // Should select this (fallback)
		{Language: iso.FromAnyCode("fr"), Channels: "6"},
	}
	
	idx, err := suite.selector.SelectTrack(tracks, targetLang)
	
	suite.Error(err) // Should report language mismatch
	suite.Equal(0, idx)
	suite.Contains(err.Error(), "language mismatch")
}

func (suite *TrackSelectorSuite) TestSelectTrack_SkipAudioDescription() {
	targetLang := Lang{Language: iso.FromAnyCode("en")}
	tracks := []AudioTrack{
		{Language: iso.FromAnyCode("en"), Title: "Audio Description", Channels: "2"}, // Skip this
		{Language: iso.FromAnyCode("en"), Channels: "2"}, // Should select this
	}
	
	idx, err := suite.selector.SelectTrack(tracks, targetLang)
	
	suite.NoError(err)
	suite.Equal(1, idx)
}

func (suite *TrackSelectorSuite) TestSelectTrack_NoTracksAvailable() {
	targetLang := Lang{Language: iso.FromAnyCode("en")}
	var tracks []AudioTrack // Empty tracks
	
	idx, err := suite.selector.SelectTrack(tracks, targetLang)
	
	suite.Error(err)
	suite.Equal(-1, idx)
	suite.Contains(err.Error(), "no audio tracks found")
}

func (suite *TrackSelectorSuite) TestSelectTrack_OnlyAudioDescriptionAvailable() {
	targetLang := Lang{Language: iso.FromAnyCode("en")}
	tracks := []AudioTrack{
		{Language: iso.FromAnyCode("en"), Title: "Audio Description", Channels: "2"}, // Last resort
	}
	
	idx, err := suite.selector.SelectTrack(tracks, targetLang)
	
	suite.NoError(err) // No error since language matches
	suite.Equal(0, idx) // Use audio description as last resort
}

func (suite *TrackSelectorSuite) TestSelectTrack_PreferFirstMatch() {
	targetLang := Lang{Language: iso.FromAnyCode("en")}
	tracks := []AudioTrack{
		{Language: iso.FromAnyCode("en"), Channels: "2"}, // Should select first match
		{Language: iso.FromAnyCode("en"), Channels: "2"}, // Same quality but later
	}
	
	idx, err := suite.selector.SelectTrack(tracks, targetLang)
	
	suite.NoError(err)
	suite.Equal(0, idx) // First match wins
}

func (suite *TrackSelectorSuite) TestSelectTrack_SomeTracksWithoutLanguage() {
	targetLang := Lang{Language: iso.FromAnyCode("en")}
	tracks := []AudioTrack{
		{Language: nil, Channels: "2"},                  // No language info
		{Language: iso.FromAnyCode("en"), Channels: "2"}, // Should select this
	}
	
	idx, err := suite.selector.SelectTrack(tracks, targetLang)
	
	suite.NoError(err)
	suite.Equal(1, idx)
}

func (suite *TrackSelectorSuite) TestSelectTrack_AllTracksWithoutLanguage() {
	targetLang := Lang{Language: iso.FromAnyCode("en")}
	tracks := []AudioTrack{
		{Language: nil, Channels: "2"}, // No language info
		{Language: nil, Channels: "6"}, // No language info
	}
	
	idx, err := suite.selector.SelectTrack(tracks, targetLang)
	
	suite.Equal(0, idx) // Use first track
	// Note: There's no language to mismatch with since the track has no language
	suite.NoError(err)
}

// MockTrackSelector for testing code that uses TrackSelector
type MockTrackSelector struct {
	mock.Mock
}

func (m *MockTrackSelector) SelectTrack(tracks []AudioTrack, targetLang Lang) (int, error) {
	args := m.Called(tracks, targetLang)
	return args.Int(0), args.Error(1)
}

func TestTrackSelector_TableDriven(t *testing.T) {
	mockHandler, _, _ := createMockDependencies()
	selector := NewTrackSelector(mockHandler)
	
	tests := []struct {
		name           string
		targetLang     Lang
		tracks         []AudioTrack
		expectedIndex  int
		expectError    bool
		errorContains  string
	}{
		{
			name: "Ideal match - language and channels",
			targetLang: Lang{Language: iso.FromAnyCode("en")},
			tracks: []AudioTrack{
				{Language: iso.FromAnyCode("fr"), Channels: "6"},
				{Language: iso.FromAnyCode("en"), Channels: "2"}, // Should select this
				{Language: iso.FromAnyCode("en"), Channels: "6"},
			},
			expectedIndex: 1,
			expectError:   false,
		},
		{
			name: "Language match only",
			targetLang: Lang{Language: iso.FromAnyCode("fr")},
			tracks: []AudioTrack{
				{Language: iso.FromAnyCode("en"), Channels: "2"},
				{Language: iso.FromAnyCode("fr"), Channels: "6"}, // Should select this
			},
			expectedIndex: 1,
			expectError:   false,
		},
		{
			name: "Fallback to first non-description track",
			targetLang: Lang{Language: iso.FromAnyCode("de")},
			tracks: []AudioTrack{
				{Language: iso.FromAnyCode("en"), Channels: "2"}, // Should select this
				{Language: iso.FromAnyCode("fr"), Channels: "6"},
			},
			expectedIndex: 0,
			expectError:   true,
			errorContains: "language mismatch",
		},
		{
			name: "Skip audio description",
			targetLang: Lang{Language: iso.FromAnyCode("en")},
			tracks: []AudioTrack{
				{Language: iso.FromAnyCode("en"), Title: "Audio Description", Channels: "2"}, // Skip this
				{Language: iso.FromAnyCode("en"), Channels: "2"}, // Should select this
			},
			expectedIndex: 1,
			expectError:   false,
		},
		{
			name: "No tracks",
			targetLang: Lang{Language: iso.FromAnyCode("en")},
			tracks: []AudioTrack{},
			expectedIndex: -1,
			expectError:   true,
			errorContains: "no audio tracks found",
		},
		{
			name: "Multiple matching tracks with different channel counts",
			targetLang: Lang{Language: iso.FromAnyCode("en")},
			tracks: []AudioTrack{
				{Language: iso.FromAnyCode("en"), Channels: "6"},
				{Language: iso.FromAnyCode("en"), Channels: "2"}, // Should select this (2-channel preferred)
				{Language: iso.FromAnyCode("en"), Channels: "1"},
			},
			expectedIndex: 1,
			expectError:   false,
		},
		{
			name: "Different language but no error since first track has no language",
			targetLang: Lang{Language: iso.FromAnyCode("en")},
			tracks: []AudioTrack{
				{Language: nil, Channels: "2"}, // Should select this
				{Language: iso.FromAnyCode("fr"), Channels: "2"},
			},
			expectedIndex: 0,
			expectError:   false,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			idx, err := selector.SelectTrack(tc.tracks, tc.targetLang)
			
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
			
			assert.Equal(t, tc.expectedIndex, idx)
		})
	}
}

func TestIsAudioDescription(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected bool
	}{
		{
			name:     "Empty title",
			title:    "",
			expected: false,
		},
		{
			name:     "Audio Description",
			title:    "Audio Description",
			expected: true,
		},
		{
			name:     "Descriptive Audio",
			title:    "Descriptive Audio",
			expected: true,
		},
		{
			name:     "Commentary",
			title:    "Director's Commentary",
			expected: true,
		},
		{
			name:     "Regular title",
			title:    "English",
			expected: false,
		},
		{
			name:     "Case insensitive",
			title:    "audio DESCRIPTION",
			expected: true,
		},
		{
			name:     "Audio Description for visually impaired",
			title:    "English Audio Description for the Visually Impaired",
			expected: true,
		},
		{
			name:     "Narration variant",
			title:    "English narration track",
			expected: true,
		},
		{
			name:     "English subtitle that contains 'comment'",
			title:    "English (comments included)",
			expected: false, // Shouldn't match partial word
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isAudioDescription(tc.title)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTrackSelector_Basic(t *testing.T) {
	// Simple test for basic track selection
	handler := NewMockHandler()
	selector := NewTrackSelector(handler)
	
	targetLang := Lang{Language: iso.FromAnyCode("en")}
	tracks := []AudioTrack{
		{Language: iso.FromAnyCode("en"), Channels: "2"},
	}
	
	idx, err := selector.SelectTrack(tracks, targetLang)
	assert.NoError(t, err)
	assert.Equal(t, 0, idx)
}

// Helper function to create mock dependencies
func createMockDependencies() (*MockHandler, *MockLogger, *MockLogEvent) {
	mockHandler := NewMockHandler()
	mockLogger := new(MockLogger)
	mockLogEvent := new(MockLogEvent)
	
	// Setup is already done in NewMockHandler()
	
	return mockHandler, mockLogger, mockLogEvent
}
