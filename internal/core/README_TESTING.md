# Testing Guide for Langkit Core Package

This document provides an overview of the testing approach for the Langkit core package, focusing on the new abstraction and interface-based design patterns implemented for improved testability.

## Architecture Overview

The core package now follows a dependency injection and interface-based approach. Key areas have been refactored to use interfaces:

1. **Language Detection**
   - `LanguageDetector` interface
   - `DefaultLanguageDetector` implementation

2. **Audio Track Selection**
   - `TrackSelector` interface
   - `DefaultTrackSelector` implementation

3. **Concurrency Handling**
   - `WorkerPool` interface
   - `DefaultWorkerPool` implementation

4. **Error Recovery and Resumption Logic**
   - `ResumptionService` and `FileScanner` interfaces
   - Default implementations for both

5. **Path Construction**
   - `PathService` and `PathSanitizer` interfaces
   - Default implementations for both

6. **Media Information Processing**
   - `MediaInfoProvider` interface
   - `DefaultMediaInfoProvider` implementation

7. **Subtitle Handling**
   - `SubtitleProvider` interface
   - `DefaultSubtitleProvider` implementation

## Running Tests

To run tests for the refactored components, use the standard Go testing tools:

```bash
# Run all tests in the core package
go test github.com/tassa-yoniso-manasi-karoto/langkit/internal/core

# Run tests for a specific component with more verbose output
go test github.com/tassa-yoniso-manasi-karoto/langkit/internal/core -run TestLanguageDetection -v

# Run tests with coverage reporting
go test github.com/tassa-yoniso-manasi-karoto/langkit/internal/core -cover
```

## Writing Tests

When writing tests for components in this package, follow these guidelines:

1. **Use Table-Driven Tests**: Prefer table-driven tests to cover multiple scenarios efficiently
2. **Mock Dependencies**: Use mock implementations of interfaces to isolate the component under test
3. **Test Edge Cases**: Include edge cases and error conditions in your test cases
4. **Check Expectations**: Verify that mock objects were called as expected

Example of a test for a component:

```go
func TestSomeComponent(t *testing.T) {
    // Setup mocks
    mockDependency := new(MockDependency)
    mockDependency.On("SomeMethod", mock.Anything).Return(expectedValue)
    
    // Create component under test
    component := NewComponent(mockDependency)
    
    // Define test cases
    tests := []struct{
        name string
        input string
        expected string
    }{
        // Test cases here
    }
    
    // Run tests
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            result := component.Method(tc.input)
            assert.Equal(t, tc.expected, result)
        })
    }
    
    // Verify expectations
    mockDependency.AssertExpectations(t)
}
```

## Creating Mock Objects

For testing, you can create mock implementations of the interfaces. The package already includes some mock objects for common interfaces:

- `MockMessageHandler` - Mocks the MessageHandler interface
- `MockLogger` - Mocks the Logger interface
- `MockLogEvent` - Mocks the LogEvent interface
- `MockPathSanitizer` - Mocks the PathSanitizer interface

For other interfaces, you can create mock implementations using the testify/mock package:

```go
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
```

## Test Coverage Goals

For each key area, aim for the following test coverage:

1. **Language Detection**: 90%+ coverage, focusing on parsing file names with various patterns
2. **Audio Track Selection**: 85%+ coverage, focusing on track selection logic
3. **Path Construction**: 90%+ coverage, covering sanitization and construction
4. **Error Recovery**: 80%+ coverage, focusing on resumption logic

## Contributing New Tests

When adding new functionality, follow these steps:

1. Define an interface for the new component
2. Create a default implementation
3. Add the interface to the Task struct
4. Initialize the implementation in NewTask
5. Write tests for the new component
6. Document the component in this README