package llms

import (
	"sync"
)

// APIKeyStore provides thread-safe storage and retrieval of API keys
type APIKeyStore struct {
	keys map[string]string
	mu   sync.RWMutex
}

// NewAPIKeyStore creates a new API key store
func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{
		keys: make(map[string]string),
	}
}

// Store adds or replaces an API key for a provider
func (s *APIKeyStore) Store(provider, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[provider] = key
}

// Get retrieves an API key for a provider
func (s *APIKeyStore) Get(provider string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keys[provider]
}

// Has checks if a provider has a non-empty API key
func (s *APIKeyStore) Has(provider string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, exists := s.keys[provider]
	return exists && key != ""
}

// Delete removes an API key for a provider
func (s *APIKeyStore) Delete(provider string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, provider)
}

// List returns a map of all API keys
func (s *APIKeyStore) List() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	result := make(map[string]string, len(s.keys))
	for provider, key := range s.keys {
		result[provider] = key
	}
	
	return result
}

// Global API key store for LLM providers
var APIKeys = NewAPIKeyStore()