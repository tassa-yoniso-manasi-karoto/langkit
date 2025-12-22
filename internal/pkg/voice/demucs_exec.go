package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
)

// DemucsMode specifies CPU or GPU execution
type DemucsMode int

const (
	DemucsModeCPU DemucsMode = iota
	DemucsModeGPU
)

// ProgressHandlerKey is the context key for passing progress handler
type progressHandlerKeyType string
const ProgressHandlerKey progressHandlerKeyType = "voice.progressHandler"

// DockerRecreateKey is the context key for passing docker recreate flag
type dockerRecreateKeyType string
const DockerRecreateKey dockerRecreateKeyType = "voice.dockerRecreate"

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

	duration, err := media.GetAudioDurationSeconds(inputPath)
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
	segments, err := media.SplitAudioFile(inputPath, maxSegmentSeconds, segmentDir)
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

	// Concatenate all WAV outputs using media package
	concatenatedWav := filepath.Join(segmentDir, "concatenated.wav")
	concatListFile, err := media.CreateConcatFile(outputPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to create concat list: %w", err)
	}
	defer os.Remove(concatListFile)

	if err := media.RunFFmpegConcat(concatListFile, concatenatedWav); err != nil {
		return nil, fmt.Errorf("failed to concatenate segments: %w", err)
	}

	// Encode to final format (default: flac) using media package
	finalFormat := opts.OutputFormat
	if finalFormat == "" {
		finalFormat = "flac"
	}
	finalOutput := filepath.Join(segmentDir, "final."+finalFormat)
	if err := media.RunFFmpegConvert(concatenatedWav, finalOutput); err != nil {
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

