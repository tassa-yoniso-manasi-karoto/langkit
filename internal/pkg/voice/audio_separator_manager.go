package voice

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	composetypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/dustin/go-humanize"

	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
)

const (
	audioSepProjectName    = "langkit-audio-separator"
	audioSepImageGPU       = "beveradb/audio-separator:gpu"
	audioSepImageCPU       = "beveradb/audio-separator"
	audioSepModelFilename  = "vocals_mel_band_roformer.ckpt"

	// VRAMPerContainerGB defines the hard memory limit per container.
	// Based on testing: model default segment_size (~800) uses ~3.8GB VRAM.
	VRAMPerContainerGB = 4.0
)

// AudioSepMode specifies CPU or GPU execution
type AudioSepMode int

const (
	AudioSepModeCPU AudioSepMode = iota
	AudioSepModeGPU
)

func (m AudioSepMode) projectName() string {
	if m == AudioSepModeGPU {
		return "langkit-audio-separator-gpu"
	}
	return "langkit-audio-separator"
}

func (m AudioSepMode) containerName() string {
	if m == AudioSepModeGPU {
		return "langkit-audio-separator-gpu-audiosep-1"
	}
	return "langkit-audio-separator-audiosep-1"
}

func (m AudioSepMode) imageName() string {
	if m == AudioSepModeGPU {
		return audioSepImageGPU
	}
	return audioSepImageCPU
}

// getModelsDir returns the XDG-compliant directory for caching audio-separator models
func getModelsDir() (string, error) {
	modelsDir := filepath.Join(xdg.ConfigHome, "audio-separator-models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create models directory: %w", err)
	}
	return modelsDir, nil
}

// buildComposeProject creates the compose project definition for audio-separator
func (m AudioSepMode) buildComposeProject(configDir, modelsDir string) *composetypes.Project {
	defaultNetworkName := m.projectName() + "_default"

	service := composetypes.ServiceConfig{
		Name:          "audiosep",
		ContainerName: m.containerName(),
		Image:         m.imageName(),
		StdinOpen:     true,
		Tty:           true,
		WorkingDir:    "/workdir",
		// Override entrypoint: output init message for dockerutil, then keep container alive
		// (default entrypoint is audio-separator CLI which exits immediately)
		Entrypoint: composetypes.ShellCommand{"/bin/sh", "-c", "sleep 0.75 && echo 'langkit-audio-separator' && exec tail -f /dev/null"},
		// Use host network for DNS resolution
		NetworkMode: "host",
		Volumes: []composetypes.ServiceVolumeConfig{
			{
				Type:   composetypes.VolumeTypeBind,
				Source: filepath.Join(configDir, "workdir"),
				Target: "/workdir",
			},
			{
				Type:   composetypes.VolumeTypeBind,
				Source: modelsDir,
				Target: "/models",
			},
		},
		Environment: composetypes.MappingWithEquals{
			"AUDIO_SEPARATOR_MODEL_DIR": stringPtr("/models"),
		},
	}

	// Add GPU device reservation for GPU mode
	if m == AudioSepModeGPU {
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
		Networks: composetypes.Networks{
			"default": composetypes.NetworkConfig{
				Name: defaultNetworkName,
			},
		},
		Services: composetypes.Services{
			"audiosep": service,
		},
	}
}

func stringPtr(s string) *string {
	return &s
}

var (
	// Singleton instance management
	audioSepCPUInstance *AudioSeparatorManager
	audioSepGPUInstance *AudioSeparatorManager
	audioSepMu          sync.Mutex
	audioSepLastUsed    time.Time
	audioSepIdleTimeout = 30 * time.Minute
)

// AudioSeparatorManager handles Docker lifecycle for audio-separator
type AudioSeparatorManager struct {
	docker        *dockerutil.DockerManager
	logger        *dockerutil.ContainerLogConsumer
	mode          AudioSepMode
	projectName   string
	containerName string
	configDir     string
	modelsDir     string
}

// NewAudioSeparatorManager creates a new AudioSeparatorManager instance
func NewAudioSeparatorManager(ctx context.Context, mode AudioSepMode) (*AudioSeparatorManager, error) {
	manager := &AudioSeparatorManager{
		mode:          mode,
		projectName:   mode.projectName(),
		containerName: mode.containerName(),
	}

	// Get the config directory for volume paths
	configDir, err := dockerutil.GetConfigDir(manager.projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	// Get models directory (XDG-compliant, persistent across sessions)
	modelsDir, err := getModelsDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get models directory: %w", err)
	}

	// Ensure workdir exists
	if err := os.MkdirAll(filepath.Join(configDir, "workdir"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create workdir: %w", err)
	}

	// Build compose project
	project := mode.buildComposeProject(configDir, modelsDir)

	logConfig := dockerutil.LogConfig{
		Prefix:      manager.projectName,
		ShowService: true,
		ShowType:    true,
		InitMessage: "langkit-audio-separator", // Must match entrypoint echo
	}

	logger := dockerutil.NewContainerLogConsumer(logConfig)

	cfg := dockerutil.Config{
		ProjectName:      manager.projectName,
		Project:          project,
		RequiredServices: []string{"audiosep"},
		LogConsumer:      logger,
		Timeout: dockerutil.Timeout{
			Create:   5 * time.Minute,
			Recreate: 5 * time.Minute,
			Start:    2 * time.Minute,
		},
	}

	dockerManager, err := dockerutil.NewDockerManager(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker manager: %w", err)
	}

	manager.docker = dockerManager
	manager.logger = logger
	manager.configDir = configDir
	manager.modelsDir = modelsDir

	return manager, nil
}

// Init initializes the docker service
func (m *AudioSeparatorManager) Init(ctx context.Context) error {
	return m.docker.Init()
}

// InitRecreate forces recreation of the docker container
func (m *AudioSeparatorManager) InitRecreate(ctx context.Context) error {
	return m.docker.InitRecreate()
}

// Stop stops the docker service
func (m *AudioSeparatorManager) Stop(ctx context.Context) error {
	return m.docker.Stop()
}

// Close implements io.Closer
func (m *AudioSeparatorManager) Close() error {
	m.logger.Close()
	return m.docker.Close()
}

// GetContainerName returns the name of the main container
func (m *AudioSeparatorManager) GetContainerName() string {
	return m.containerName
}

// waitForExecReady waits for the container to be ready to accept exec commands
func (m *AudioSeparatorManager) waitForExecReady(ctx context.Context) error {
	maxRetries := 10
	retryDelay := 200 * time.Millisecond
	if m.mode == AudioSepModeGPU {
		maxRetries = 30
		retryDelay = 500 * time.Millisecond
	}

	for i := 0; i < maxRetries; i++ {
		_, err := m.execInContainer(ctx, []string{"true"})
		if err == nil {
			return nil
		}
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("container not ready after %d attempts", maxRetries)
}

// removeStaleContainer removes any existing container with the same name
func (m *AudioSeparatorManager) removeStaleContainer(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil
	}
	defer cli.Close()

	stopTimeout := 5
	_ = cli.ContainerStop(ctx, m.containerName, container.StopOptions{Timeout: &stopTimeout})
	time.Sleep(100 * time.Millisecond)

	err = cli.ContainerRemove(ctx, m.containerName, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: false,
	})
	if err != nil && !strings.Contains(err.Error(), "No such container") &&
		!strings.Contains(err.Error(), "not found") {
		Logger.Warn().Err(err).Str("container", m.containerName).Msg("Failed to remove stale container")
	}

	time.Sleep(200 * time.Millisecond)
	return nil
}

// execInContainer executes a command in the audio-separator container
func (m *AudioSeparatorManager) execInContainer(ctx context.Context, cmd []string) (string, error) {
	return m.execInContainerWithProgress(ctx, cmd, nil)
}

// AudioSepPhase represents the current phase of audio-separator execution
type AudioSepPhase int

const (
	AudioSepPhaseUnknown AudioSepPhase = iota
	AudioSepPhaseModelDownload
	AudioSepPhaseProcessing
)

// AudioSepProgressUpdate contains progress info with phase context
type AudioSepProgressUpdate struct {
	Phase   AudioSepPhase
	Percent int
}

// AudioSepProgressCallback is called with progress updates
type AudioSepProgressCallback func(update AudioSepProgressUpdate)

// execInContainerWithProgress executes a command with optional progress callback
func (m *AudioSeparatorManager) execInContainerWithProgress(ctx context.Context, cmd []string, progressCb AudioSepProgressCallback) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// audio-separator entrypoint is the CLI itself, so cmd is just the args
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Env:          []string{"COLUMNS=200", "TERM=xterm-256color"},
	}

	execID, err := cli.ContainerExecCreate(ctx, m.containerName, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{
		Tty:         true,
		ConsoleSize: &[2]uint{50, 200},
	})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	var output bytes.Buffer
	buf := make([]byte, 4096)
	var lastPercent int = -1
	var currentPhase AudioSepPhase = AudioSepPhaseUnknown

	for {
		n, readErr := resp.Reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			output.Write(chunk)

			if progressCb != nil {
				phase, pct := parseAudioSepProgress(chunk, currentPhase)
				if phase != AudioSepPhaseUnknown {
					currentPhase = phase
				}
				if pct >= 0 && (pct != lastPercent || phase != AudioSepPhaseUnknown) {
					progressCb(AudioSepProgressUpdate{Phase: currentPhase, Percent: pct})
					lastPercent = pct
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	inspectResp, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return output.String(), fmt.Errorf("failed to inspect exec: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return output.String(), fmt.Errorf("command exited with code %d: %s", inspectResp.ExitCode, output.String())
	}

	return output.String(), nil
}

// parseAudioSepProgress extracts phase and percentage from audio-separator output
func parseAudioSepProgress(data []byte, currentPhase AudioSepPhase) (AudioSepPhase, int) {
	str := string(data)
	detectedPhase := AudioSepPhaseUnknown

	// Detect phase from output patterns:
	// Download progress: "0% 81.9k/913M [00:00<19:45, 770kiB/s]" - has "iB/s" (bytes per second)
	// Processing progress: "0% 0/24 [00:00<?, ?it/s]" - has "it/s" (iterations per second)
	if bytes.Contains(data, []byte("iB/s")) {
		detectedPhase = AudioSepPhaseModelDownload
	} else if bytes.Contains(data, []byte("it/s")) {
		detectedPhase = AudioSepPhaseProcessing
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

// pullImageWithProgress pulls the audio-separator Docker image with progress reporting
func pullAudioSepImageWithProgress(ctx context.Context, mode AudioSepMode, handler ProgressHandler) error {
	imageName := mode.imageName()

	if handler != nil {
		handler.ZeroLog().Info().
			Str("image", imageName).
			Msg("Pulling Docker image for MelBand RoFormer voice separation...")
	}

	opts := dockerutil.DefaultPullOptions()
	taskID := progress.BarAudioSepDockerDL

	if handler != nil {
		var lastBytes int64

		opts.OnProgress = func(current, total int64, status string) {
			increment := current - lastBytes
			if increment > 0 {
				handler.IncrementDownloadProgress(
					taskID,
					int(increment),
					int(total),
					20,
					"Audio Separator Setup",
					status,
					"",
					humanize.Bytes(uint64(current))+" / "+humanize.Bytes(uint64(total)),
				)
				lastBytes = current
			}
		}
		opts.OnRetry = func(err error, nextRetryIn time.Duration) {
			handler.RemoveProgressBar(taskID)
			handler.ZeroLog().Warn().
				Err(err).
				Dur("retry_in", nextRetryIn).
				Msg("Docker pull failed, retrying...")
		}
	}

	err := dockerutil.PullImage(ctx, imageName, opts)
	if handler != nil {
		handler.RemoveProgressBar(taskID)
		if err == nil {
			handler.ZeroLog().Info().Msg("Docker image pull complete")
		}
	}
	return err
}

// GetAudioSeparatorManager returns or creates the singleton manager for the specified mode
func GetAudioSeparatorManager(ctx context.Context, mode AudioSepMode) (*AudioSeparatorManager, error) {
	audioSepMu.Lock()
	defer audioSepMu.Unlock()

	// Check if recreate is requested via context
	wantRecreate := false
	if val, ok := ctx.Value(DockerRecreateKey).(bool); ok {
		wantRecreate = val
	}

	// Select the appropriate instance based on mode
	var instance **AudioSeparatorManager
	if mode == AudioSepModeGPU {
		instance = &audioSepGPUInstance
	} else {
		instance = &audioSepCPUInstance
	}

	// If recreate is requested and instance exists, stop and clear it
	if wantRecreate && *instance != nil {
		modeStr := "CPU"
		if mode == AudioSepModeGPU {
			modeStr = "GPU"
		}
		Logger.Info().
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

		// Pull the Docker image first
		if err := pullAudioSepImageWithProgress(ctx, mode, handler); err != nil {
			return nil, fmt.Errorf("failed to pull Docker image: %w", err)
		}

		mgr, err := NewAudioSeparatorManager(ctx, mode)
		if err != nil {
			return nil, err
		}

		// Initialize container
		var initErr error
		if wantRecreate {
			Logger.Info().Msg("Recreating Docker container")
			initErr = mgr.InitRecreate(ctx)
		} else {
			initErr = mgr.Init(ctx)
		}

		// Handle name conflict errors reactively
		if initErr != nil && strings.Contains(initErr.Error(), "already in use") {
			Logger.Warn().Msg("Container name conflict detected, cleaning up stale container and retrying")
			mgr.removeStaleContainer(ctx)
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
		if err := mgr.waitForExecReady(ctx); err != nil {
			return nil, fmt.Errorf("container not ready for exec: %w", err)
		}

		*instance = mgr
	}

	audioSepLastUsed = time.Now()
	return *instance, nil
}

// StopAudioSeparatorManager stops all singleton managers
func StopAudioSeparatorManager() error {
	audioSepMu.Lock()
	defer audioSepMu.Unlock()

	var lastErr error
	if audioSepCPUInstance != nil {
		cleanupAudioSepWorkdir(audioSepCPUInstance)
		if err := audioSepCPUInstance.Stop(context.Background()); err != nil {
			lastErr = err
		}
		audioSepCPUInstance = nil
	}
	if audioSepGPUInstance != nil {
		cleanupAudioSepWorkdir(audioSepGPUInstance)
		if err := audioSepGPUInstance.Stop(context.Background()); err != nil {
			lastErr = err
		}
		audioSepGPUInstance = nil
	}
	return lastErr
}

// cleanupAudioSepWorkdir removes the workdir contents after processing
func cleanupAudioSepWorkdir(m *AudioSeparatorManager) {
	if m == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		Logger.Warn().Err(err).Msg("Failed to create Docker client for cleanup")
		return
	}
	defer cli.Close()

	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", "rm -rf /workdir/*"},
		AttachStdout: false,
		AttachStderr: false,
	}

	execID, err := cli.ContainerExecCreate(ctx, m.containerName, execConfig)
	if err != nil {
		Logger.Warn().Err(err).Msg("Failed to create cleanup exec")
		return
	}

	if err := cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		Logger.Warn().Err(err).Msg("Failed to execute cleanup command")
		return
	}

	Logger.Debug().Str("container", m.containerName).Msg("Cleaned up audio-separator workdir")
}

// IsAudioSeparatorAvailable checks if Docker is available
func IsAudioSeparatorAvailable() bool {
	_, err := exec.LookPath("docker")
	if err != nil {
		return false
	}
	if err := dockerutil.EngineIsReachable(); err != nil {
		return false
	}
	return true
}

// since this model is GPU cores-bound, processes sequentially and only need 4GB of VRAM,
// multi container parallelism was considered to improve processing speed however 
// preliminary tests revealed +10.5% longer processing due to GPU contention when running
// 2 containers so the code below is kept aside as it could be useful for supporting other models
// in the future. Rule of thumb was made by Gemini using Wikipedia data "List of Nvidia GPUs"
func CalculateParallelContainers(gpuName string, vramGB float64) int {
	nameUpper := strings.ToUpper(gpuName)

	// 1. Check Outliers (high VRAM but weak compute)
	outliers := []string{
		"GTX 16",   // No Tensor Cores
		"RTX 2050", "RTX 3050", "RTX 4050", "RTX 5050", "RTX 6050", // Entry level
		"RTX 2060", "RTX 3060", "RTX 4060", "RTX 5060", "RTX 6060",
	}

	for _, s := range outliers {
		if strings.Contains(nameUpper, s) {
			return 1
		}
	}

	// 2. Determine Divisor based on Platform/Architecture
	divisor := 6.0 // Default: Desktop Consumer

	conservativeKeywords := []string{
		"LAPTOP", "MOBILE", "MAX-Q",              // Mobile platforms
		"QUADRO", "RTX A", "TESLA", "BLACKWELL",  // Workstation/Server
		"T1000", "T600", "T500",                  // Entry-level Workstation
		"RTX 2000", "RTX 5880",                   // Specific Ada Generation Workstation cards
	}

	for _, k := range conservativeKeywords {
		if strings.Contains(nameUpper, k) {
			divisor = 8.0
			break
		}
	}

	// 3. Calculate max containers based on Compute Rule of Thumb
	maxByCompute := int(math.Round(vramGB / divisor))

	// 4. Calculate max containers based on Physical VRAM Capacity
	maxByVRAM := int(vramGB / VRAMPerContainerGB)

	// 5. Apply Constraints - take the minimum
	count := maxByCompute
	if maxByVRAM < count {
		count = maxByVRAM
	}

	if count < 1 {
		return 1
	}

	return count
}

// GetGPUInfo returns the GPU name and VRAM in GB
func GetGPUInfo() (name string, vramGB float64) {
	name = executils.GetNvidiaGPUName()
	vramMiB := executils.GetNvidiaVRAMMiB()
	vramGB = float64(vramMiB) / 1024.0
	return
}
