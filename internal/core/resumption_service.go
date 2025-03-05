package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// DefaultFileScanner implements the FileScanner interface
type DefaultFileScanner struct {
	handler MessageHandler
}

// NewFileScanner creates a new DefaultFileScanner
func NewFileScanner(handler MessageHandler) FileScanner {
	return &DefaultFileScanner{
		handler: handler,
	}
}

// ScanForContent scans a file for a specific pattern
func (s *DefaultFileScanner) ScanForContent(filePath, pattern string) (bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, so pattern definitely not found
			return false, nil
		}
		return false, fmt.Errorf("error reading file %s: %w", filePath, err)
	}
	
	if len(content) == 0 {
		return false, nil
	}
	
	return strings.Contains(string(content), pattern), nil
}

// GetLastProcessedIndex gets the index of the last processed item based on timestamps in the file
func (s *DefaultFileScanner) GetLastProcessedIndex(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, so no items processed yet
			return 0, nil
		}
		return 0, fmt.Errorf("error opening file %s: %w", filePath, err)
	}
	defer file.Close()
	
	var maxLine int = -1
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			maxLine = lineNum
		}
		lineNum++
	}
	
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error scanning file %s: %w", filePath, err)
	}
	
	return maxLine + 1, nil
}

// DefaultResumptionService implements the ResumptionService interface
type DefaultResumptionService struct {
	fileScanner FileScanner
	separator   string
	handler     MessageHandler
}

// NewResumptionService creates a new DefaultResumptionService
func NewResumptionService(fileScanner FileScanner, separator string, handler MessageHandler) ResumptionService {
	return &DefaultResumptionService{
		fileScanner: fileScanner,
		separator:   separator,
		handler:     handler,
	}
}

// IsAlreadyProcessed checks if an item has already been processed by looking for its identifier in the output file
func (s *DefaultResumptionService) IsAlreadyProcessed(identifier string) (bool, error) {
	// In real implementation, we would use an extended handler with GetOutputFilePath
	// For default implementation, we'll use a fixed path or empty path
	outputFile := ""
	
	// Check if handler implements the extended interface with GetOutputFilePath
	if extHandler, ok := s.handler.(MessageHandlerEx); ok {
		outputFile = extHandler.GetOutputFilePath()
	}
	
	if outputFile == "" {
		return false, nil
	}
	
	return s.fileScanner.ScanForContent(outputFile, identifier)
}

// MarkAsProcessed marks an item as processed (not needed in this implementation as writing to file serves this purpose)
func (s *DefaultResumptionService) MarkAsProcessed(identifier string) error {
	// This is handled implicitly by writing to the output file
	return nil
}

// GetResumePoint finds the point at which to resume processing
func (s *DefaultResumptionService) GetResumePoint(outputFile string) (int, error) {
	if outputFile == "" {
		return 0, nil
	}
	
	return s.fileScanner.GetLastProcessedIndex(outputFile)
}