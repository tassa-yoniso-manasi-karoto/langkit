package core

import (
	"os"
	"strings"
	"path/filepath"
	"fmt"
	"io"
	"context"
	"errors"
	"time"
	
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/schollz/progressbar/v3"

	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/fsutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/profiling"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)


var (
	itembar *progressbar.ProgressBar
	totalItems int
	memoryProfilerDone chan struct{} // Channel to stop memory profiler goroutine
)

func (tsk *Task) Routing(ctx context.Context) (procErr *ProcessingError) {
	// Reset totalItems at the start of each processing run to prevent accumulation
	// across cancellations and restarts
	totalItems = 0

	// safety measure against cross-runtime dataraces: see commit message
	// of b8faf4e for lessons learned on this topic
	if !tsk.Handler.IsCLI() {
		tsk.Handler.ZeroLog().Trace().Msg("SLEEPING 750ms FOR FRONTEND TO BE READY")
		time.Sleep(750 * time.Millisecond)
	}
	reporter := crash.Reporter
   	reporter.SaveSnapshot("Starting routing", tsk.DebugVals()) // necessity: high

	// Initialize TranslitProviderManager if not already initialized
	if DefaultProviderManager == nil {
		logger := tsk.Handler.ZeroLog().With().Str("component", "provider_manager_init").Logger()
		logger.Info().Msg("Initializing TranslitProviderManager")
		InitTranslitService(logger)
	}
	
	voice.Logger = *tsk.Handler.ZeroLog()
	
	// Register a deferred cleanup function to ensure TranslitProviderManager and Demucs
	// are properly shut down for non-bulk processing tasks (bulk processing has its own cleanup)
	if !tsk.IsBulkProcess {
		defer func() {
			if DefaultProviderManager != nil {
				tsk.Handler.ZeroLog().Info().Msg("Shutting down TranslitProviderManager")
				ShutdownTranslitService()
			}
			// Stop demucs container if it was started during this processing run
			if err := voice.StopDemucsManager(); err != nil {
				tsk.Handler.ZeroLog().Warn().Err(err).Msg("Failed to stop Demucs container")
			}
			// Stop audio-separator container if it was started
			if err := voice.StopAudioSeparatorManager(); err != nil {
				tsk.Handler.ZeroLog().Warn().Err(err).Msg("Failed to stop audio-separator container")
			}
		}()
	}

	// Start memory profiler if enabled (30 second interval)
	if profiling.IsMemoryProfilingEnabled() {
		memoryProfilerDone = profiling.StartMemoryProfiler("routing", 30*time.Second)
		tsk.Handler.ZeroLog().Info().Msg("Memory profiling enabled (30s interval)")
		
		// Make sure to stop the profiler when we're done
		defer func() {
			if memoryProfilerDone != nil {
				close(memoryProfilerDone)
				memoryProfilerDone = nil
				tsk.Handler.ZeroLog().Info().Msg("Memory profiler stopped")
			}
		}()
	}
	
	// Start CPU profiling if enabled
	var cpuProfileFile *os.File
	if profiling.IsCPUProfilingEnabled() {
		var err error
		cpuProfileFile, err = profiling.StartCPUProfile("routing")
		if err != nil {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("Failed to start CPU profiling")
		} else if cpuProfileFile != nil {
			tsk.Handler.ZeroLog().Info().Msg("CPU profiling enabled")
			defer profiling.StopCPUProfile(cpuProfileFile)
		}
	}
	
	version, err := media.GetFFmpegVersion()
	if err != nil {
		reporter.SaveSnapshot("FFmpeg access failed", tsk.DebugVals()) // necessity: critical
		return tsk.Handler.LogErr(err, AbortAllTasks, "failed to access FFmpeg binary")
	}
	crash.Reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
		gs.FFmpegPath = media.FFmpegPath
		gs.FFmpegVersion = version
	})
	
	// reassign to have root dir if IsBulkProcess
	userProvided := tsk.MediaSourceFile
	
	tsk.Handler.ZeroLog().Info().
		Str("path", userProvided).
		Str("mode", tsk.Mode.String()).
		Msg("Starting routing")
	
	stat, err := os.Stat(userProvided)
	if err != nil {
		reporter.SaveSnapshot("Media file/dir access failed", tsk.DebugVals()) // necessity: high
		// NOTE: these two loggers are equivalent: they would both log to STDERR
		// and to the GUI (if applicable). The only difference is that
		// Log[Err][Fields]() returns a ProcessingError that can be used
		// to define an error handling strategy. Also, it can be a bit more concise.
		return tsk.Handler.LogErr(err, AbortAllTasks, "can't access passed media file/directory")
		//tsk.Handler.ZeroLog().Error().
		//	Err(err).Str("behavior", AbortAllTasks).
		//	Msg("can't access passed media file/directory")
	}

	// Check disk space before starting processing.
	// 10 GB minimum is required for temporary files, intermediate outputs, etc.
	requiredDiskSpaceGB := 10
	usingDockerVoiceSep := strings.Contains(tsk.SeparationLib, "docker")
	if usingDockerVoiceSep {
		// Docker demucs image is ~7 GB compressed but expands to ~14 GB on disk,
		// so we need 20 GB to safely accommodate the image plus processing headroom.
		requiredDiskSpaceGB = 20
		tsk.Handler.ZeroLog().Debug().Msg("Docker voice separation detected, requiring 20 GB disk space")
	}

	// Pre-pull busybox for maintenance tasks (migration, cleanup) if Docker is available
	if dockerutil.EngineIsReachable() == nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			cmd := executils.CommandContext(ctx, "docker", "pull", "-q", "busybox")
			_ = cmd.Run() // Best effort, ignore errors
		}()
	}

	// Get the directory to check (use parent directory if path is a file)
	checkPath := userProvided
	if !stat.IsDir() {
		checkPath = filepath.Dir(userProvided)
	}

	// Check both media path and Docker data root (if using Docker demucs)
	if err := fsutil.CheckDiskSpaceBoth(checkPath, requiredDiskSpaceGB, usingDockerVoiceSep, tsk.Handler.ZeroLog()); err != nil {
		return tsk.Handler.LogErr(err, AbortAllTasks, "")
	}

	if tsk.IsBulkProcess = stat.IsDir(); !tsk.IsBulkProcess {
		if ok := tsk.checkIntegrity(); ok  {
			tsk.Execute(ctx)
		}
	} else {
		var tasks []Task
		// initial scanning
		err = filepath.Walk(userProvided, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return tsk.Handler.LogErr(err, AbortAllTasks,
					"error during recursive exploration of provided directory").Err
			}
			if info.IsDir() && strings.HasSuffix(info.Name(), ".media") {
				return filepath.SkipDir
			}
			filename := filepath.Base(path)
			if !strings.HasSuffix(path, ".mp4") && !strings.HasSuffix(filename, ".mkv") ||
				isLangkitMadeMergedOutput(filename) {
				return nil
			}
			
			tsk.NativeSubFile = ""
			tsk.TargSubFile = ""
			tsk.MediaSourceFile = path
			if ok := tsk.checkIntegrity(); !ok  {
				return nil
			}
			
			if tsk.Mode != Enhance {
				if procErr := tsk.Autosub(); procErr != nil {
					return nil // don't return err, other files may be processable
				}
				foreignSubs, err := subs.OpenFile(tsk.TargSubFile, false)
				if err != nil {
					tsk.Handler.ZeroLog().Error().Err(err).Msg("can't read foreign subtitles")
				}
				// Use IsCCorDubs set by Autosub() - works for both standalone and embedded
				if tsk.IsCCorDubs {
					foreignSubs.TrimCC2Dubs()
				}
				// Filter ASS/SSA to Default-style only for accurate item count
				if isASSFormat(tsk.TargSubFile) {
					foreignSubs.FilterToDefaultStyle()
				}
				totalItems += len(foreignSubs.Items)
			}
			
			tasks = append(tasks, *tsk)
			return nil
		})
		if err != nil {
			return
		}
		reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
			es.BulkProcessingDir = userProvided
			es.ExpectedFileCount = len(tasks)
		}) // necessity: high
		
		// Check if this is a dry run and apply configuration
		if !tsk.Handler.IsCLI() {
			if config := GetCurrentDryRunConfig(); config != nil && config.Enabled {
				tsk.Handler.ZeroLog().Info().
					Int("fileCount", len(tasks)).
					Int("delayMs", config.DelayMs).
					Int("scheduledErrors", len(config.ErrorPoints)).
					Msg("Starting dry run bulk processing")
				
				// Apply dry run config to all tasks
				for i := range tasks {
					tasks[i].IsDryRun = true
					tasks[i].DryRunConfig = config
				}
			}
		}
		
		tsk.Handler.IncrementProgress(
			ProgressBarIDMedia,
			0,
			len(tasks),
			10,
			"Processing",
			"Total media files done...",
			"", // Use importance map for height class
		)
		for idx, tsk := range tasks {
			// Dry run mode - simulate processing without actual work
			if tsk.IsDryRun && tsk.DryRunConfig != nil && tsk.DryRunConfig.Enabled {
				config := tsk.DryRunConfig
				config.ProcessedCount = idx + 1
				
				// Check for scheduled errors at this index
				if errorType, exists := config.ErrorPoints[idx]; exists {
					if errorType == "abort_all" {
						return tsk.Handler.LogErr(fmt.Errorf("dry run test error"), AbortAllTasks, 
							fmt.Sprintf("DRY RUN: Critical error at file %d", idx+1))
					} else if errorType == "abort_task" {
						tsk.Handler.LogErr(fmt.Errorf("dry run test error"), AbortTask, 
							fmt.Sprintf("DRY RUN: Task error at file %d", idx+1))
					} else if errorType == "error" {
						// Regular error - just log it without any abort behavior
						tsk.Handler.ZeroLog().Error().
							Err(fmt.Errorf("dry run regular error")).
							Str("file", filepath.Base(tsk.MediaSourceFile)).
							Int("index", idx+1).
							Msg("DRY RUN: Regular error (no abort)")
					}
				}
				
				// Check for manual injection
				if config.NextErrorIndex == idx && config.NextErrorType != "" {
					if config.NextErrorType == "abort_all" {
						// Reset injection before returning
						config.NextErrorIndex = -1
						config.NextErrorType = ""
						return tsk.Handler.LogErr(fmt.Errorf("dry run manual error"), AbortAllTasks, 
							"DRY RUN: Manual critical error injection")
					} else if config.NextErrorType == "abort_task" {
						tsk.Handler.LogErr(fmt.Errorf("dry run manual error"), AbortTask, 
							"DRY RUN: Manual task error injection")
						// Reset injection
						config.NextErrorIndex = -1
						config.NextErrorType = ""
					} else if config.NextErrorType == "error" {
						// Regular error - just log it without any abort behavior
						tsk.Handler.ZeroLog().Error().
							Err(fmt.Errorf("dry run manual regular error")).
							Str("file", filepath.Base(tsk.MediaSourceFile)).
							Int("index", idx+1).
							Msg("DRY RUN: Manual regular error injection")
						// Reset injection
						config.NextErrorIndex = -1
						config.NextErrorType = ""
					}
				}
				
				// Simulate processing delay
				if config.DelayMs > 0 {
					time.Sleep(time.Duration(config.DelayMs) * time.Millisecond)
				}
				
				// Update progress with dry run indication
				tsk.Handler.IncrementProgress(
					ProgressBarIDMedia,
					1,
					len(tasks),
					10,
					"Dry Run Processing",
					fmt.Sprintf("Testing file %d of %d", idx+1, len(tasks)),
					"", // Use importance map for height class
				)
				
				// Log the file being "processed"
				tsk.Handler.ZeroLog().Info().
					Str("file", filepath.Base(tsk.MediaSourceFile)).
					Int("index", idx+1).
					Int("total", len(tasks)).
					Msg("DRY RUN: Simulating file processing")
				
				continue // Skip actual processing
			}
			reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
				es.CurrentFileIndex = idx
				es.CurrentFilePath = tsk.MediaSourceFile
				es.TotalFileCount = len(tasks)
			}) // necessity: high

			// Check disk space during processing and log warnings if low
			fsutil.LogDiskSpaceWarnings(userProvided, tsk.Handler.ZeroLog())

			// trick to have a new line without the log prefix
			tsk.Handler.ZeroLog().Info().Msg("\r             \n")//+mediabar.String())
			tsk.Handler.ZeroLog().Info().Msg("now: ." + strings.TrimPrefix(tsk.MediaSourceFile, userProvided))

			if err := tsk.Execute(ctx); err != nil {
				reporter.SaveSnapshot("Execute failed in bulk mode", tsk.DebugVals()) // necessity: high
				if errors.Is(err.Err, context.Canceled) {
					tsk.Handler.ZeroLog().Info().Msg("Processing canceled by the user")
				} else if errors.Is(err.Err, context.DeadlineExceeded) {
					tsk.Handler.ZeroLog().Warn().Msg("A timeout error occured")
				}
				
				tsk.Handler.ZeroLog().Debug().Msgf("Routing: behavior %s after error: %s\n",
					err.Behavior, err.Err)
				if err.Behavior == AbortTask {
					if idx != len(tasks)-1 {
						tsk.Handler.ZeroLog().Trace().Msg("AbortTask behavior: continuning to the next task planned...")
					}
					continue
				}
				tsk.Handler.ZeroLog().Debug().Msg("Aborting processing")
				
				// Ensure we shutdown resources even when aborting due to error
				if DefaultProviderManager != nil {
					tsk.Handler.ZeroLog().Info().Msg("Shutting down TranslitProviderManager after error")
					ShutdownTranslitService()
				}
				// Stop demucs container if it was started
				if err := voice.StopDemucsManager(); err != nil {
					tsk.Handler.ZeroLog().Warn().Err(err).Msg("Failed to stop Demucs container after error")
				}
				// Stop audio-separator container if it was started
				if err := voice.StopAudioSeparatorManager(); err != nil {
					tsk.Handler.ZeroLog().Warn().Err(err).Msg("Failed to stop audio-separator container after error")
				}

				return
			}
			tsk.Handler.IncrementProgress(
				ProgressBarIDMedia,
				1,
				len(tasks),
				10,
				"Processing",
				"Total media files done...",
				"", // Use importance map for height class
			)
		}
		
		// Shutdown provider manager after bulk processing is complete
		if DefaultProviderManager != nil {
			tsk.Handler.ZeroLog().Info().Msg("Shutting down TranslitProviderManager after bulk processing")
			ShutdownTranslitService()
		}
		// Stop demucs container after bulk processing is complete
		if err := voice.StopDemucsManager(); err != nil {
			tsk.Handler.ZeroLog().Warn().Err(err).Msg("Failed to stop Demucs container after bulk processing")
		}
		// Stop audio-separator container after bulk processing is complete
		if err := voice.StopAudioSeparatorManager(); err != nil {
			tsk.Handler.ZeroLog().Warn().Err(err).Msg("Failed to stop audio-separator container after bulk processing")
		}
	}
	tsk.Handler.ZeroLog().Debug().Msg("Routing completed")
	return
}


func (tsk *Task) checkIntegrity() bool {
	isCorrupted, err := media.CheckValidData(tsk.MediaSourceFile)
	l := tsk.Handler.ZeroLog().Error().Err(err).Str("video", tsk.MediaSourceFile)
	if isCorrupted {
		l.Msg("Invalid data found when processing video. Video is misformed or corrupted.")
	} else if err != nil {
		l.Msg("unspecified error found trying to check the video's integrity")
	}
	return !isCorrupted
}

// i is the total sum
func mkMediabar(i int) *progressbar.ProgressBar {
	return progressbar.NewOptions(i,
		progressbar.OptionSetDescription("Processing videos..."),
		progressbar.OptionShowCount(),
		//progressbar.OptionUseANSICodes(false),
		//progressbar.OptionSetRenderBlankState(true),
		//progressbar.OptionSetVisibility(false),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionSetWriter(io.Discard),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "#",
			SaucerPadding: "-",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

// // for debugging GUI bars  
// func waitRandomWithContext(ctx context.Context) error {
// 	rand.Seed(time.Now().UnixNano())
// 	waitTime := time.Duration(3+rand.Float64()*2) * time.Second

// 	fmt.Printf("Waiting for %v...\n", waitTime)

// 	// Create a timer that will fire after waitTime
// 	timer := time.NewTimer(waitTime)
// 	defer timer.Stop() // Ensure the timer is cleaned up

// 	select {
// 	case <-timer.C: // Timer expired, meaning the wait is complete
// 		fmt.Println("Done waiting!")
// 		return nil
// 	case <-ctx.Done(): // Context was canceled
// 		fmt.Println("Context canceled before timeout:", ctx.Err())
// 		return ctx.Err()
// 	}
// }


func placeholder23456345467() {
	fmt.Println("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}


			/* Dry run testing is now implemented properly via the DryRunConfig system.
			   Use the DevDashboard > Tests > Bulk Processing Dry Run Test to configure and run tests. */