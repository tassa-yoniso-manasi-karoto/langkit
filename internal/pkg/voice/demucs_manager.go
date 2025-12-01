package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
)

const (
	demucsRemote         = "https://github.com/tassa-yoniso-manasi-karoto/docker-facebook-demucs.git"
	demucsProjectName    = "langkit-demucs" // Base project name for config dir
	demucsImageName      = "langkit-demucs:latest" // Local image (TODO: change to GHCR after push)
	demucsImageSizeBytes = 5_000_000_000 // ~5 GB for CUDA 12 + PyTorch + demucs models
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
	if m == DemucsModeGPU {
		return "langkit-demucs-gpu"
	}
	return "langkit-demucs"
}

func (m DemucsMode) composeFile() string {
	if m == DemucsModeGPU {
		return "docker-compose-gpu.yml"
	}
	return "docker-compose.yml"
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
	Model        string  // htdemucs, htdemucs_ft, etc. (default: htdemucs)
	OutputFormat string  // wav, mp3, flac (default: wav)
	Stems        string  // vocals, drums, bass, other (default: vocals)
	Shifts       int     // shift trick for better quality (default: 1)
	Overlap      float64 // overlap between prediction windows (default: 0.25)
}

// DefaultDemucsOptions returns default options for demucs
func DefaultDemucsOptions() DemucsOptions {
	return DemucsOptions{
		Model:        "htdemucs",
		OutputFormat: "flac", // flac/opus keep timing sync, mp3/wav can cause A/V desync
		Stems:        "vocals",
		Shifts:       1,
		Overlap:      0.25,
	}
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
	manager := &DemucsManager{
		mode:          mode,
		projectName:   mode.projectName(),
		containerName: mode.containerName(),
	}

	logConfig := dockerutil.LogConfig{
		Prefix:      manager.projectName,
		ShowService: true,
		ShowType:    true,
		LogLevel:    zerolog.DebugLevel,
		InitMessage: manager.projectName,
	}

	logger := dockerutil.NewContainerLogConsumer(logConfig)

	cfg := dockerutil.Config{
		ProjectName:      manager.projectName,
		ComposeFile:      mode.composeFile(),
		RemoteRepo:       demucsRemote,
		RequiredServices: []string{"demucs"},
		LogConsumer:      logger,
		Timeout: dockerutil.Timeout{
			Create:   300 * time.Second,  // 5 min for initial image pull
			Recreate: 10 * time.Minute,   // 10 min for recreate with model download
			Start:    60 * time.Second,   // 1 min to reach running state
		},
	}

	dockerManager, err := dockerutil.NewDockerManager(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker manager: %w", err)
	}

	// Get the config directory for volume paths
	configDir, err := dockerutil.GetConfigDir(demucsProjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
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

// ProcessAudio runs demucs on the input audio file and returns the vocals track
func (dm *DemucsManager) ProcessAudio(ctx context.Context, inputPath string, opts DemucsOptions) ([]byte, error) {
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
		Strs("cmd", cmdArgs).
		Str("container", dm.containerName).
		Msg("Executing demucs command")

	// Extract progress handler from context if available
	var progressCb ProgressCallback
	if h := ctx.Value(ProgressHandlerKey); h != nil {
		if handler, ok := h.(ProgressHandler); ok {
			downloadTaskID := "demucs-model-download"
			processTaskID := "demucs-process"
			var lastDownloadPercent, lastProcessPercent int
			var currentPhase DemucsPhase

			progressCb = func(update ProgressUpdate) {
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
						handler.IncrementProgress(downloadTaskID, increment, 100, 25, "Demucs Setup", "Downloading model weights...", "h-2")
						lastDownloadPercent = update.Percent
					}
				case PhaseProcessing:
					increment := update.Percent - lastProcessPercent
					if increment > 0 {
						handler.IncrementProgress(processTaskID, increment, 100, 30, "Voice Separation", "Processing audio...", "h-2")
						lastProcessPercent = update.Percent
					}
				}
			}
		}
	}

	output, err := dm.execInContainerWithProgress(ctx, cmdArgs, progressCb)
	if err != nil {
		return nil, fmt.Errorf("demucs execution failed: %w\nOutput: %s", err, output)
	}

	DemucsLogger.Debug().Str("output", output).Msg("Demucs command completed")

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
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	// Create the exec instance
	execID, err := cli.ContainerExecCreate(ctx, dm.containerName, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to the exec instance
	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{Tty: true})
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
	// "Downloading htdemucs" indicates model download phase
	if bytes.Contains(data, []byte("Downloading")) {
		detectedPhase = PhaseModelDownload
	}
	// "Separated track" or audio file extensions indicate processing phase
	if bytes.Contains(data, []byte("Separated track")) ||
		bytes.Contains(data, []byte(".opus")) ||
		bytes.Contains(data, []byte(".mp3")) ||
		bytes.Contains(data, []byte(".flac")) ||
		bytes.Contains(data, []byte(".wav")) {
		// Only switch to processing if we see these AND we're past download
		// (the filename appears in both phases, so check for "Separated" or after download)
		if bytes.Contains(data, []byte("Separated track")) || currentPhase == PhaseModelDownload {
			detectedPhase = PhaseProcessing
		}
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

// pullImageWithProgress pulls the demucs Docker image with progress reporting
func pullImageWithProgress(ctx context.Context, handler ProgressHandler) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Check if image already exists
	_, _, err = cli.ImageInspectWithRaw(ctx, demucsImageName)
	if err == nil {
		DemucsLogger.Debug().Str("image", demucsImageName).Msg("Docker image already exists, skipping pull")
		return nil
	}

	if handler != nil {
		handler.ZeroLog().Info().
			Str("image", demucsImageName).
			Msg("Pulling Docker image for local voice separation (first-time setup, ~2.5GB download)...")
	}
	DemucsLogger.Info().Str("image", demucsImageName).Msg("Pulling Docker image")

	// Pull the image
	reader, err := cli.ImagePull(ctx, demucsImageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Track progress per layer
	type layerProgress struct {
		current int64
		total   int64
	}
	layers := make(map[string]*layerProgress)
	var lastReportedBytes int64
	taskID := "docker-pull"

	decoder := json.NewDecoder(reader)
	for {
		var msg jsonmessage.JSONMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode pull progress: %w", err)
		}

		// Track layer progress
		if msg.ID != "" && msg.Progress != nil {
			if layers[msg.ID] == nil {
				layers[msg.ID] = &layerProgress{}
			}
			layers[msg.ID].current = msg.Progress.Current
			layers[msg.ID].total = msg.Progress.Total
		}

		// Calculate current downloaded bytes across all layers
		var currentBytes int64
		for _, lp := range layers {
			currentBytes += lp.current
		}

		// Report progress using fixed known total size (avoids backwards progress)
		if handler != nil && currentBytes > 0 {
			increment := currentBytes - lastReportedBytes
			if increment > 0 {
				// Show user-friendly status without cryptic layer IDs
				description := "Downloading..."
				if msg.Status == "Extracting" {
					description = "Extracting..."
				} else if msg.Status == "Pull complete" || msg.Status == "Already exists" {
					description = "Finalizing..."
				}
				handler.IncrementDownloadProgress(
					taskID,
					int(increment),
					demucsImageSizeBytes, // Use fixed known size
					20,                   // priority (lower than main tasks)
					"Demucs Setup (docker pull)",
					description,
					"h-3", // height class for download progress bar
					humanize.Bytes(uint64(currentBytes))+" / "+humanize.Bytes(demucsImageSizeBytes),
				)
				lastReportedBytes = currentBytes
			}
		}

		DemucsLogger.Trace().
			Str("status", msg.Status).
			Str("id", msg.ID).
			Int64("current", currentBytes).
			Int64("total", demucsImageSizeBytes).
			Msg("Pull progress")
	}

	// Clean up progress bar
	if handler != nil {
		handler.RemoveProgressBar(taskID)
		handler.ZeroLog().Info().Msg("Docker image pull complete")
	}
	DemucsLogger.Info().Str("image", demucsImageName).Msg("Docker image pull complete")

	return nil
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

		// Use InitRecreate if recreate was requested, otherwise normal Init
		if wantRecreate {
			DemucsLogger.Info().Msg("Recreating Docker container")
			if err := mgr.InitRecreate(ctx); err != nil {
				return nil, err
			}
		} else {
			if err := mgr.Init(ctx); err != nil {
				return nil, err
			}
		}

		// Brief pause to ensure container is fully ready for exec
		// (race condition: container "running" but not yet accepting exec)
		time.Sleep(500 * time.Millisecond)

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

// StopDemucsManager stops all singleton managers if running
func StopDemucsManager() error {
	demucsMu.Lock()
	defer demucsMu.Unlock()

	var lastErr error
	if demucsCPUInstance != nil {
		if err := demucsCPUInstance.Stop(context.Background()); err != nil {
			lastErr = err
		}
		demucsCPUInstance = nil
	}
	if demucsGPUInstance != nil {
		if err := demucsGPUInstance.Stop(context.Background()); err != nil {
			lastErr = err
		}
		demucsGPUInstance = nil
	}
	return lastErr
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
