package voice

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
)

// DemucsMaxSegmentMinutes is the maximum segment duration for demucs processing.
// Set by config package to avoid import cycle. Initialized based on GPU VRAM if available.
var DemucsMaxSegmentMinutes = CalculateOptimalSegmentMinutes()

// CalculateOptimalSegmentMinutes determines the optimal segment duration based on GPU VRAM.
// Based on empirical testing:
//   - Baseline (model): ~1100 MiB
//   - Per minute of audio: ~210 MiB (includes output tensor allocation spike)
//
// The output phase requires a large tensor allocation after processing completes,
// which can cause OOM even if inference used less memory steadily.
//
// Formula: max_minutes = (available_VRAM - 1100) / 210
// Uses 80% of total VRAM to leave headroom for system usage and allocation spikes.
func CalculateOptimalSegmentMinutes() int {
	vramMiB := executils.GetNvidiaVRAMMiB()
	if vramMiB == 0 {
		// No NVIDIA GPU detected or nvidia-smi not available, use conservative default
		return 15
	}

	// Use 80% of VRAM for safety margin (output allocation can spike)
	availableMiB := float64(vramMiB) * 0.80

	// Baseline model size ~1100 MiB, ~210 MiB per minute (includes output tensor)
	const baselineMiB = 1100
	const mibPerMinute = 210

	if availableMiB <= baselineMiB {
		// Not enough VRAM even for baseline, use minimum
		return 1
	}

	maxMinutes := int((availableMiB - baselineMiB) / mibPerMinute)

	// Clamp to reasonable range (no upper cap - let GPU VRAM be the limit)
	if maxMinutes < 1 {
		return 1
	}

	return maxMinutes
}

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
		// Check for NVIDIA GPU not available error
		errStr := err.Error()
		if strings.Contains(errStr, "nvidia-container-cli") && strings.Contains(errStr, "no adapters were found") {
			hint := "Use the CPU-based 'docker-demucs' provider instead"
			if runtime.GOOS == "linux" {
				hint += ", or install NVIDIA Container Toolkit if you have an NVIDIA GPU"
			}
			return nil, fmt.Errorf("NVIDIA GPU not available: the Docker GPU provider requires an NVIDIA graphics card with proper drivers. %s", hint)
		}
		if strings.Contains(errStr, "nvidia-container-cli") {
			hint := "Make sure your NVIDIA GPU drivers are up to date"
			if runtime.GOOS == "linux" {
				hint = "Make sure NVIDIA Container Toolkit is properly installed and your GPU drivers are up to date"
			}
			return nil, fmt.Errorf("NVIDIA container runtime error: %w. %s", err, hint)
		}
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

	// Use the package-level max segment setting (set by config package)
	if DemucsMaxSegmentMinutes > 0 {
		opts.MaxSegmentMinutes = DemucsMaxSegmentMinutes
	}

	// Extract progress handler from context for cleanup
	var handler ProgressHandler
	if h := ctx.Value(ProgressHandlerKey); h != nil {
		handler, _ = h.(ProgressHandler)
	}

	// Pre-download model weights with progress tracking before starting demucs
	// This provides reliable progress UI; demucs will find files already present
	if err := PreDownloadDemucsModel(ctx, opts.Model, manager.modelsDir, handler); err != nil {
		Logger.Warn().Err(err).Str("model", opts.Model).Msg("Pre-download failed, will fall back to demucs internal download")
		// Don't return error - let demucs try its own download as fallback
	}

	// Create download expectation for cleanup on retry
	expectation := &DownloadExpectation{
		ModelDir:   manager.modelsDir,
		ModelFiles: DemucsModelFiles[opts.Model],
		ProgressBars: []string{
			progress.BarDemucsModelDL,
			progress.BarDemucsProcess,
		},
		Handler: handler,
	}

	// Build retry policy with cleanup
	policy := buildRetryPolicyWithCleanup[[]byte](maxTry, expectation)

	// Execute with retry policy
	audioBytes, err := failsafe.Get(func() ([]byte, error) {
		// Create a fresh context for this attempt with timeout
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		return manager.ProcessAudio(attemptCtx, audioFile, opts)
	}, policy)

	if err != nil {
		// Clean up incomplete model files on cancellation (AbortIf skips OnRetry callback)
		if ctx.Err() != nil || strings.Contains(err.Error(), "context canceled") {
			expectation.Cleanup()
		}
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
