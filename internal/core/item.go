package core

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"path/filepath"
	"sync"
	"sort"
	"io/fs"
	"errors"
	"context"

	astisub "github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)

// ProcessedItem represents the exported information of a single subtitle item,
// where Time is the primary field which identifies the item and ForeignCurr is
// the actual text of the item. The fields NativeCurr, NativePrev and NativeNext
// will be empty unless a second subtitle file was specified for the export and
// that second subtitle file is sufficiently aligned with the first.
type ProcessedItem struct {
	ID          time.Duration
	AlreadyDone bool
	Sound       string
	Time        string
	Source      string
	Image       string
	ForeignCurr string
	NativeCurr  string
	ForeignPrev string
	NativePrev  string
	ForeignNext string
	NativeNext  string
}

// ProcessedItemWriter should write an exported item in whatever format is // selected by the user.
type ProcessedItemWriter func(*os.File, *ProcessedItem)

// Supervisor manages multiple workers processing subtitle items concurrently
// It handles both GUI-triggered cancellation and internal processing errors
func (tsk *Task) Supervisor(ctx context.Context, outStream *os.File, write ProcessedItemWriter) *ProcessingError {
	var (
		subLineChan = make(chan *astisub.Item)                    // Channel for distributing work to workers
		itemChan    = make(chan ProcessedItem, len(tsk.TargSubs.Items)) // Buffered channel for processed items
		errChan     = make(chan *ProcessingError)                 // Channel for worker errors
		items       []ProcessedItem                               // Slice to store results for sorting
		wg          sync.WaitGroup                                // WaitGroup to track active workers
		skipped     int
	)

	// Create supervisor's own cancellation context as child of parent
	// This allows cancellation either from parent (GUI) or locally (processing error)
	supCtx, supCancel := context.WithCancel(ctx)
	defer supCancel() // Ensure cleanup when Supervisor exits

	tsk.Handler.ZeroLog().Debug().
		Int("capItemChan", len(tsk.TargSubs.Items)).
		Int("workersMax", tsk.Meta.WorkersMax).
		Msg("Supervisor initialized, starting workers")

	// Start worker pool
	for i := 1; i <= tsk.Meta.WorkersMax; i++ {
		wg.Add(1)
		go tsk.worker(WorkerConfig{
			ctx:         supCtx,
			id:          i,
			subLineChan: subLineChan,
			itemChan:    itemChan,
			errChan:     errChan,
			wg:          &wg,
		})
	}
	//go checkStringsInFile(tsk.outputFile(), toCheckChan, isAlreadyChan)

	// Producer goroutine
	go func() {
		defer close(subLineChan) // Ensure channel closes when all work is distributed
		for _, subLine := range tsk.TargSubs.Items {
			// FIXME: Right now, due to parrallelism, writing is only done when all workers are done
			// so this kind of check will only work on completed tasks, therefore there is no resuming capability
			// therefore ASR/TTS already done is lost upon interruption.
			// The obvious way to fix this is to write another goroutine that sorts and checks the ID of items and
			// write them on-the-fly in order when the next ID expected is available, keeping that for some other time...
			/*if toCheckChan <- tsk.FieldSep + timePosition(subLine.StartAt) + tsk.FieldSep; <-isAlreadyChan {
				tsk.Handler.ZeroLog().Trace().Int("lineNum", i).Msg("Skipping subtitle line previously processed")
				skipped += 1
				totalItems -= 1
				continue
			}*/
			select {
			case <-supCtx.Done():
				// Exit if cancelled
				return
			case subLineChan <- subLine:
				// Send work to workers
			}
		}
		if skipped != 0 {
			tsk.Handler.ZeroLog().Info().Msgf("%.1f%% of items were already done and skipped (%d/%d)",
					float64(skipped)/float64(len(tsk.TargSubs.Items))*100, skipped, len(tsk.TargSubs.Items))
		} else {
			tsk.Handler.ZeroLog().Debug().Msg("No line previously processed was found")
		}
	}()

	// Closer goroutine
	go func() {
		tsk.Handler.ZeroLog().Debug().
			Int("lenItemChan", len(itemChan)).
			Int("capItemChan", len(tsk.TargSubs.Items)).
			Int("lenSubLineChan", len(subLineChan)).
			Msg("Waiting for workers to finish.")
		wg.Wait()
		// Close channels to signal completion
		close(itemChan)
		close(errChan)
		//close(toCheckChan)
		//close(isAlreadyChan)
	}()

	// Main processing loop - handles results and errors
	for {
		select {
		case <-ctx.Done():
			// Parent (GUI) triggered cancellation
			return tsk.Handler.Log(Debug, AbortAllTasks, "operation cancelled by user")
			
		case item, ok := <-itemChan:
			if !ok {
				// itemChan closed = all workers finished
				goto ProcessResults
			}
			// Store result and update progress
			items = append(items, item)
			if !item.AlreadyDone {
				if itembar == nil {
					itembar = mkItemBar(totalItems, tsk.descrBar())
					// in case: some encoding was done with incorrect settings,
					// user deleted .media directory to redo but in the os.Walk order,
					// there is some already processed media between that deleted dir
					// and the one that remain to do. Hence need to update bar to count these out.
				} else if itembar.GetMax() != totalItems {
					itembar.ChangeMax(totalItems)
				}
				itembar.Add(1)
			}

		case procErr, ok := <-errChan:
			if !ok {
				continue
			}
			// note: supCancel() was already deferred earlier
			tsk.Handler.ZeroLog().Error().
				Err(procErr.Err).
				Msg("Processing error occurred, cancelling all workers")
			return procErr
		}
	}

ProcessResults:
	if itembar != nil {
		itembar.Clear()
	}

	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	for _, item := range items {
		write(outStream, &item)
	}

	tsk.ConcatWAVstoOGG("CONDENSED") // TODO probably better to put it elsewhere
	return nil
}

type WorkerConfig struct {
	ctx         context.Context
	id          int
	subLineChan <-chan *astisub.Item
	itemChan    chan ProcessedItem
	errChan     chan *ProcessingError
	wg          *sync.WaitGroup
}

func (tsk *Task) worker(cfg WorkerConfig) {
	defer tsk.Handler.ZeroLog().Trace().
		Int("workerID", cfg.id).
		Int("lenSubLineChan", len(cfg.subLineChan)).
		Msg("Terminating worker")
	defer cfg.wg.Done()

	for {
		select {
		case <-cfg.ctx.Done():
			return
		case subLine, ok := <-cfg.subLineChan:
			if !ok {
				// No more work to do
				return
			}
			item, procErr := tsk.ProcessItem(cfg.ctx, subLine)
			if procErr != nil {
				// Try to send error, but don't block if cancelled
				select {
				case cfg.errChan <- procErr:
				case <-cfg.ctx.Done():
				}
				return
			}
			// Try to send result, but don't block if cancelled
			select {
			case <-cfg.ctx.Done():
				return
			case cfg.itemChan <- item:
				tsk.Handler.ZeroLog().Trace().
					Int("workerID", cfg.id).
					Int("lenItemChan", len(cfg.itemChan)).
					Msg("Item successfully sent")
			}
		}
	}
}


func (tsk *Task) ProcessItem(ctx context.Context, foreignItem *astisub.Item) (item ProcessedItem, procErr *ProcessingError) {
	item.Source = tsk.outputBase()
	item.ForeignCurr = joinLines(foreignItem.String())

	if tsk.NativeSubs != nil {
		if nativeItem := tsk.NativeSubs.Translate(foreignItem); nativeItem != nil {
			item.NativeCurr = joinLines(nativeItem.String())
		}
	}
	audiofile, err := media.ExtractAudio("ogg", tsk.UseAudiotrack,
		tsk.Offset, foreignItem.StartAt, foreignItem.EndAt,
			tsk.MediaSourceFile, tsk.MediaPrefix, false)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("can't extract ogg audio")
	}
	if !tsk.DubsOnly {
		_, err = media.ExtractAudio("wav", tsk.UseAudiotrack,
			time.Duration(0), foreignItem.StartAt, foreignItem.EndAt,
				tsk.MediaSourceFile, tsk.MediaPrefix, false)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("can't extract wav audio")
		}
	}
	imageFile, err := media.ExtractImage(foreignItem.StartAt, foreignItem.EndAt,
		tsk.MediaSourceFile, tsk.MediaPrefix, tsk.DubsOnly)
	if err != nil {
		// check done on the AVIF because it is the most computing intensive
		if errors.Is(err, fs.ErrExist) {
			item.AlreadyDone = true
			totalItems -= 1
		} else {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("can't extract image")
		}
	}
	item.ID = foreignItem.StartAt
	item.Time = timePosition(foreignItem.StartAt)
	item.Image = fmt.Sprintf("<img src=\"%s\">", path.Base(imageFile))
	item.Sound = fmt.Sprintf("[sound:%s]", path.Base(audiofile))
	
	lang := tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language
	dub := ""
	switch tsk.STT {
	case "whisper":
		dub, err = voice.Whisper(ctx, audiofile, 5, tsk.TimeoutSTT, lang.Part1, "")
	case "insanely-fast-whisper":
		dub, err = voice.InsanelyFastWhisper(ctx, audiofile, 5, tsk.TimeoutSTT, lang.Part1)
	case "universal-1":
		dub, err = voice.Universal1(ctx, audiofile, 5, tsk.TimeoutSTT, lang.Part1)
	}
	item.ForeignCurr = dub
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return item, tsk.Handler.Log(Debug, AbortAllTasks, "STT: Operation cancelled due to context cancellation.")
		} else if errors.Is(err, context.DeadlineExceeded) {
			return item, tsk.Handler.LogErr(err, AbortTask, "STT: Operation timed out.")
		}
		return item, tsk.Handler.LogErrFields(err, AbortTask, tsk.STT + " error",
			map[string]interface{}{"item": foreignItem.String()})
	}
	/*if i > 0 { // FIXME this has never worked for some reason
		prevItem := tsk.TargSubs.Items[i-1]
		item.ForeignPrev = prevItem.String()
	}

	if i+1 < len(tsk.TargSubs.Items) {
		nextItem := tsk.TargSubs.Items[i+1]
		item.ForeignNext = nextItem.String()
	}*/
	return
}





func (tsk *Task) ConcatWAVstoOGG(suffix string) {
	out := fmt.Sprint(tsk.MediaPrefix, ".", suffix,".ogg")
	if  _, err := os.Stat(out); err == nil {
		return
	}
	wavFiles, err := filepath.Glob(tsk.MediaPrefix+ "_*.wav")
	if err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("Error searching for .wav files")
	}

	if len(wavFiles) == 0 {
		tsk.Handler.ZeroLog().Warn().
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("No .wav files found")
	}
	// Generate the concat list for ffmpeg
	concatFile, err := media.CreateConcatFile(wavFiles)
	if err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Error creating temporary concat file")
	}
	defer os.Remove(concatFile)

	// Run FFmpeg to concatenate and create the audio file
	media.RunFFmpegConcat(concatFile, tsk.MediaPrefix+".wav")

	// Convert WAV to OPUS using FFmpeg
	media.RunFFmpegConvert(tsk.MediaPrefix+".wav", out)
	// Clean up
	os.Remove(tsk.MediaPrefix+".wav")
	for _, f := range wavFiles {
		if err := os.Remove(f); err != nil {
			tsk.Handler.ZeroLog().Warn().Str("file", f).Msg("Removing file failed")
		}
	}
}


func (tsk *Task) descrBar() string {
	if tsk.IsBulkProcess {
		return "Bulk"
	}
	return "Items"
}


// Function that runs in a goroutine to check multiple strings in a file
func checkStringsInFile(filepath string, toCheckChan <-chan string, isAlreadyChan chan<- bool) error {
	content, _ := os.ReadFile(filepath)
	fileContent := string(content)
	for searchString := range toCheckChan {
		if len(content) == 0 {
			//color.Redln("len(content) == 0, ASSUMING "+searchString+" IT IS NOT IN FILE "+filepath)
			isAlreadyChan <- false
		} else {
			//color.Redln("SEARCHING "+searchString+"IN FILE")
			isAlreadyChan <- strings.Contains(fileContent, searchString)
		}
	}
	return nil
}

// timePosition formats the given time.Duration as a time code which can safely
// be used in file names on all platforms.
func timePosition(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func joinLines(s string) string {
	s = strings.Replace(s, "\t", " ", -1)
	return strings.Replace(s, "\n", " ", -1)
}

func IsZeroLengthTimespan(last, t time.Duration) (b bool) {
	if t - last == 0 {
		b = true
	}
	return
}



func placeholder4() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}


