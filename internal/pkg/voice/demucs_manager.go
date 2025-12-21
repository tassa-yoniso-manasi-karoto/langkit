package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	composetypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
)

const (
	demucsProjectName = "langkit-demucs" // Base project name for config dir
	demucsImageName   = "ghcr.io/tassa-yoniso-manasi-karoto/langkit-demucs:latest"
)

// DemucsMode specifies CPU or GPU execution
type DemucsMode int

const (
	DemucsModeCPU DemucsMode = iota
	DemucsModeGPU
)

func (m DemucsMode) projectName() string {
	if m == DemucsModeGPU {
		return "langkit-demucs-gpu"
	}
	return "langkit-demucs"
}

func (m DemucsMode) containerName() string {
	// Docker Compose generates: {project}-{service}-1
	if m == DemucsModeGPU {
		return "langkit-demucs-gpu-demucs-1"
	}
	return "langkit-demucs-demucs-1"
}

// buildComposeProject creates the compose project definition for demucs
func (m DemucsMode) buildComposeProject(configDir string) *composetypes.Project {
	// Network name follows Docker Compose convention: {project}_{network}
	defaultNetworkName := m.projectName() + "_default"

	service := composetypes.ServiceConfig{
		Name:          "demucs",
		ContainerName: m.containerName(), // Explicit container name for exec commands
		Image:         demucsImageName,
		StdinOpen:     true,
		Tty:           true,
		WorkingDir:    "/workspace",
		Volumes: []composetypes.ServiceVolumeConfig{
			{
				Type:   composetypes.VolumeTypeBind,
				Source: filepath.Join(configDir, "input"),
				Target: "/data/input",
			},
			{
				Type:   composetypes.VolumeTypeBind,
				Source: filepath.Join(configDir, "output"),
				Target: "/data/output",
			},
			{
				Type:   composetypes.VolumeTypeBind,
				Source: filepath.Join(configDir, "models"),
				Target: "/root/.demucs/models", // demucs-next ignores TORCH_HOME, uses ~/.demucs/models
			},
		},
		// Attach to default network
		Networks: map[string]*composetypes.ServiceNetworkConfig{
			"default": nil,
		},
	}

	// Add GPU device reservation for GPU mode
	if m == DemucsModeGPU {
		service.Deploy = &composetypes.DeployConfig{
			Resources: composetypes.Resources{
				Reservations: &composetypes.Resource{
					Devices: []composetypes.DeviceRequest{{
						Capabilities: []string{"gpu"},
						Driver:       "nvidia",
						Count:        -1, // all GPUs
					}},
				},
			},
		}
	}

	return &composetypes.Project{
		Name: m.projectName(),
		// Default network required for container networking
		Networks: composetypes.Networks{
			"default": composetypes.NetworkConfig{
				Name: defaultNetworkName,
			},
		},
		Services: composetypes.Services{
			"demucs": service,
		},
	}
}

// ProgressHandlerKey is the context key for passing progress handler
type progressHandlerKeyType string
const ProgressHandlerKey progressHandlerKeyType = "voice.progressHandler"

// DockerRecreateKey is the context key for passing docker recreate flag
type dockerRecreateKeyType string
const DockerRecreateKey dockerRecreateKeyType = "voice.dockerRecreate"

// ProgressHandler is called to report progress updates
type ProgressHandler interface {
	// IncrementDownloadProgress is for file/image downloads - displays humanized bytes
	IncrementDownloadProgress(taskID string, increment, total, priority int, operation, descr, heightClass, humanizedSize string)
	// IncrementProgress is for processing tasks - displays percentage
	IncrementProgress(taskID string, increment, total, priority int, operation, descr, size string)
	RemoveProgressBar(taskID string)
	ZeroLog() *zerolog.Logger
}

var (
	// Singleton instance management - separate instances for CPU and GPU
	demucsCPUInstance *DemucsManager
	demucsGPUInstance *DemucsManager
	demucsMu          sync.Mutex
	demucsLastUsed    time.Time
	demucsIdleTimeout = 30 * time.Minute
	demucsWatcherOnce sync.Once

	// DemucsLogger is the logger for demucs operations
	DemucsLogger = zerolog.Nop()
)

// DemucsOptions holds configuration for demucs processing
type DemucsOptions struct {
	Model              string  // htdemucs, htdemucs_ft, etc. (default: htdemucs)
	OutputFormat       string  // wav, mp3, flac (default: wav)
	Stems              string  // vocals, drums, bass, other (default: vocals)
	Shifts             int     // shift trick for better quality (default: 1)
	Overlap            float64 // overlap between prediction windows (default: 0.25)
	MaxSegmentMinutes  int     // max minutes per segment to avoid GPU OOM (default: 20, 0=no limit)
}

// DefaultDemucsOptions returns default options for demucs
func DefaultDemucsOptions() DemucsOptions {
	return DemucsOptions{
		Model:             "htdemucs",
		OutputFormat:      "flac", // flac/opus keep timing sync, mp3/wav can cause A/V desync
		Stems:             "vocals",
		Shifts:            1,
		Overlap:           0.25,
		MaxSegmentMinutes: 20, // 20 min segments ~2GB output tensor, safe for most GPUs
	}
}

// getAudioDurationSeconds returns the duration of an audio file in seconds using ffmpeg
func getAudioDurationSeconds(filePath string) (float64, error) {
	// Use ffmpeg -i to get file info (duration is in stderr)
	cmd := exec.Command(media.FFmpegPath, "-i", filePath, "-hide_banner", "-f", "null", "-")
	output, _ := cmd.CombinedOutput() // ffmpeg returns error for -f null, ignore it

	// Parse "Duration: HH:MM:SS.ms" from output
	outputStr := string(output)
	durationIdx := strings.Index(outputStr, "Duration: ")
	if durationIdx == -1 {
		return 0, fmt.Errorf("could not find duration in ffmpeg output")
	}

	// Extract duration string (format: "HH:MM:SS.ms")
	durationStart := durationIdx + len("Duration: ")
	commaIdx := strings.Index(outputStr[durationStart:], ",")
	if commaIdx == -1 {
		return 0, fmt.Errorf("could not parse duration format")
	}
	durationStr := outputStr[durationStart : durationStart+commaIdx]

	// Parse HH:MM:SS.ms format
	parts := strings.Split(durationStr, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("unexpected duration format: %s", durationStr)
	}

	hours, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse hours: %w", err)
	}
	minutes, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse minutes: %w", err)
	}
	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse seconds: %w", err)
	}

	return hours*3600 + minutes*60 + seconds, nil
}

// splitAudioFile splits an audio file into segments of specified duration
// Returns paths to the segment files
func splitAudioFile(inputPath string, segmentSeconds int, outputDir string) ([]string, error) {
	ext := filepath.Ext(inputPath)

	// Use simple prefix to avoid glob issues with special chars in original filename
	segmentPattern := filepath.Join(outputDir, "seg_%03d"+ext)

	args := []string{
		"-y", "-loglevel", "error",
		"-i", inputPath,
		"-f", "segment",
		"-segment_time", strconv.Itoa(segmentSeconds),
		"-c", "copy", // Copy codec, no re-encoding
		"-reset_timestamps", "1",
		segmentPattern,
	}

	cmd := exec.Command(media.FFmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to split audio: %w, output: %s", err, string(output))
	}

	// Find all segment files
	pattern := filepath.Join(outputDir, "seg_*"+ext)
	segments, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find segments: %w", err)
	}

	if len(segments) == 0 {
		return nil, fmt.Errorf("no segments created")
	}

	// Sort segments to ensure correct order
	sort.Strings(segments)
	return segments, nil
}

// encodeAudio encodes an audio file to the specified format using ffmpeg
func encodeAudio(inputPath, outputPath, format string) error {
	var codecArgs []string
	switch format {
	case "flac":
		codecArgs = []string{"-c:a", "flac"}
	case "wav":
		codecArgs = []string{"-c:a", "pcm_s24le"}
	case "mp3":
		codecArgs = []string{"-c:a", "libmp3lame", "-q:a", "2"}
	case "opus":
		codecArgs = []string{"-c:a", "libopus", "-b:a", "128k"}
	default:
		codecArgs = []string{"-c:a", "copy"}
	}

	args := []string{"-y", "-loglevel", "error", "-i", inputPath}
	args = append(args, codecArgs...)
	args = append(args, outputPath)

	cmd := exec.Command(media.FFmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg encoding failed: %w, output: %s", err, string(output))
	}
	return nil
}

// concatenateAudioFiles joins multiple audio files into one using ffmpeg concat demuxer
// Outputs PCM/WAV to avoid timestamp issues from segmented files
func concatenateAudioFiles(inputFiles []string, outputPath string) error {
	if len(inputFiles) == 0 {
		return fmt.Errorf("no input files to concatenate")
	}
	if len(inputFiles) == 1 {
		// Just copy the single file
		return copyFile(inputFiles[0], outputPath)
	}

	// Create a temporary concat list file
	listFile := outputPath + ".concat.txt"
	defer os.Remove(listFile)

	var content strings.Builder
	for _, f := range inputFiles {
		// FFmpeg concat requires escaped paths
		escaped := strings.ReplaceAll(f, "'", "'\\''")
		content.WriteString(fmt.Sprintf("file '%s'\n", escaped))
	}

	if err := os.WriteFile(listFile, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to create concat list: %w", err)
	}

	// Decode to PCM and output as WAV (avoids wasteful re-encoding)
	args := []string{
		"-y", "-loglevel", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-c:a", "pcm_s24le",
		outputPath,
	}

	cmd := exec.Command(media.FFmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to concatenate audio: %w, output: %s", err, string(output))
	}

	return nil
}

// DemucsManager handles Docker lifecycle for the Demucs project
type DemucsManager struct {
	docker        *dockerutil.DockerManager
	logger        *dockerutil.ContainerLogConsumer
	mode          DemucsMode
	projectName   string
	containerName string
	configDir     string
}

// NewDemucsManager creates a new Demucs manager instance with specified mode
func NewDemucsManager(ctx context.Context, mode DemucsMode) (*DemucsManager, error) {
	// Enable docker log output for debugging
	// dockerutil.SetLogOutput(dockerutil.LogToStdout)

	manager := &DemucsManager{
		mode:          mode,
		projectName:   mode.projectName(),
		containerName: mode.containerName(),
	}

	// Get the config directory for volume paths (use mode-specific project name)
	configDir, err := dockerutil.GetConfigDir(manager.projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	// Ensure volume directories exist
	for _, subdir := range []string{"input", "output", "models"} {
		if err := os.MkdirAll(filepath.Join(configDir, subdir), 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s directory: %w", subdir, err)
		}
	}

	// Build compose project
	project := mode.buildComposeProject(configDir)

	logConfig := dockerutil.LogConfig{
		Prefix:      manager.projectName,
		ShowService: true,
		ShowType:    true,
		LogLevel:    zerolog.DebugLevel,
		InitMessage: "langkit-demucs", // Fixed string matching Dockerfile echo
	}

	logger := dockerutil.NewContainerLogConsumer(logConfig)

	cfg := dockerutil.Config{
		ProjectName:      manager.projectName,
		Project:          project,
		RequiredServices: []string{"demucs"},
		LogConsumer:      logger,
		Timeout: dockerutil.Timeout{
			Create:   5 * time.Minute,  // container creation (image pull is separate with no timeout)
			Recreate: 5 * time.Minute,  // container recreation
			Start:    2 * time.Minute,  // reach running state
		},
	}

	dockerManager, err := dockerutil.NewDockerManager(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker manager: %w", err)
	}

	manager.docker = dockerManager
	manager.logger = logger
	manager.configDir = configDir

	return manager, nil
}

// Init initializes the docker service
func (dm *DemucsManager) Init(ctx context.Context) error {
	return dm.docker.Init()
}

// InitQuiet initializes the docker service with reduced logging
func (dm *DemucsManager) InitQuiet(ctx context.Context) error {
	return dm.docker.InitQuiet()
}

// InitRecreate forces recreation of the docker container
func (dm *DemucsManager) InitRecreate(ctx context.Context) error {
	return dm.docker.InitRecreate()
}

// waitForExecReady waits for the container to be ready to accept exec commands
func (dm *DemucsManager) waitForExecReady(ctx context.Context) error {
	// GPU mode needs more time for CUDA initialization
	maxRetries := 10
	retryDelay := 200 * time.Millisecond
	if dm.mode == DemucsModeGPU {
		maxRetries = 30
		retryDelay = 500 * time.Millisecond
	}

	for i := 0; i < maxRetries; i++ {
		// Try a simple exec command to verify container is ready
		_, err := dm.execInContainer(ctx, []string{"true"})
		if err == nil {
			return nil
		}
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("container not ready after %d attempts", maxRetries)
}

// removeStaleContainer removes any existing container with the same name
func (dm *DemucsManager) removeStaleContainer(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil // Ignore errors, this is best-effort cleanup
	}
	defer cli.Close()

	// First try to stop the container (might be running)
	stopTimeout := 5
	_ = cli.ContainerStop(ctx, dm.containerName, container.StopOptions{Timeout: &stopTimeout})

	// Brief pause to ensure container is fully stopped
	time.Sleep(100 * time.Millisecond)

	// Remove the container with force but preserve volumes (model weights)
	err = cli.ContainerRemove(ctx, dm.containerName, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: false,
	})
	if err != nil {
		// Only ignore "not found" errors
		if !strings.Contains(err.Error(), "No such container") &&
			!strings.Contains(err.Error(), "not found") {
			DemucsLogger.Warn().Err(err).Str("container", dm.containerName).Msg("Failed to remove stale container")
		}
		return nil
	}

	// Brief pause to ensure Docker has processed the removal
	time.Sleep(200 * time.Millisecond)

	DemucsLogger.Debug().Str("container", dm.containerName).Msg("Removed stale container")
	return nil
}

// removeContainerByName is a helper to remove a container by name (best-effort)
func removeContainerByName(ctx context.Context, name string) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()

	stopTimeout := 3
	_ = cli.ContainerStop(ctx, name, container.StopOptions{Timeout: &stopTimeout})
	_ = cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true, RemoveVolumes: false})
}

// Stop stops the docker service
func (dm *DemucsManager) Stop(ctx context.Context) error {
	return dm.docker.Stop()
}

// Close implements io.Closer
func (dm *DemucsManager) Close() error {
	dm.logger.Close()
	return dm.docker.Close()
}

// Status returns the current status of the project
func (dm *DemucsManager) Status(ctx context.Context) (string, error) {
	return dm.docker.Status()
}

// GetContainerName returns the name of the main container
func (dm *DemucsManager) GetContainerName() string {
	return dm.containerName
}

// ProcessAudio runs demucs on the input audio file and returns the vocals track.
// If the audio is longer than MaxSegmentMinutes, it will be split into segments,
// processed separately, and concatenated to avoid GPU OOM errors.
func (dm *DemucsManager) ProcessAudio(ctx context.Context, inputPath string, opts DemucsOptions) ([]byte, error) {
	// Check for cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Ensure input file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file does not exist: %s", inputPath)
	}

	// Prepare directories
	inputDir := filepath.Join(dm.configDir, "input")
	outputDir := filepath.Join(dm.configDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create input directory: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if we need to split the audio
	maxSegmentMinutes := opts.MaxSegmentMinutes
	if maxSegmentMinutes <= 0 {
		maxSegmentMinutes = 20 // Default to 20 minutes if not set
	}
	maxSegmentSeconds := maxSegmentMinutes * 60

	duration, err := getAudioDurationSeconds(inputPath)
	if err != nil {
		DemucsLogger.Warn().Err(err).Msg("Could not determine audio duration, processing without splitting")
		return dm.processSingleFile(ctx, inputPath, inputDir, outputDir, opts, nil)
	}

	DemucsLogger.Debug().
		Float64("duration_seconds", duration).
		Int("max_segment_seconds", maxSegmentSeconds).
		Msg("Audio duration check")

	// If audio is short enough, process directly
	if duration <= float64(maxSegmentSeconds) {
		return dm.processSingleFile(ctx, inputPath, inputDir, outputDir, opts, nil)
	}

	// Audio is too long - split into segments
	numSegments := int(duration/float64(maxSegmentSeconds)) + 1
	DemucsLogger.Info().
		Float64("duration_minutes", duration/60).
		Int("max_segment_minutes", maxSegmentMinutes).
		Int("num_segments", numSegments).
		Msg("Audio exceeds max segment duration, splitting for processing")

	// Create temp directory for segments
	segmentDir := filepath.Join(inputDir, fmt.Sprintf("segments_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(segmentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create segment directory: %w", err)
	}
	defer os.RemoveAll(segmentDir) // Clean up segments after processing

	// Check for cancellation before splitting
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Split the audio
	segments, err := splitAudioFile(inputPath, maxSegmentSeconds, segmentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to split audio: %w", err)
	}

	DemucsLogger.Debug().
		Int("num_segments", len(segments)).
		Strs("segments", segments).
		Msg("Audio split into segments")

	// For multi-segment processing, use WAV for intermediate files (faster, no encoding)
	// then encode to requested format at the end
	segmentOpts := opts
	segmentOpts.OutputFormat = "wav"

	// Process each segment and collect output paths
	var outputPaths []string
	totalSegments := len(segments)
	for i, segment := range segments {
		// Check for cancellation before each segment
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		DemucsLogger.Info().
			Int("segment", i+1).
			Int("total", totalSegments).
			Str("file", filepath.Base(segment)).
			Msg("Processing segment")

		// Process this segment with segment info for progress scaling
		segInfo := &segmentInfo{index: i, total: totalSegments}
		audioData, err := dm.processSingleFile(ctx, segment, inputDir, outputDir, segmentOpts, segInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to process segment %d: %w", i+1, err)
		}

		// Write segment output to temp file for concatenation
		segmentOutput := filepath.Join(segmentDir, fmt.Sprintf("output_%03d.wav", i))
		if err := os.WriteFile(segmentOutput, audioData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write segment output: %w", err)
		}
		outputPaths = append(outputPaths, segmentOutput)
	}

	// Concatenate all WAV outputs
	concatenatedWav := filepath.Join(segmentDir, "concatenated.wav")
	if err := concatenateAudioFiles(outputPaths, concatenatedWav); err != nil {
		return nil, fmt.Errorf("failed to concatenate segments: %w", err)
	}

	// Encode to final format (default: flac)
	finalFormat := opts.OutputFormat
	if finalFormat == "" {
		finalFormat = "flac"
	}
	finalOutput := filepath.Join(segmentDir, "final."+finalFormat)
	if err := encodeAudio(concatenatedWav, finalOutput, finalFormat); err != nil {
		return nil, fmt.Errorf("failed to encode final output to %s: %w", finalFormat, err)
	}

	// Read the final encoded output
	result, err := os.ReadFile(finalOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to read final output: %w", err)
	}

	DemucsLogger.Info().
		Int("segments_processed", len(segments)).
		Int("output_size", len(result)).
		Msg("Successfully processed and concatenated all segments")

	return result, nil
}

// processSingleFile processes a single audio file through demucs (no splitting)
// If segInfo is provided, progress is scaled to reflect position within multi-segment processing
func (dm *DemucsManager) processSingleFile(ctx context.Context, inputPath, inputDir, outputDir string, opts DemucsOptions, segInfo *segmentInfo) ([]byte, error) {
	// Copy input file to the input directory
	inputFilename := filepath.Base(inputPath)
	destPath := filepath.Join(inputDir, inputFilename)
	if err := copyFile(inputPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy input file: %w", err)
	}
	defer os.Remove(destPath) // Clean up input file after processing

	// Build the demucs command
	model := opts.Model
	if model == "" {
		model = "htdemucs"
	}
	outputFormat := opts.OutputFormat
	if outputFormat == "" {
		outputFormat = "wav"
	}
	stems := opts.Stems
	if stems == "" {
		stems = "vocals"
	}

	// Build command arguments for demucs-inference CLI
	// Output template: /data/output/{model}/{track}/{stem}.{ext}
	cmdArgs := []string{
		"demucs", "separate",
		"-m", model,
		"-o", "/data/output/{model}/{track}/{stem}.{ext}",
		"-f", outputFormat,
		"--isolate-stem", stems,
	}

	// Add shifts if not default
	if opts.Shifts > 1 {
		cmdArgs = append(cmdArgs, "--shifts", fmt.Sprintf("%d", opts.Shifts))
	}

	// Add overlap if not default
	if opts.Overlap != 0.25 && opts.Overlap > 0 {
		cmdArgs = append(cmdArgs, "--split-overlap", fmt.Sprintf("%.2f", opts.Overlap))
	}

	// Add the input file path (inside container)
	cmdArgs = append(cmdArgs, "/data/input/"+inputFilename)

	// Execute command in container
	DemucsLogger.Debug().
		Str("cmd", strings.Join(cmdArgs, "")).
		Str("container", dm.containerName).
		Msg("Executing demucs command")

	// Extract progress handler from context if available
	var progressCb ProgressCallback
	if h := ctx.Value(ProgressHandlerKey); h != nil {
		if handler, ok := h.(ProgressHandler); ok {
			DemucsLogger.Debug().Msg("Progress handler found in context")
			downloadTaskID := progress.BarDemucsModelDL
			processTaskID := progress.BarDemucsProcess
			var lastDownloadPercent int
			var currentPhase DemucsPhase

			// Initialize lastOverallPercent to base progress from completed segments
			// This prevents progress jumping when a new segment starts
			var lastOverallPercent int
			if segInfo != nil && segInfo.total > 1 {
				// Multiply before divide to avoid truncation when total > 100
				lastOverallPercent = (segInfo.index * 100) / segInfo.total
			}

			progressCb = func(update ProgressUpdate) {
				DemucsLogger.Trace().
					Int("phase", int(update.Phase)).
					Int("percent", update.Percent).
					Int("currentPhase", int(currentPhase)).
					Msg("Progress callback received")
				// Handle phase transitions
				if update.Phase != PhaseUnknown && update.Phase != currentPhase {
					// Phase changed
					if currentPhase == PhaseModelDownload && update.Phase == PhaseProcessing {
						// Finished download, remove download progress bar
						handler.RemoveProgressBar(downloadTaskID)
					}
					currentPhase = update.Phase
				}

				switch currentPhase {
				case PhaseModelDownload:
					increment := update.Percent - lastDownloadPercent
					if increment > 0 {
						handler.IncrementProgress(downloadTaskID, increment, 100, 25, "Demucs Setup", "Downloading model weights...", "")
						lastDownloadPercent = update.Percent
					}
				case PhaseProcessing:
					// Calculate overall progress considering segment position
					var overallPercent int
					var description string
					if segInfo != nil && segInfo.total > 1 {
						// Multi-segment: scale progress across all segments
						// Formula: (index * 100 + segmentPercent) / total
						// Multiply before divide to avoid truncation when total > 100
						overallPercent = (segInfo.index*100 + update.Percent) / segInfo.total
						description = fmt.Sprintf("Processing segment %d/%d...", segInfo.index+1, segInfo.total)

						// Ensure last segment at 100% completes the overall progress
						if segInfo.index == segInfo.total-1 && update.Percent == 100 {
							overallPercent = 100
						}
					} else {
						// Single file: use percent directly
						overallPercent = update.Percent
						description = "Processing audio..."
					}

					increment := overallPercent - lastOverallPercent
					if increment > 0 {
						handler.IncrementProgress(processTaskID, increment, 100, 30, "Voice Separation", description, "")
						lastOverallPercent = overallPercent
					}
				}
			}
		}
	}

	output, err := dm.execInContainerWithProgress(ctx, cmdArgs, progressCb)
	if err != nil {
		// Check for CUDA OOM error and provide helpful message
		if strings.Contains(output, "CUDA out of memory") {
			return nil, fmt.Errorf("GPU out of memory during voice separation. Try lowering the 'Max segment duration' setting in Settings → Voice Separation. Current audio may be too long for your GPU's VRAM")
		}
		return nil, fmt.Errorf("demucs execution failed: %w\nOutput: %s", err, output)
	}
	// Also check output for OOM even if no error (demucs may report success despite failure)
	if strings.Contains(output, "CUDA out of memory") {
		return nil, fmt.Errorf("GPU out of memory during voice separation. Try lowering the 'Max segment duration' setting in Settings → Voice Separation")
	}

	// Write demucs output to temp log file instead of logger (can be very large)
	logFile := filepath.Join(os.TempDir(), fmt.Sprintf("demucs_%d.log", time.Now().Unix()))
	if err := os.WriteFile(logFile, []byte(output), 0644); err == nil {
		DemucsLogger.Debug().Str("log_file", logFile).Msg("Demucs command completed")
	} else {
		DemucsLogger.Debug().Msg("Demucs command completed")
	}

	// Find the output file
	// demucs-inference outputs to: /data/output/<model>/<track_name>/<stem>.<ext>
	trackName := inputFilename[:len(inputFilename)-len(filepath.Ext(inputFilename))]

	// The vocals file will be at: output/<model>/<trackname>/vocals.<ext>
	vocalsPath := filepath.Join(outputDir, model, trackName, stems+"."+outputFormat)

	DemucsLogger.Debug().Str("vocals_path", vocalsPath).Msg("Looking for output file")

	// Read the output file
	audioData, err := os.ReadFile(vocalsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read output file %s: %w", vocalsPath, err)
	}

	// Clean up output directory for this track
	trackOutputDir := filepath.Join(outputDir, model, trackName)
	os.RemoveAll(trackOutputDir)

	return audioData, nil
}

// segmentInfo tracks the current segment being processed for progress calculation
type segmentInfo struct {
	index int // 0-based index of current segment
	total int // total number of segments
}

// DemucsPhase represents the current phase of demucs execution
type DemucsPhase int

const (
	PhaseUnknown DemucsPhase = iota
	PhaseModelDownload
	PhaseProcessing
)

// ProgressUpdate contains progress info with phase context
type ProgressUpdate struct {
	Phase   DemucsPhase
	Percent int
}

// ProgressCallback is called with progress updates including phase
type ProgressCallback func(update ProgressUpdate)

// execInContainer executes a command in the demucs container
func (dm *DemucsManager) execInContainer(ctx context.Context, cmd []string) (string, error) {
	return dm.execInContainerWithProgress(ctx, cmd, nil)
}

// execInContainerWithProgress executes a command with optional progress callback
// Uses TTY mode to get real-time Rich progress bar output
func (dm *DemucsManager) execInContainerWithProgress(ctx context.Context, cmd []string, progressCb ProgressCallback) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Create exec configuration - use TTY for Rich progress output
	// Set COLUMNS to force Rich library to output full progress bar with percentages
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Env:          []string{"COLUMNS=200", "TERM=xterm-256color"},
	}

	// Create the exec instance
	execID, err := cli.ContainerExecCreate(ctx, dm.containerName, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to the exec instance with console size to ensure Rich outputs percentages
	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{
		Tty:         true,
		ConsoleSize: &[2]uint{50, 200}, // height, width - wide enough for Rich progress
	})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read output - with TTY, stdout/stderr are combined
	var output bytes.Buffer
	buf := make([]byte, 4096)
	var lastPercent int = -1
	var currentPhase DemucsPhase = PhaseUnknown

	for {
		n, readErr := resp.Reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			output.Write(chunk)

			// Parse progress from Rich TTY output
			if progressCb != nil {
				phase, pct := parseDemucsProgress(chunk, currentPhase)
				if pct >= 0 {
					DemucsLogger.Trace().
						Int("parsed_phase", int(phase)).
						Int("parsed_pct", pct).
						Int("current_phase", int(currentPhase)).
						Int("last_pct", lastPercent).
						Msg("Parsed progress from output")
				}
				if phase != PhaseUnknown {
					currentPhase = phase
				}
				// Report progress when percentage changes or phase changes
				if pct >= 0 && (pct != lastPercent || phase != PhaseUnknown) {
					progressCb(ProgressUpdate{Phase: currentPhase, Percent: pct})
					lastPercent = pct
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	// Check exec exit code
	inspectResp, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return output.String(), fmt.Errorf("failed to inspect exec: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return output.String(), fmt.Errorf("command exited with code %d: %s", inspectResp.ExitCode, output.String())
	}

	return output.String(), nil
}

// parseDemucsProgress extracts phase and percentage from Rich TTY output
// Returns detected phase (or PhaseUnknown if no phase indicator) and percentage (-1 if none)
func parseDemucsProgress(data []byte, currentPhase DemucsPhase) (DemucsPhase, int) {
	str := string(data)
	detectedPhase := PhaseUnknown

	// Detect phase from content
	// "Downloading" indicates model download phase
	if bytes.Contains(data, []byte("Downloading")) {
		detectedPhase = PhaseModelDownload
	}

	// Find percentage - look for pattern \d+%
	lastPercent := -1
	for i := 0; i < len(str)-1; i++ {
		if str[i] >= '0' && str[i] <= '9' {
			j := i
			for j < len(str) && str[j] >= '0' && str[j] <= '9' {
				j++
			}
			if j < len(str) && str[j] == '%' {
				numStr := str[i:j]
				var pct int
				if _, err := fmt.Sscanf(numStr, "%d", &pct); err == nil {
					if pct >= 0 && pct <= 100 {
						lastPercent = pct
					}
				}
				i = j
			}
		}
	}

	// If we found a percentage and we're not downloading, we must be processing
	// This handles the case where model is cached and we skip the download phase
	if lastPercent >= 0 && detectedPhase != PhaseModelDownload {
		detectedPhase = PhaseProcessing
	}

	return detectedPhase, lastPercent
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
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

// pullImageWithProgress pulls the demucs Docker image with progress reporting and retry
func pullImageWithProgress(ctx context.Context, handler ProgressHandler) error {
	if handler != nil {
		handler.ZeroLog().Info().
			Str("image", demucsImageName).
			Msg("Pulling Docker image for local voice separation (first-time setup, ~7GB download)...")
	}

	opts := dockerutil.DefaultPullOptions()
	taskID := progress.BarDemucsDockerDL

	// Progress callback for UI
	if handler != nil {
		var lastBytes int64

		opts.OnProgress = func(current, total int64, status string) {
			// PullImage reports cumulative progress including baseline from cached layers.
			// On retry, 'current' may initially be lower than 'lastBytes' if some
			// in-progress bytes weren't persisted. Clamp to prevent backward progress.
			increment := current - lastBytes
			if increment > 0 {
				handler.IncrementDownloadProgress(
					taskID,
					int(increment),
					int(total),
					20,
					"Demucs Setup",
					status,
					"", // Use importance map for height class
					humanize.Bytes(uint64(current))+" / "+humanize.Bytes(uint64(total)),
				)
				lastBytes = current
			}
		}
		opts.OnRetry = func(err error, nextRetryIn time.Duration) {
			handler.RemoveProgressBar(taskID)
			// Don't reset lastBytes - PullImage tracks cumulative progress internally.
			// The next OnProgress call may have a lower 'current' initially, but the
			// increment clamp above handles this gracefully.
			handler.ZeroLog().Warn().
				Err(err).
				Dur("retry_in", nextRetryIn).
				Msg("Docker pull failed, retrying...")
		}
	}

	err := dockerutil.PullImage(ctx, demucsImageName, opts)
	if handler != nil {
		handler.RemoveProgressBar(taskID)
		if err == nil {
			handler.ZeroLog().Info().Msg("Docker image pull complete")
		}
	}
	return err
}

// GetDemucsManager returns or creates the singleton manager for the specified mode
func GetDemucsManager(ctx context.Context, mode DemucsMode) (*DemucsManager, error) {
	demucsMu.Lock()
	defer demucsMu.Unlock()

	// Check if recreate is requested via context
	wantRecreate := false
	if val, ok := ctx.Value(DockerRecreateKey).(bool); ok {
		wantRecreate = val
	}

	// Select the appropriate instance based on mode
	var instance **DemucsManager
	if mode == DemucsModeGPU {
		instance = &demucsGPUInstance
	} else {
		instance = &demucsCPUInstance
	}

	// If recreate is requested and instance exists, stop and clear it
	if wantRecreate && *instance != nil {
		modeStr := "CPU"
		if mode == DemucsModeGPU {
			modeStr = "GPU"
		}
		DemucsLogger.Info().
			Str("mode", modeStr).
			Msg("Docker recreate requested, stopping existing container")
		(*instance).Stop(ctx)
		*instance = nil
	}

	if *instance == nil {
		// Extract progress handler from context if available
		var handler ProgressHandler
		if h := ctx.Value(ProgressHandlerKey); h != nil {
			if ph, ok := h.(ProgressHandler); ok {
				handler = ph
				DemucsLogger = *handler.ZeroLog()
			}
		}

		// Pull the Docker image first (with progress if handler available)
		if err := pullImageWithProgress(ctx, handler); err != nil {
			return nil, fmt.Errorf("failed to pull Docker image: %w", err)
		}

		mgr, err := NewDemucsManager(ctx, mode)
		if err != nil {
			return nil, err
		}

		// Initialize container - use InitRecreate if explicitly requested
		var initErr error
		if wantRecreate {
			DemucsLogger.Info().Msg("Recreating Docker container")
			initErr = mgr.InitRecreate(ctx)
		} else {
			initErr = mgr.Init(ctx)
		}

		// Handle name conflict errors reactively (instead of preemptive cleanup)
		if initErr != nil && strings.Contains(initErr.Error(), "already in use") {
			DemucsLogger.Warn().Msg("Container name conflict detected, cleaning up stale container and retrying")
			mgr.removeStaleContainer(ctx)
			// Retry initialization
			if wantRecreate {
				initErr = mgr.InitRecreate(ctx)
			} else {
				initErr = mgr.Init(ctx)
			}
		}
		if initErr != nil {
			return nil, initErr
		}

		// Verify container is ready for exec commands
		// (race condition: container "running" but not yet accepting exec)
		if err := mgr.waitForExecReady(ctx); err != nil {
			return nil, fmt.Errorf("container not ready for exec: %w", err)
		}

		*instance = mgr

		// Start idle watcher only once
		demucsWatcherOnce.Do(func() {
			go startDemucsIdleWatcher()
		})
	}

	demucsLastUsed = time.Now()
	return *instance, nil
}

// startDemucsIdleWatcher stops containers after idle timeout
func startDemucsIdleWatcher() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		demucsMu.Lock()
		if time.Since(demucsLastUsed) > demucsIdleTimeout {
			if demucsCPUInstance != nil {
				DemucsLogger.Info().Msg("Stopping idle demucs CPU container")
				demucsCPUInstance.Stop(context.Background())
				demucsCPUInstance = nil
			}
			if demucsGPUInstance != nil {
				DemucsLogger.Info().Msg("Stopping idle demucs GPU container")
				demucsGPUInstance.Stop(context.Background())
				demucsGPUInstance = nil
			}
		}
		demucsMu.Unlock()
	}
}

// StopDemucsManager stops all singleton managers if running and cleans up output directories
func StopDemucsManager() error {
	demucsMu.Lock()
	defer demucsMu.Unlock()

	var lastErr error
	if demucsCPUInstance != nil {
		cleanupDemucsOutput(demucsCPUInstance)
		if err := demucsCPUInstance.Stop(context.Background()); err != nil {
			lastErr = err
		}
		demucsCPUInstance = nil
	}
	if demucsGPUInstance != nil {
		cleanupDemucsOutput(demucsGPUInstance)
		if err := demucsGPUInstance.Stop(context.Background()); err != nil {
			lastErr = err
		}
		demucsGPUInstance = nil
	}
	return lastErr
}

// cleanupDemucsOutput removes the output directory contents after processing
// by executing rm inside the container (since files are owned by root)
func cleanupDemucsOutput(dm *DemucsManager) {
	if dm == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execute cleanup inside the container where we have root permissions
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		DemucsLogger.Warn().Err(err).Msg("Failed to create Docker client for cleanup")
		return
	}
	defer cli.Close()

	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", "rm -rf /data/output/*"},
		AttachStdout: false,
		AttachStderr: false,
	}

	execID, err := cli.ContainerExecCreate(ctx, dm.containerName, execConfig)
	if err != nil {
		DemucsLogger.Warn().Err(err).Msg("Failed to create cleanup exec")
		return
	}

	if err := cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		DemucsLogger.Warn().Err(err).Msg("Failed to execute cleanup command")
		return
	}

	DemucsLogger.Debug().Str("container", dm.containerName).Msg("Cleaned up demucs output directory")
}

// IsDemucsAvailable checks if Docker is available for running demucs
func IsDemucsAvailable() bool {
	// Check if docker command exists
	_, err := exec.LookPath("docker")
	if err != nil {
		return false
	}

	// Check if Docker daemon is reachable
	if err := dockerutil.EngineIsReachable(); err != nil {
		return false
	}

	return true
}
