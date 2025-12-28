package voice

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	composetypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
)

const (
	demucsProjectName = "langkit-demucs" // Base project name for config dir
	demucsImageName   = "ghcr.io/tassa-yoniso-manasi-karoto/langkit-demucs:latest"
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
func (m DemucsMode) buildComposeProject(configDir, modelsDir string) *composetypes.Project {
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
				Source: modelsDir,
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

var (
	// Singleton instance management - separate instances for CPU and GPU
	demucsCPUInstance *DemucsManager
	demucsGPUInstance *DemucsManager
	demucsMu          sync.Mutex
	demucsLastUsed    time.Time
	demucsIdleTimeout = 30 * time.Minute
	demucsWatcherOnce sync.Once
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


// DemucsManager handles Docker lifecycle for the Demucs project
type DemucsManager struct {
	docker        *dockerutil.DockerManager
	logger        *dockerutil.ContainerLogConsumer
	mode          DemucsMode
	projectName   string
	containerName string
	configDir     string
	modelsDir     string
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

	// Get shared models directory (migration is handled by GetDemucsManager before creating manager)
	modelsDir, err := GetDemucsModelsDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get models directory: %w", err)
	}

	// Ensure volume directories exist (models dir is now shared, not per-container)
	for _, subdir := range []string{"input", "output"} {
		if err := os.MkdirAll(filepath.Join(configDir, subdir), 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s directory: %w", subdir, err)
		}
	}

	// Build compose project with shared models directory
	project := mode.buildComposeProject(configDir, modelsDir)

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
	manager.modelsDir = modelsDir

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
			Logger.Warn().Err(err).Str("container", dm.containerName).Msg("Failed to remove stale container")
		}
		return nil
	}

	// Brief pause to ensure Docker has processed the removal
	time.Sleep(200 * time.Millisecond)

	Logger.Debug().Str("container", dm.containerName).Msg("Removed stale container")
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

	// Run migration from old per-container directories to shared location.
	// If migration occurred, containers need recreation to use new volume mounts.
	if migrated, err := MigrateDemucsModels(); err != nil {
		Logger.Warn().Err(err).Msg("Model migration had issues, continuing anyway")
	} else if migrated {
		Logger.Info().Msg("Model files migrated to shared directory, containers will be recreated")
		wantRecreate = true
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
			Logger.Info().Msg("Recreating Docker container")
			initErr = mgr.InitRecreate(ctx)
		} else {
			initErr = mgr.Init(ctx)
		}

		// Handle name conflict errors reactively (instead of preemptive cleanup)
		if initErr != nil && strings.Contains(initErr.Error(), "already in use") {
			Logger.Warn().Msg("Container name conflict detected, cleaning up stale container and retrying")
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
	}

	demucsLastUsed = time.Now()
	return *instance, nil
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

// cleanupDemucsOutput removes the input and output directory contents after processing
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
		Logger.Warn().Err(err).Msg("Failed to create Docker client for cleanup")
		return
	}
	defer cli.Close()

	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", "rm -rf /data/output/* /data/input/*"},
		AttachStdout: false,
		AttachStderr: false,
	}

	execID, err := cli.ContainerExecCreate(ctx, dm.containerName, execConfig)
	if err != nil {
		// Silently ignore "not running" errors - container may have been stopped by user cancellation
		if !strings.Contains(err.Error(), "is not running") {
			Logger.Warn().Err(err).Msg("Failed to create cleanup exec")
		}
		return
	}

	if err := cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		if !strings.Contains(err.Error(), "is not running") {
			Logger.Warn().Err(err).Msg("Failed to execute cleanup command")
		}
		return
	}

	Logger.Debug().Str("container", dm.containerName).Msg("Cleaned up demucs input/output directories")
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
