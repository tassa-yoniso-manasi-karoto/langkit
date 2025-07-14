package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// Service represents a WebRPC service that can be registered
type Service interface {
	// Name returns the service name for routing (e.g., "LanguageService")
	Name() string
	
	// Handler returns the HTTP handler for this service
	Handler() http.Handler
	
	// Description returns a human-readable description of the service
	Description() string
}

// ServiceInfo contains metadata about a registered service
type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Registered  bool   `json:"registered"`
}

// Registry manages WebRPC services
type Registry struct {
	services map[string]Service
	mu       sync.RWMutex
	logger   zerolog.Logger
}

// NewRegistry creates a new service registry
func NewRegistry(logger zerolog.Logger) *Registry {
	return &Registry{
		services: make(map[string]Service),
		logger:   logger,
	}
}

// Register adds a service to the registry
func (r *Registry) Register(service Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := service.Name()
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if _, exists := r.services[name]; exists {
		return fmt.Errorf("service %s already registered", name)
	}

	r.services[name] = service
	r.logger.Info().
		Str("service", name).
		Str("description", service.Description()).
		Msg("WebRPC service registered")

	return nil
}

// Unregister removes a service from the registry
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.services[name]; !exists {
		return fmt.Errorf("service %s not found", name)
	}

	delete(r.services, name)
	r.logger.Info().
		Str("service", name).
		Msg("WebRPC service unregistered")

	return nil
}

// Get returns a service by name
func (r *Registry) Get(name string) (Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	service, exists := r.services[name]
	return service, exists
}

// List returns information about all registered services
func (r *Registry) List() []ServiceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]ServiceInfo, 0, len(r.services))
	for name, service := range r.services {
		infos = append(infos, ServiceInfo{
			Name:        name,
			Description: service.Description(),
			Registered:  true,
		})
	}

	return infos
}

// Mount registers all services on a chi router
func (r *Registry) Mount(router chi.Router) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a dispatcher that routes to the correct service based on path
	dispatcher := &serviceDispatcher{
		services: make(map[string]http.Handler),
		logger:   r.logger,
	}
	
	for name, service := range r.services {
		dispatcher.services[name] = service.Handler()
		
		r.logger.Debug().
			Str("service", name).
			Msg("Service registered in dispatcher")
	}
	
	// Mount the dispatcher at root
	router.Handle("/*", dispatcher)
}

// serviceDispatcher routes requests to the appropriate service based on the URL path
type serviceDispatcher struct {
	services map[string]http.Handler
	logger   zerolog.Logger
}

func (d *serviceDispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract service name from path: /rpc/ServiceName/Method
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "rpc" {
		http.NotFound(w, r)
		return
	}
	
	serviceName := parts[1]
	handler, exists := d.services[serviceName]
	if !exists {
		d.logger.Debug().
			Str("service", serviceName).
			Str("path", r.URL.Path).
			Msg("Service not found")
		http.NotFound(w, r)
		return
	}
	
	// Let the service handler process the request
	handler.ServeHTTP(w, r)
}