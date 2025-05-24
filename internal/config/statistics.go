package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Statistics holds application usage statistics
type Statistics struct {
	mu sync.RWMutex
	data map[string]interface{}
}

// NewStatistics creates a new Statistics instance
func NewStatistics() *Statistics {
	return &Statistics{
		data: make(map[string]interface{}),
	}
}

// GetStatisticsPath returns the path to the statistics file
func GetStatisticsPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "statistics.json"), nil
}

// LoadStatistics loads statistics from disk
func LoadStatistics() (*Statistics, error) {
	stats := NewStatistics()
	
	path, err := GetStatisticsPath()
	if err != nil {
		return nil, err
	}
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize with default values
			stats.data["countAppStart"] = 0
			stats.data["countProcessStart"] = 0
			return stats, nil
		}
		return nil, err
	}
	
	if err := json.Unmarshal(data, &stats.data); err != nil {
		return nil, err
	}
	
	// Ensure required fields exist
	if _, ok := stats.data["countAppStart"]; !ok {
		stats.data["countAppStart"] = 0
	}
	if _, ok := stats.data["countProcessStart"]; !ok {
		stats.data["countProcessStart"] = 0
	}
	
	return stats, nil
}

// SaveStatistics saves statistics to disk
func (s *Statistics) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	path, err := GetStatisticsPath()
	if err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(path, data, 0644)
}

// Get retrieves a statistic value
func (s *Statistics) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	val, ok := s.data[key]
	return val, ok
}

// Set updates a statistic value
func (s *Statistics) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.data[key] = value
}

// Update allows partial updates to statistics
func (s *Statistics) Update(updates map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for key, value := range updates {
		s.data[key] = value
	}
}

// GetAll returns a copy of all statistics
func (s *Statistics) GetAll() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return a copy to prevent external modifications
	result := make(map[string]interface{})
	for k, v := range s.data {
		result[k] = v
	}
	return result
}

// IncrementCounter increments a counter statistic
func (s *Statistics) IncrementCounter(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	current, ok := s.data[key]
	if !ok {
		s.data[key] = 1
		return 1
	}
	
	// Try to convert to int
	switch v := current.(type) {
	case int:
		s.data[key] = v + 1
		return v + 1
	case float64: // JSON unmarshals numbers as float64
		newVal := int(v) + 1
		s.data[key] = newVal
		return newVal
	default:
		// If it's not a number, reset to 1
		s.data[key] = 1
		return 1
	}
}