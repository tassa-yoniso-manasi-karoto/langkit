package llms

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrProviderNotFound = errors.New("llm provider not found")
	ErrModelNotFound    = errors.New("model not found")
	ErrInvalidRequest   = errors.New("invalid completion request")
)

// Client provides a unified interface to work with various LLM providers
type Client struct {
	providers       map[string]Provider
	defaultProvider string
	mu              sync.RWMutex
}

// NewClient creates a new LLM client
func NewClient() *Client {
	return &Client{
		providers: make(map[string]Provider),
	}
}

// RegisterProvider adds a provider to the client
func (c *Client) RegisterProvider(provider Provider) {
	if provider == nil {
		return
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers[provider.GetName()] = provider
	
	// Set as default if first provider
	if c.defaultProvider == "" {
		c.defaultProvider = provider.GetName()
	}
}

// SetDefaultProvider sets the default provider
func (c *Client) SetDefaultProvider(name string) error {
	c.mu.RLock()
	_, exists := c.providers[name]
	c.mu.RUnlock()
	
	if !exists {
		return ErrProviderNotFound
	}
	
	c.mu.Lock()
	c.defaultProvider = name
	c.mu.Unlock()
	return nil
}

// GetProvider returns a provider by name
func (c *Client) GetProvider(name string) (Provider, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	provider, ok := c.providers[name]
	return provider, ok
}

// GetDefaultProvider returns the default provider
func (c *Client) GetDefaultProvider() (Provider, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.defaultProvider == "" {
		return nil, ErrProviderNotFound
	}
	
	provider, ok := c.providers[c.defaultProvider]
	if !ok {
		return nil, ErrProviderNotFound
	}
	
	return provider, nil
}

// ListProviders returns all registered providers
func (c *Client) ListProviders() []Provider {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	providers := make([]Provider, 0, len(c.providers))
	for _, provider := range c.providers {
		providers = append(providers, provider)
	}
	
	return providers
}

// Complete generates a completion using the specified provider and model
func (c *Client) Complete(ctx context.Context, providerName string, request CompletionRequest) (CompletionResponse, error) {
	var provider Provider
	var err error
	
	// If provider not specified, use default
	if providerName == "" {
		provider, err = c.GetDefaultProvider()
		if err != nil {
			return CompletionResponse{}, err
		}
	} else {
		var ok bool
		provider, ok = c.GetProvider(providerName)
		if !ok {
			return CompletionResponse{}, ErrProviderNotFound
		}
	}
	
	return provider.Complete(ctx, request)
}