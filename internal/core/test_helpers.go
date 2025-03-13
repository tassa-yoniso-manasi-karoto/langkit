package core

import (
	"bytes"
	"io"
	
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

// Priority is an alias for zerolog level for cleaner parameter naming
type Priority = int8

// LogLevel represents different log levels
type LogLevel = string

// MockLogger mocks the Logger interface for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug() LogEvent {
	args := m.Called()
	return args.Get(0).(LogEvent)
}

func (m *MockLogger) Info() LogEvent {
	args := m.Called()
	return args.Get(0).(LogEvent)
}

func (m *MockLogger) Warn() LogEvent {
	args := m.Called()
	return args.Get(0).(LogEvent)
}

func (m *MockLogger) Error() LogEvent {
	args := m.Called()
	return args.Get(0).(LogEvent)
}

func (m *MockLogger) Fatal() LogEvent {
	args := m.Called()
	return args.Get(0).(LogEvent)
}

func (m *MockLogger) Trace() LogEvent {
	args := m.Called()
	return args.Get(0).(LogEvent)
}

// MockLogEvent mocks the LogEvent interface for testing
type MockLogEvent struct {
	mock.Mock
}

func (m *MockLogEvent) Err(err error) LogEvent {
	args := m.Called(err)
	return args.Get(0).(LogEvent)
}

func (m *MockLogEvent) Str(key, val string) LogEvent {
	args := m.Called(key, val)
	return args.Get(0).(LogEvent)
}

func (m *MockLogEvent) Int(key string, val int) LogEvent {
	args := m.Called(key, val)
	return args.Get(0).(LogEvent)
}

func (m *MockLogEvent) Bool(key string, val bool) LogEvent {
	args := m.Called(key, val)
	return args.Get(0).(LogEvent)
}

func (m *MockLogEvent) Msg(msg string) {
	m.Called(msg)
}

func (m *MockLogEvent) Msgf(format string, v ...interface{}) {
	m.Called(format, v)
}

// NewMockHandler creates a new mock message handler for tests
func NewMockHandler() *MockHandler {
	mockHandler := new(MockHandler)
	
	// Create an actual zerolog.Logger for the mock to return
	writer := zerolog.ConsoleWriter{Out: io.Discard, NoColor: true}
	logger := zerolog.New(writer).With().Timestamp().Logger()
	
	// Setup default behavior
	mockHandler.On("ZeroLog").Return(&logger)
	mockHandler.On("GetLogBuffer").Return(bytes.Buffer{})
	mockHandler.On("IsCLI").Return(true)
	mockHandler.On("ResetProgress").Return()
	mockHandler.On("IncrementProgress", 
		mock.AnythingOfType("string"),
		mock.AnythingOfType("int"),
		mock.AnythingOfType("int"),
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string")).Return()
	
	return mockHandler
}

// MockHandler is a mock implementation of MessageHandler
type MockHandler struct {
	mock.Mock
}

func (m *MockHandler) IsCLI() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockHandler) Log(level int8, behavior string, msg string) *ProcessingError {
	args := m.Called(level, behavior, msg)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	args := m.Called(err, behavior, msg)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockHandler) LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError {
	args := m.Called(level, err, behavior, msg)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockHandler) LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	args := m.Called(level, behavior, msg, fields)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	args := m.Called(err, behavior, msg, fields)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockHandler) ZeroLog() *zerolog.Logger {
	args := m.Called()
	return args.Get(0).(*zerolog.Logger)
}

func (m *MockHandler) GetLogBuffer() bytes.Buffer {
	args := m.Called()
	return args.Get(0).(bytes.Buffer)
}

func (m *MockHandler) HandleStatus(status string) {
	m.Called(status)
}

func (m *MockHandler) IncrementProgress(taskID string, increment, total, priority int, operation, descr, size string) {
	m.Called(taskID, increment, total, priority, operation, descr, size)
}

func (m *MockHandler) ResetProgress() {
	m.Called()
}

// MockResumptionHandler adjusted to implement MessageHandler 
type MockResumptionHandler struct {
	mock.Mock
}

func (m *MockResumptionHandler) IsCLI() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockResumptionHandler) Log(level int8, behavior string, msg string) *ProcessingError {
	args := m.Called(level, behavior, msg)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockResumptionHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	args := m.Called(err, behavior, msg)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockResumptionHandler) LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError {
	args := m.Called(level, err, behavior, msg)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockResumptionHandler) LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	args := m.Called(level, behavior, msg, fields)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockResumptionHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	args := m.Called(err, behavior, msg, fields)
	if p, ok := args.Get(0).(*ProcessingError); ok {
		return p
	}
	return nil
}

func (m *MockResumptionHandler) ZeroLog() *zerolog.Logger {
	args := m.Called()
	return args.Get(0).(*zerolog.Logger)
}

func (m *MockResumptionHandler) GetLogBuffer() bytes.Buffer {
	args := m.Called()
	return args.Get(0).(bytes.Buffer)
}

func (m *MockResumptionHandler) HandleStatus(status string) {
	m.Called(status)
}

func (m *MockResumptionHandler) IncrementProgress(taskID string, increment, total, priority int, operation, descr, size string) {
	m.Called(taskID, increment, total, priority, operation, descr, size)
}

func (m *MockResumptionHandler) ResetProgress() {
	m.Called()
}

func (m *MockResumptionHandler) GetOutputFilePath() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockResumptionHandler) UpdateProgress(barName string, progress, total int, description string) {
	m.Called(barName, progress, total, description)
}

// MockCrashReporter is a mock implementation of Reporter
type MockCrashReporter struct {
	mock.Mock
}

func (m *MockCrashReporter) Record(update func(*crash.GlobalScope, *crash.ExecutionScope)) {
	m.Called(update)
}

// WithNoExpects returns a mock resumption handler with no expectations set for the common methods
func NewMockResumptionHandlerWithDefaults() *MockResumptionHandler {
    mockHandler := new(MockResumptionHandler)
    
    // Create an actual zerolog.Logger for the mock to return
    writer := zerolog.ConsoleWriter{Out: io.Discard, NoColor: true}
    logger := zerolog.New(writer).With().Timestamp().Logger()
    
    // Setup default behavior
    mockHandler.On("IsCLI").Return(true)
    mockHandler.On("ZeroLog").Return(&logger)
    mockHandler.On("GetLogBuffer").Return(bytes.Buffer{})
    mockHandler.On("ResetProgress").Return()
    mockHandler.On("IncrementProgress", 
        mock.AnythingOfType("string"),
        mock.AnythingOfType("int"),
        mock.AnythingOfType("int"),
        mock.AnythingOfType("int"),
        mock.AnythingOfType("string"),
        mock.AnythingOfType("string"),
        mock.AnythingOfType("string")).Return()
    
    return mockHandler
}