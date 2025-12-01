package voice

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
)

// DockerDemucsProvider implements AudioSeparationProvider using Docker-based Demucs
type DockerDemucsProvider struct {
	useFinetuned bool
	useGPU       bool
	mu           sync.Mutex
	initialized  bool
}

// NewDockerDemucsProvider creates a new DockerDemucsProvider
func NewDockerDemucsProvider(useFinetuned, useGPU bool) *DockerDemucsProvider {
	return &DockerDemucsProvider{
		useFinetuned: useFinetuned,
		useGPU:       useGPU,
	}
}

// GetName returns the provider name
func (p *DockerDemucsProvider) GetName() string {
	base := "docker"
	if p.useGPU {
		base = "docker-nvidia"
	}
	if p.useFinetuned {
		return base + "-demucs_ft"
	}
	return base + "-demucs"
}

// IsAvailable checks if Docker is available for running demucs
func (p *DockerDemucsProvider) IsAvailable() bool {
	return IsDemucsAvailable()
}

// SeparateVoice extracts voice from audio using Docker-based Demucs
func (p *DockerDemucsProvider) SeparateVoice(ctx context.Context, audioFile, outputFormat string, maxTry, timeout int) ([]byte, error) {
	// Determine mode
	mode := DemucsModeCPU
	if p.useGPU {
		mode = DemucsModeGPU
	}

	// Get or create the demucs manager
	manager, err := GetDemucsManager(ctx, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get demucs manager: %w", err)
	}

	// Prepare options
	opts := DefaultDemucsOptions()
	if outputFormat != "" {
		opts.OutputFormat = outputFormat
	}
	if p.useFinetuned {
		opts.Model = "htdemucs_ft"
	}

	// Build a retry policy for the processing
	policy := buildRetryPolicy[[]byte](maxTry)

	// Execute with retry policy
	audioBytes, err := failsafe.Get(func() ([]byte, error) {
		// Create a fresh context for this attempt with timeout
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		return manager.ProcessAudio(attemptCtx, audioFile, opts)
	}, policy)

	if err != nil {
		return nil, fmt.Errorf("docker demucs processing failed after retries: %w", err)
	}

	return audioBytes, nil
}

// Default provider instances for standard use
var (
	// CPU providers
	defaultDockerDemucsProvider         = NewDockerDemucsProvider(false, false)
	defaultDockerDemucsFinetunedProvider = NewDockerDemucsProvider(true, false)
	// GPU providers
	defaultDockerNvidiaDemucsProvider         = NewDockerDemucsProvider(false, true)
	defaultDockerNvidiaDemucsFinetunedProvider = NewDockerDemucsProvider(true, true)
)

// DockerDemucs provides direct access to the Docker-based Demucs voice separation
func DockerDemucs(ctx context.Context, filepath, ext string, maxTry, timeout int, wantFinetuned, wantGPU bool) ([]byte, error) {
	if wantGPU {
		if wantFinetuned {
			return defaultDockerNvidiaDemucsFinetunedProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
		}
		return defaultDockerNvidiaDemucsProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
	}
	if wantFinetuned {
		return defaultDockerDemucsFinetunedProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
	}
	return defaultDockerDemucsProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
}
