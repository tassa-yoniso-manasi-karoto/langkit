package core

import (
	"os"
	"sync"
	"context"
	"strings"

	astisub "github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/schollz/progressbar/v3"
)

// IndexedSubItem wraps a subtitle item along with its original index.
type IndexedSubItem struct {
	Index int
	Item  *astisub.Item
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
		skippedIndexes = make(map[int]bool)
	)

	toCheckChan := make(chan string)
	isAlreadyChan := make(chan bool)
	go func() {
		if err := checkStringsInFile(tsk.outputFile(), toCheckChan, isAlreadyChan); err != nil {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("Error checking strings in file")
		}
	}()
	
	// init progress  bar
	tsk.Handler.IncrementProgress(
		"item-bar",
		0,
		totalItems,
		20,
		"Subtitle lines processed (all files)...",
		tsk.descrBar(),
		"h-3",
	)
	updateBar := func(item *ProcessedItem) {
		if !item.AlreadyDone {
			tsk.Handler.IncrementProgress(
				"item-bar",
				1,
				totalItems,
				20,
				"Subtitle lines processed (all files)...",
				tsk.descrBar(),
				"h-3",
			)
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
		for i, astitem := range tsk.TargSubs.Items {
			searchString := tsk.FieldSep + timePosition(astitem.StartAt) + tsk.FieldSep
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
				// If the item was already processed, mark it as skipped
				tsk.Handler.ZeroLog().Trace().
					Int("idx", i).
					Str("subline", getSubLineText(*astitem)).
					Msg("Skipping subtitle line previously processed")
				skipped++
				skippedIndexes[i] = true
				totalItems--
				continue
			}
			
			// Otherwise, dispatch this item for processing
			select {
			case <-supCtx.Done():
				return
			case subLineChan <- IndexedSubItem{Index: i, Item: astitem}:
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
		
		waitingRoom := make(map[int]ProcessedItem)
		nextIndex := 0
		
		for {
			// Skip any consecutive indexes that were already processed (skipped).
			for skippedIndexes[nextIndex] {
				tsk.Handler.ZeroLog().Trace().
					Int("idx", nextIndex).
					Msg("writer: item exist in file already, skipping...")
				// We pretend that item was “written,” so we just jump ahead.
				nextIndex++
			}
			
			if item, exists := waitingRoom[nextIndex]; exists {
				tsk.Handler.ZeroLog().Debug().
					Int("idx", item.Index).
					Msg("writer: item is already in waitingRoom")
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
					for {
						// Skip any future indexes that we know were processed
						for skippedIndexes[nextIndex] {
							tsk.Handler.ZeroLog().Trace().
								Int("idx", nextIndex).
								Msg("writer: item exist in file already, skipping...")
							nextIndex++
						}
						
						if nextItem, exists := waitingRoom[nextIndex]; exists {
							tsk.Handler.ZeroLog().Trace().
								Int("idx", nextItem.Index).
								Msg("writer: no more items to come: flushing waitingRoom in order")
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
					tsk.Handler.ZeroLog().Trace().
						Int("idx", nextIndex).
						Msg("writer: just received the correct next item")
					write(outStream, &item)
					updateBar(&item)
					nextIndex++
					for {
						// Again, skip any that were marked as already processed.
						for skippedIndexes[nextIndex] {
							tsk.Handler.ZeroLog().Trace().
								Int("idx", nextIndex).
								Msg("writer: item exist in file already, skipping...")
							nextIndex++
						}
						if nextItem, exists := waitingRoom[nextIndex]; exists {
							tsk.Handler.ZeroLog().Trace().
								Int("idx", nextItem.Index).
								Msg("writer: SUBSEQUENT item is already in waitingRoom")
							write(outStream, &nextItem)
							updateBar(&nextItem)
							delete(waitingRoom, nextIndex)
							nextIndex++
						} else {
							break
						}
					}
				} else {
					tsk.Handler.ZeroLog().Trace().
						Int("idx", item.Index).
						Msg("writer: STORING in waitingRoom out-of-order item")
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
		return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "supervisor: operation canceled by user")
	}
	if tsk.WantCondensedAudio {
		tsk.ConcatWAVstoOGG("CONDENSED") // TODO probably better to put it elsewhere
	}
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
			item, procErr := tsk.ProcessItem(cfg.ctx, indexedSub)
			if procErr != nil {
				// Try to send error, but don't block if canceled
				select {
				case cfg.errChan <- procErr:
				case <-cfg.ctx.Done():
				}
				return
			}
			// Stamp the processed item with its original index.
			item.Index = indexedSub.Index
			// Try to send result, but don't block if canceled
			select {
			case <-cfg.ctx.Done():
				return
			case cfg.itemChan <- item:
				//tsk.Handler.ZeroLog().Trace().
				//	Int("workerID", cfg.id).
				//	Int("lenItemChan", len(cfg.itemChan)).
				//	Msg("Item successfully sent")
			}
		}
	}
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


func (tsk *Task) descrBar() string {
	if tsk.IsBulkProcess {
		return "Bulk"
	}
	return "Items"
}

func getSubLineText(subLine astisub.Item) string {
	if len(subLine.Lines) == 0 {
		return ""
	}
	if len(subLine.Lines[0].Items) == 0 {
		return ""
	}
	return subLine.Lines[0].Items[0].Text
}


func placeholder4234565() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

