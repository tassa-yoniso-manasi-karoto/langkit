package voice

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
)

// AudioSeparatorProvider implements AudioSeparationProvider using Docker-based audio-separator
// with MelBand RoFormer model (vocals_mel_band_roformer.ckpt)
type AudioSeparatorProvider struct {
	useGPU      bool
	mu          sync.Mutex
	initialized bool
}

// NewAudioSeparatorProvider creates a new AudioSeparatorProvider
func NewAudioSeparatorProvider(useGPU bool) *AudioSeparatorProvider {
	return &AudioSeparatorProvider{
		useGPU: useGPU,
	}
}

// GetName returns the provider name
func (p *AudioSeparatorProvider) GetName() string {
	if p.useGPU {
		return "docker-nvidia-mel-roformer-kim"
	}
	return "docker-mel-roformer-kim"
}

// IsAvailable checks if Docker is available
func (p *AudioSeparatorProvider) IsAvailable() bool {
	return IsAudioSeparatorAvailable()
}

// SeparateVoice extracts voice from audio using Docker-based audio-separator
func (p *AudioSeparatorProvider) SeparateVoice(ctx context.Context, audioFile, outputFormat string, maxTry, timeout int) ([]byte, error) {
	mode := AudioSepModeCPU
	if p.useGPU {
		mode = AudioSepModeGPU
	}

	// Get or create the manager
	manager, err := GetAudioSeparatorManager(ctx, mode)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "nvidia-container-cli") && strings.Contains(errStr, "no adapters were found") {
			hint := "Use the CPU-based 'docker-mel-roformer-kim' provider instead"
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
		return nil, fmt.Errorf("failed to get audio-separator manager: %w", err)
	}

	// Build retry policy
	policy := buildRetryPolicy[[]byte](maxTry)

	// Execute with retry policy
	audioBytes, err := failsafe.Get(func() ([]byte, error) {
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		return p.processAudio(attemptCtx, manager, audioFile, outputFormat)
	}, policy)

	if err != nil {
		return nil, fmt.Errorf("audio-separator processing failed after retries: %w", err)
	}

	return audioBytes, nil
}

// processAudio runs audio-separator on the input file
func (p *AudioSeparatorProvider) processAudio(ctx context.Context, manager *AudioSeparatorManager, inputPath, outputFormat string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file does not exist: %s", inputPath)
	}

	// Prepare workdir
	workdir := filepath.Join(manager.configDir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workdir: %w", err)
	}

	// Copy input file to workdir
	inputFilename := filepath.Base(inputPath)
	destPath := filepath.Join(workdir, inputFilename)
	if err := copyFileForAudioSep(inputPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy input file: %w", err)
	}
	defer os.Remove(destPath)

	// Determine output format
	outFormat := outputFormat
	if outFormat == "" {
		outFormat = "flac"
	}

	// Build audio-separator command arguments
	// We need to call audio-separator explicitly since the container entrypoint is overridden
	cmdArgs := []string{
		"audio-separator",
		inputFilename,
		"--model_filename", audioSepModelFilename,
		"--model_file_dir", "/models",
		"--output_format", strings.ToUpper(outFormat),
		"--single_stem", "Vocals",
		"--output_dir", "/workdir",
	}

	Logger.Debug().
		Strs("cmd", cmdArgs).
		Str("container", manager.containerName).
		Msg("Executing audio-separator command")

	// Extract progress handler from context
	var progressCb AudioSepProgressCallback
	if h := ctx.Value(ProgressHandlerKey); h != nil {
		if handler, ok := h.(ProgressHandler); ok {
			Logger.Debug().Msg("Progress handler found in context")
			modelDLTaskID := progress.BarAudioSepModelDL
			processTaskID := progress.BarAudioSepProcess
			var lastDownloadPercent int
			var lastProcessPercent int
			var currentPhase AudioSepPhase

			progressCb = func(update AudioSepProgressUpdate) {
				Logger.Trace().
					Int("phase", int(update.Phase)).
					Int("percent", update.Percent).
					Msg("Progress callback received")

				if update.Phase != AudioSepPhaseUnknown && update.Phase != currentPhase {
					if currentPhase == AudioSepPhaseModelDownload && update.Phase == AudioSepPhaseProcessing {
						handler.RemoveProgressBar(modelDLTaskID)
					}
					currentPhase = update.Phase
				}

				switch currentPhase {
				case AudioSepPhaseModelDownload:
					increment := update.Percent - lastDownloadPercent
					if increment > 0 {
						handler.IncrementProgress(modelDLTaskID, increment, 100, 25, "RoFormer Setup", "Downloading model weights...", "")
						lastDownloadPercent = update.Percent
					}
				case AudioSepPhaseProcessing:
					increment := update.Percent - lastProcessPercent
					if increment > 0 {
						handler.IncrementProgress(processTaskID, increment, 100, 30, "Voice Separation", "Processing audio...", "")
						lastProcessPercent = update.Percent
					}
				}
			}
		}
	}

	output, err := manager.execInContainerWithProgress(ctx, cmdArgs, progressCb)

	// Log output to temp file
	logFile := filepath.Join(os.TempDir(), fmt.Sprintf("audio_separator_%d.log", time.Now().Unix()))
	if writeErr := os.WriteFile(logFile, []byte(output), 0644); writeErr == nil {
		Logger.Debug().Str("log_file", logFile).Msg("audio-separator command completed")
	} else {
		Logger.Debug().Msg("audio-separator command completed")
	}

	if err != nil {
		if strings.Contains(output, "CUDA out of memory") {
			return nil, ErrCUDAOutOfMemory
		}
		if strings.Contains(output, "Failed to download file") ||
			strings.Contains(output, "ConnectionError") ||
			strings.Contains(output, "NewConnectionError") {
			return nil, ErrModelDownloadFailed
		}
		return nil, fmt.Errorf("audio-separator execution failed: %w\nOutput: %s", err, output)
	}
	if strings.Contains(output, "CUDA out of memory") {
		return nil, ErrCUDAOutOfMemory
	}

	// Find the output file
	// audio-separator outputs: <inputname>_(vocals)_<model>.flac
	trackName := inputFilename[:len(inputFilename)-len(filepath.Ext(inputFilename))]

	// Search for the vocals output file (lowercase 'vocals' in output)
	pattern := filepath.Join(workdir, trackName+"*(vocals)*"+"."+outFormat)
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		// Try alternative pattern without format extension
		pattern = filepath.Join(workdir, trackName+"*(vocals)*")
		matches, _ = filepath.Glob(pattern)
	}

	if len(matches) == 0 {
		// List workdir contents for debugging
		entries, _ := os.ReadDir(workdir)
		var files []string
		for _, e := range entries {
			files = append(files, e.Name())
		}
		Logger.Error().
			Str("pattern", pattern).
			Strs("workdir_contents", files).
			Msg("Could not find vocals output file")
		return nil, fmt.Errorf("vocals output file not found, workdir contains: %v", files)
	}

	vocalsPath := matches[0]
	Logger.Debug().Str("vocals_path", vocalsPath).Msg("Found vocals output file")

	audioData, err := os.ReadFile(vocalsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read output file %s: %w", vocalsPath, err)
	}

	// Clean up output file
	os.Remove(vocalsPath)

	return audioData, nil
}

// copyFileForAudioSep copies a file from src to dst
func copyFileForAudioSep(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// Default provider instances
var (
	defaultAudioSepCPUProvider = NewAudioSeparatorProvider(false)
	defaultAudioSepGPUProvider = NewAudioSeparatorProvider(true)
)

// AudioSeparator provides direct access to the Docker-based audio-separator voice separation
func AudioSeparator(ctx context.Context, filepath, ext string, maxTry, timeout int, wantGPU bool) ([]byte, error) {
	if wantGPU {
		return defaultAudioSepGPUProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
	}
	return defaultAudioSepCPUProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
}
