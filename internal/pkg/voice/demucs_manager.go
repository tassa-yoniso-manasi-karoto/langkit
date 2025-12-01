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
	demucsRemote        = "https://github.com/tassa-yoniso-manasi-karoto/docker-facebook-demucs.git"
	demucsProjectName   = "langkit-demucs"
	demucsContainerName = "langkit-demucs-demucs-1"
	demucsImageName     = "xserrat/facebook-demucs:latest"
)

// ProgressHandlerKey is the context key for passing progress handler
type progressHandlerKeyType string
const ProgressHandlerKey progressHandlerKeyType = "voice.progressHandler"

// ProgressHandler is called to report progress updates
// increment: bytes since last update, total: total bytes, status: current operation
type ProgressHandler interface {
	IncrementProgress(taskID string, increment, total, priority int, operation, descr, size string)
	RemoveProgressBar(taskID string)
	ZeroLog() *zerolog.Logger
}

var (
	// Singleton instance management
	demucsInstance    *DemucsManager
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
		OutputFormat: "wav",
		Stems:        "vocals",
		Shifts:       1,
		Overlap:      0.25,
	}
}

// DemucsManager handles Docker lifecycle for the Demucs project
type DemucsManager struct {
	docker        *dockerutil.DockerManager
	logger        *dockerutil.ContainerLogConsumer
	projectName   string
	containerName string
	configDir     string
}

// NewDemucsManager creates a new Demucs manager instance
func NewDemucsManager(ctx context.Context) (*DemucsManager, error) {
	manager := &DemucsManager{
		projectName:   demucsProjectName,
		containerName: demucsContainerName,
	}

	logConfig := dockerutil.LogConfig{
		Prefix:      manager.projectName,
		ShowService: true,
		ShowType:    true,
		LogLevel:    zerolog.DebugLevel,
		InitMessage: "langkit-demucs",
	}

	logger := dockerutil.NewContainerLogConsumer(logConfig)

	cfg := dockerutil.Config{
		ProjectName:      manager.projectName,
		ComposeFile:      "docker-compose.yml",
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

	// Build command arguments
	cmdArgs := []string{
		"python3", "-m", "demucs",
		"-n", model,
		"--out", "/data/output",
		"--two-stems", stems,
	}

	// Add output format for mp3
	if outputFormat == "mp3" {
		cmdArgs = append(cmdArgs, "--mp3")
	}

	// Add shifts if not default
	if opts.Shifts > 1 {
		cmdArgs = append(cmdArgs, "--shifts", fmt.Sprintf("%d", opts.Shifts))
	}

	// Add overlap if not default
	if opts.Overlap != 0.25 && opts.Overlap > 0 {
		cmdArgs = append(cmdArgs, "--overlap", fmt.Sprintf("%.2f", opts.Overlap))
	}

	// Add the input file path (inside container)
	cmdArgs = append(cmdArgs, "/data/input/"+inputFilename)

	// Execute command in container
	DemucsLogger.Debug().
		Strs("cmd", cmdArgs).
		Str("container", dm.containerName).
		Msg("Executing demucs command")

	output, err := dm.execInContainer(ctx, cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("demucs execution failed: %w\nOutput: %s", err, output)
	}

	DemucsLogger.Debug().Str("output", output).Msg("Demucs command completed")

	// Find the output file
	// Demucs outputs to: /data/output/<model>/<track_name>/<stems>.wav
	trackName := inputFilename[:len(inputFilename)-len(filepath.Ext(inputFilename))]
	ext := outputFormat
	if ext == "" {
		ext = "wav"
	}

	// The vocals file will be at: output/<model>/<trackname>/vocals.<ext>
	vocalsPath := filepath.Join(outputDir, model, trackName, stems+"."+ext)

	DemucsLogger.Debug().Str("vocals_path", vocalsPath).Msg("Looking for output file")

	// Read the output file
	audioData, err := os.ReadFile(vocalsPath)
	if err != nil {
		// Try alternative path structure
		altPath := filepath.Join(outputDir, model, trackName, "vocals."+ext)
		audioData, err = os.ReadFile(altPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read output file: %w (tried %s and %s)", err, vocalsPath, altPath)
		}
	}

	// Clean up output directory for this track
	trackOutputDir := filepath.Join(outputDir, model, trackName)
	os.RemoveAll(trackOutputDir)

	return audioData, nil
}

// execInContainer executes a command in the demucs container
func (dm *DemucsManager) execInContainer(ctx context.Context, cmd []string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Create exec configuration
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Create the exec instance
	execID, err := cli.ContainerExecCreate(ctx, dm.containerName, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to the exec instance
	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read output
	var stdout, stderr bytes.Buffer
	_, err = stdCopy(&stdout, &stderr, resp.Reader)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Check exec exit code
	inspectResp, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return stdout.String() + stderr.String(), fmt.Errorf("failed to inspect exec: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return stdout.String() + stderr.String(), fmt.Errorf("command exited with code %d: %s", inspectResp.ExitCode, stderr.String())
	}

	return stdout.String() + stderr.String(), nil
}

// stdCopy is a helper to demultiplex docker output streams
func stdCopy(stdout, stderr io.Writer, src io.Reader) (int64, error) {
	// Docker multiplexes stdout/stderr with 8-byte headers
	// Header format: [STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4]
	// STREAM_TYPE: 0=stdin, 1=stdout, 2=stderr
	var total int64
	header := make([]byte, 8)

	for {
		_, err := io.ReadFull(src, header)
		if err != nil {
			return total, err
		}

		// Get payload size from header bytes 4-7 (big endian)
		size := int64(header[4])<<24 | int64(header[5])<<16 | int64(header[6])<<8 | int64(header[7])

		var dst io.Writer
		switch header[0] {
		case 1: // stdout
			dst = stdout
		case 2: // stderr
			dst = stderr
		default:
			dst = stdout
		}

		n, err := io.CopyN(dst, src, size)
		total += n
		if err != nil {
			return total, err
		}
	}
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

		// Calculate total progress across all layers
		var totalBytes, currentBytes int64
		for _, lp := range layers {
			totalBytes += lp.total
			currentBytes += lp.current
		}

		// Report progress if handler available and we have meaningful data
		if handler != nil && totalBytes > 0 {
			increment := currentBytes - lastReportedBytes
			if increment > 0 {
				status := msg.Status
				if msg.ID != "" {
					status = msg.ID + ": " + status
				}
				handler.IncrementProgress(
					taskID,
					int(increment),
					int(totalBytes),
					20, // priority (lower than main tasks)
					"Docker Pull",
					status,
					humanize.Bytes(uint64(currentBytes)) + " / " + humanize.Bytes(uint64(totalBytes)),
				)
				lastReportedBytes = currentBytes
			}
		}

		DemucsLogger.Trace().
			Str("status", msg.Status).
			Str("id", msg.ID).
			Int64("current", currentBytes).
			Int64("total", totalBytes).
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

// GetDemucsManager returns or creates the singleton manager
func GetDemucsManager(ctx context.Context) (*DemucsManager, error) {
	demucsMu.Lock()
	defer demucsMu.Unlock()

	if demucsInstance == nil {
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

		mgr, err := NewDemucsManager(ctx)
		if err != nil {
			return nil, err
		}
		if err := mgr.Init(ctx); err != nil {
			return nil, err
		}
		demucsInstance = mgr

		// Start idle watcher only once
		demucsWatcherOnce.Do(func() {
			go startDemucsIdleWatcher()
		})
	}

	demucsLastUsed = time.Now()
	return demucsInstance, nil
}

// startDemucsIdleWatcher stops container after idle timeout
func startDemucsIdleWatcher() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		demucsMu.Lock()
		if demucsInstance != nil && time.Since(demucsLastUsed) > demucsIdleTimeout {
			DemucsLogger.Info().Msg("Stopping idle demucs container")
			demucsInstance.Stop(context.Background())
			demucsInstance = nil
		}
		demucsMu.Unlock()
	}
}

// StopDemucsManager stops the singleton manager if running
func StopDemucsManager() error {
	demucsMu.Lock()
	defer demucsMu.Unlock()

	if demucsInstance != nil {
		err := demucsInstance.Stop(context.Background())
		demucsInstance = nil
		return err
	}
	return nil
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
