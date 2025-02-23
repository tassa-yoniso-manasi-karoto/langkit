package core

import (
	"os"
	"strings"
	"path/filepath"
	"fmt"
	"io"
	"context"
	"errors"
	
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/schollz/progressbar/v3"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)


func (tsk *Task) Routing(ctx context.Context) (procErr *ProcessingError) {
	version, err := media.GetFFmpegVersion()
	if err != nil {
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
		// NOTE: these two loggers are equivalent: they would both log to STDERR
		// and to the GUI (if applicable). The only difference is that
		// Log[Err][Fields]() returns a ProcessingError that can be used
		// to define an error handling strategy. Also, it can be a bit more concise.
		return tsk.Handler.LogErr(err, AbortAllTasks, "can't access passed media file/directory")
		//tsk.Handler.ZeroLog().Error().
		//	Err(err).Str("behavior", AbortAllTasks).
		//	Msg("can't access passed media file/directory")
	}
	if tsk.IsBulkProcess = stat.IsDir(); !tsk.IsBulkProcess {
		if ok := tsk.checkIntegrity(); ok  {
			tsk.Execute(ctx)
		}
	} else {
		var tasks []Task
		err = filepath.Walk(userProvided, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return tsk.Handler.LogErr(err, AbortAllTasks,
					"error during recursive exploration of provided directory").Err
			}
			if info.IsDir() && strings.HasSuffix(info.Name(), ".media") {
				return filepath.SkipDir
			}
			filename := filepath.Base(path)
			if !strings.HasSuffix(path, ".mp4") && !strings.HasSuffix(filename, ".mkv")  {
				return nil
			}
			tsk.NativeSubFile = ""
			tsk.TargSubFile = ""
			tsk.MediaSourceFile = path
			if ok := tsk.checkIntegrity(); !ok  {
				return nil
			}
			tsk.Autosub()
			foreignSubs, err := subs.OpenFile(tsk.TargSubFile, false)
			if err != nil {
				tsk.Handler.ZeroLog().Error().Err(err).Msg("can't read foreign subtitles")
			}
			if strings.Contains(strings.ToLower(tsk.TargSubFile), "closedcaption") { //TODO D.R.Y. cards.go#L162
				foreignSubs.TrimCC2Dubs()
			}
			totalItems += len(foreignSubs.Items)
			tasks = append(tasks, *tsk)
			return nil
		})
		if err != nil {
			return
		}
		mediabar := mkMediabar(len(tasks))
		for idx, tsk := range tasks {
			mediabar.Add(1)
			// trick to have a new line without the log prefix
			tsk.Handler.ZeroLog().Info().Msg("\r             \n"+mediabar.String())
			tsk.Handler.ZeroLog().Info().Msg("now: ." + strings.TrimPrefix(tsk.MediaSourceFile, userProvided))
			// Update progress for file start
			// 	a.updateProgress(ProgressUpdate{
			// 		Progress:    float64(i) / float64(totalFiles) * 100,
			// 		Current:     i + 1,
			// 		Total:      totalFiles,
			// 		CurrentFile: file,
			// 		Operation:   string(task.Mode),
			// 	})
			
			if err := tsk.Execute(ctx); err != nil {
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
				return
			}
		}
	}
	tsk.Handler.ZeroLog().Debug().Msg("Routing completed successfully")
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

func mkItemBar(i int, descr string) *progressbar.ProgressBar {
	return progressbar.NewOptions(i,
		progressbar.OptionSetDescription(descr),
		progressbar.OptionShowCount(),		
		progressbar.OptionSetWidth(31),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "#",
			SaucerPadding: "-",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}





func placeholder23456345467() {
	fmt.Println("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
