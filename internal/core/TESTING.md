# Langkit Integration Testing Guide

This document provides a guide to the integration testing framework for Langkit.

## Running Tests

To run the integration tests, you need to provide test media and subtitle files via environment variables:

```bash
# Set test media and subtitle file paths
export LANGKIT_TEST_MEDIA_FILE=/path/to/your/video.mp4
export LANGKIT_TEST_SUBTITLE_FILE=/path/to/your/subtitle.srt
export LANGKIT_USE_MOCK_PROVIDERS=true

# Run all tests
go test ./internal/core -v

# Run specific test 
go test ./internal/core -run TestSubs2Cards -v

# Run with profiling for transliteration tests
LANGKIT_PROFILE_TRANSLIT=1 go test -v ./internal/core -run TestSelectiveTransliteration
```

## Test Files

For best results:
- Use short video files (30-60 seconds)
- Use Japanese subtitles for transliteration tests
- Standard formats work best (MP4 for video, SRT for subtitles)

## Key Features

1. **Mock External Providers**: Tests simulate external API services without making real calls
2. **Output Verification**: Tests validate that the correct output files are generated
3. **Feature Combinations**: Comprehensive tests for different feature combinations
4. **Realistic Processing**: Tests actual processing without GUI/CLI dependencies

## Framework Components

- **Provider Interfaces**: `AIServiceProvider`, `SpeechToTextProvider`, `AudioSeparationProvider`
- **Task Configuration**: `TaskConfig` with comprehensive options
- **Test Expectations**: `TaskExpectation` for verifying outputs
- **Output Registry**: Interface for tracking and verifying output files

## Testing Approaches

Langkit supports two different approaches to integration testing:

### 1. Direct Task-Based Testing

This approach directly configures a `Task` using the `TaskConfig` structure.

Example:
```go
func TestMyFeature(t *testing.T) {
    TestSkipIfNoMediaFile(t)
    
    config := TaskConfig{
        Mode: YourMode,
        UseMockProviders: true,
        TargetLanguage: Lang{Language: iso.FromPart2Code("ja")},
        // Other settings...
    }
    
    expectations := TaskExpectation{
        ShouldSucceed: true,
        OutputExtensions: map[string]bool{
            ".expected-extension": true,
        },
    }
    
    t.Run("YourTest", func(t *testing.T) {
        RunTaskTest(t, config, expectations)
    })
}
```

### 2. GUI Integration Testing

This approach simulates the GUI frontend requests, closely matching how the application behaves in real usage.

Example:
```go
func TestGUIFeature(t *testing.T) {
    TestSkipIfNoMediaFile(t)
    
    // Create a request similar to what the GUI would send
    request := GUIProcessRequest{
        Path: os.Getenv(EnvTestMediaFile),
        SelectedFeatures: map[string]bool{
            "subtitleRomanization": true,
        },
        Options: map[string]map[string]interface{}{
            "subtitleRomanization": {
                "style":            "hepburn",
                "mergeOutputFiles": true,
            },
        },
        LanguageCode: "ja",
    }
    
    expectations := TaskExpectation{
        ShouldSucceed: true,
        OutputExtensions: map[string]bool{
            ".ROMANIZED.srt": true,
        },
    }
    
    t.Run("GUITest", func(t *testing.T) {
        RunGUITest(t, request, expectations)
    })
}
```

The GUI testing approach provides these advantages:
- Tests the exact same processing flow as the real application
- Ensures frontend feature selection works correctly
- Validates the behavior of feature combinations as seen by users

## Testing Feature Combinations

One key advantage of the testing framework is the ability to test combinations of features:

```go
// Direct approach
config := TaskConfig{
    Mode: Subs2Cards,
    WantTranslit: true,
    TranslitTypes: []TranslitType{Romanize},
    // ...
}

// GUI approach
request := GUIProcessRequest{
    SelectedFeatures: map[string]bool{
        "subs2cards": true,
        "subtitleRomanization": true,
    },
    // ...
}
```

## Real vs. Mock Testing

By default, tests use mock providers. For testing with real APIs:

1. Set `UseMockProviders: false` in your test config or omit the `LANGKIT_USE_MOCK_PROVIDERS` environment variable
2. Configure API keys in your environment:
   - `REPLICATE_API_KEY`
   - `ASSEMBLYAI_API_KEY`
   - `ELEVENLABS_API_KEY`

## Example Tests

See:
- `integration_test.go` for individual feature tests
- `feature_combo_test.go.example` for advanced feature combination tests