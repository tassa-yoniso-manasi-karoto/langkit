package services

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/interfaces"
)

// Compile-time check that ProcessingService implements api.Service
var _ api.Service = (*ProcessingService)(nil)

// ProcessingService implements the WebRPC ProcessingService interface
type ProcessingService struct {
	mu           sync.Mutex
	isProcessing bool
	provider     interfaces.ProcessingProvider
	logger       zerolog.Logger
	handler      http.Handler
	wsServer     interfaces.WebsocketService
}

// NewProcessingService creates a new processing service instance
func NewProcessingService(logger zerolog.Logger, provider interfaces.ProcessingProvider, wsServer interfaces.WebsocketService) *ProcessingService {
	svc := &ProcessingService{
		logger:   logger,
		provider: provider,
		wsServer: wsServer,
	}
	
	// Create the WebRPC handler
	svc.handler = generated.NewProcessingServiceServer(svc)
	
	return svc
}

// Name implements api.Service
func (s *ProcessingService) Name() string {
	return "ProcessingService"
}

// Handler implements api.Service
func (s *ProcessingService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *ProcessingService) Description() string {
	return "Media processing service with single-instance locking"
}

// SendProcessingRequest starts a new processing task
func (s *ProcessingService) SendProcessingRequest(ctx context.Context, request *generated.ProcessRequest) (*generated.ProcessingStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.isProcessing {
		errorMsg := "Processing already in progress"
		return &generated.ProcessingStatus{
			IsProcessing: true,
			Error:        &errorMsg,
		}, nil
	}
	
	s.isProcessing = true
	
	// Emit processing started event
	if s.wsServer != nil {
		s.wsServer.Emit("processing.started", map[string]interface{}{
			"timestamp": time.Now().Unix(),
		})
	}
	
	// Start processing in background goroutine with its own context
	go func() {
		// Track error for completion event
		var processingError error
		
		defer func() {
			s.mu.Lock()
			s.isProcessing = false
			s.mu.Unlock()
			s.logger.Debug().Msg("Processing completed")
			
			// Emit processing completed event
			if s.wsServer != nil {
				eventData := map[string]interface{}{
					"timestamp": time.Now().Unix(),
				}
				
				if processingError != nil {
					eventData["status"] = "error"
					eventData["error"] = processingError.Error()
				} else {
					eventData["status"] = "success"
				}
				
				s.wsServer.Emit("processing.completed", eventData)
			}
		}()
		
		// Create a new context that isn't tied to the HTTP request
		// This prevents automatic cancellation when the HTTP response is sent
		processCtx := context.Background()
		err := s.provider.SendProcessingRequest(processCtx, request)
		if err != nil {
			processingError = err
			if errors.Is(err, context.Canceled) {
				s.logger.Info().Msg("Processing cancelled by user")
			} else {
				s.logger.Error().Err(err).Msg("Processing failed")
			}
		}
	}()
	
	s.logger.Info().Msg("Processing started")
	return &generated.ProcessingStatus{IsProcessing: true}, nil
}

// CancelProcessing cancels the current processing task
func (s *ProcessingService) CancelProcessing(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.isProcessing {
		s.logger.Debug().Msg("CancelProcessing called but no processing in progress")
		return nil
	}
	
	s.provider.CancelProcessing()
	s.logger.Info().Msg("Processing cancellation requested")
	return nil
}

// GetProcessingStatus returns the current processing status
func (s *ProcessingService) GetProcessingStatus(ctx context.Context) (*generated.ProcessingStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return &generated.ProcessingStatus{
		IsProcessing: s.isProcessing,
	}, nil
}