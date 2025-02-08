package core

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"path/filepath"
	"sync"
	"io/fs"
	"errors"
	"context"

	astisub "github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)

// IndexedSubItem wraps a subtitle item along with its original index.
type IndexedSubItem struct {
	Index int
	Item  *astisub.Item
}

// ProcessedItem represents the exported information of a single subtitle item.
type ProcessedItem struct {
	Index       int
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

type ProcessedItemWriter func(*os.File, *ProcessedItem)

// Supervisor manages multiple workers processing subtitle items concurrently.
// It handles cancellation, error reporting, duplicate checking for resuming a previous aborted run, and on‑the‑fly writing.
func (tsk *Task) Supervisor(ctx context.Context, outStream *os.File, write ProcessedItemWriter) *ProcessingError {
	var (
		subLineChan = make(chan IndexedSubItem)				// Channel for distributing work (each item tagged with its index).
		itemChan    = make(chan ProcessedItem, len(tsk.TargSubs.Items))	// Buffered channel for processed items.
		errChan     = make(chan *ProcessingError, tsk.Meta.WorkersMax)	// Channel for worker errors.
		wg          sync.WaitGroup
		skipped     int
	)

	toCheckChan := make(chan string)
	isAlreadyChan := make(chan bool)
	go func() {
		if err := checkStringsInFile(tsk.outputFile(), toCheckChan, isAlreadyChan); err != nil {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("Error checking strings in file")
		}
	}()

	updateBar := func(item *ProcessedItem) {
		// Only update progress for items that were not already done.
		if !item.AlreadyDone {
			if itembar == nil {
				itembar = mkItemBar(totalItems, tsk.descrBar())
			} else if itembar.GetMax() != totalItems {
				itembar.ChangeMax(totalItems)
			}
			itembar.Add(1)
		}
	}

	supCtx, supCancel := context.WithCancel(ctx)
	defer supCancel()

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

	// Producer goroutine
	go func() {
		// Ensure both work and duplicate-check channels are closed when done.
		defer close(subLineChan)
		defer close(toCheckChan)
		for i, subLine := range tsk.TargSubs.Items {
			searchString := tsk.FieldSep + timePosition(subLine.StartAt) + tsk.FieldSep
			// Send the search string to check for duplicates.
			select {
			case <-supCtx.Done():
				return
			case toCheckChan <- searchString:
			}
			
			// Wait for the result.
			var already bool
			select {
			case <-supCtx.Done():
				return
			case already = <-isAlreadyChan:
			}
			
			if already {
				tsk.Handler.ZeroLog().Trace().
					Int("lineNum", i).
					Msg("Skipping subtitle line previously processed")
				skipped++
				totalItems--
				continue
			}
			
			// Then dispatch work
			select {
			case <-supCtx.Done():
				return
			case subLineChan <- IndexedSubItem{Index: i, Item: subLine}:
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
		close(itemChan)
		close(errChan)
	}()

	// Error Handling (for all workers)
	var finalErr *ProcessingError
	var errOnce sync.Once
	go func() {
		if procErr, ok := <-errChan; ok {
			errOnce.Do(func() {
				finalErr = procErr
				supCancel()
			})
		}
	}()

	// Writer Goroutine
	// This goroutine receives processed items, buffers out-of-order items,
	// writes them in order, and updates the progress bar.
	var writerWG sync.WaitGroup
	writerWG.Add(1)
	go func() {
		defer writerWG.Done()
		// waitingRoom holds processed items that have arrived out-of-order.
		waitingRoom := make(map[int]ProcessedItem)
		nextIndex := 0
		for {
			// If the next expected item is already in waitingRoom, write it.
			if item, exists := waitingRoom[nextIndex]; exists {
				write(outStream, &item)
				updateBar(&item)
				delete(waitingRoom, nextIndex)
				nextIndex++
				continue
			}
			// Otherwise, read a new processed item.
			select {
			case <-supCtx.Done():
				return
			case item, ok := <-itemChan:
				if !ok {
					// No more items; flush any remaining in-order items.
					for {
						if nextItem, exists := waitingRoom[nextIndex]; exists {
							write(outStream, &nextItem)
							updateBar(&nextItem)
							delete(waitingRoom, nextIndex)
							nextIndex++
						} else {
							return
						}
					}
				}
				if item.Index == nextIndex {
					// Write the expected item immediately.
					write(outStream, &item)
					updateBar(&item)
					nextIndex++
					// Check if subsequent items are already waitingRoom.
					for {
						if nextItem, exists := waitingRoom[nextIndex]; exists {
							write(outStream, &nextItem)
							updateBar(&nextItem)
							delete(waitingRoom, nextIndex)
							nextIndex++
						} else {
							break
						}
					}
				} else {
					// Item arrived out-of-order; store it for later.
					waitingRoom[item.Index] = item
				}
			}
		}
	}()

	writerWG.Wait()

	if itembar != nil {
		itembar.Clear()
	}

	if finalErr != nil {
		tsk.Handler.ZeroLog().Error().
			Err(finalErr.Err).
			Msg("Processing error occurred, cancelling all workers")
		return finalErr
	}
	if ctx.Err() != nil {
		return tsk.Handler.Log(Debug, AbortAllTasks, "operation cancelled by user")
	}

	tsk.ConcatWAVstoOGG("CONDENSED") // TODO probably better to put it elsewhere
	return nil
}

type WorkerConfig struct {
	ctx         context.Context
	id          int
	subLineChan <-chan IndexedSubItem
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
	tsk.Handler.ZeroLog().Trace().
		Int("workerID", cfg.id).
		Msg("Starting worker")

	for {
		select {
		case <-cfg.ctx.Done():
			return
		case indexedSub, ok := <-cfg.subLineChan:
			if !ok {
				// No more work.
				return
			}
			item, procErr := tsk.ProcessItem(cfg.ctx, indexedSub.Item)
			if procErr != nil {
				// Try to send error, but don't block if cancelled
				select {
				case cfg.errChan <- procErr:
				case <-cfg.ctx.Done():
				}
				return
			}
			// Stamp the processed item with its original index.
			item.Index = indexedSub.Index
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


// checkStringsInFile runs in its own goroutine and checks multiple strings in a file.
// For each string sent down toCheckChan, it sends a bool on isAlreadyChan indicating
// whether that string is found in the file (which is read once at startup).
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


