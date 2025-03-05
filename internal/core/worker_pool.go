package core

import (
	"context"
	"os"
	"sync"

	"github.com/asticode/go-astisub"
)


// DefaultWorkerPool implements the WorkerPool interface for processing subtitle items concurrently
type DefaultWorkerPool struct {
	// Configuration
	task          TaskInterface
	maxWorkers    int
	handler       MessageHandler
	progressTracker ProgressTracker
	
	// Resumption
	resumptionService ResumptionService
	
	// Output
	outputFile    *os.File
	itemWriter    ProcessedItemWriter
}

// NewWorkerPool creates a new DefaultWorkerPool
func NewWorkerPool(
	task TaskInterface,
	maxWorkers int,
	handler MessageHandler,
	resumptionService ResumptionService,
	progressTracker ProgressTracker,
) WorkerPool {
	return &DefaultWorkerPool{
		task:             task,
		maxWorkers:       maxWorkers,
		handler:          handler,
		resumptionService: resumptionService,
		progressTracker:  progressTracker,
	}
}

// Submit adds a subtitle item to the processing queue (implementation detail, not exposed)
func (p *DefaultWorkerPool) Submit(item IndexedSubItem) error {
	// This is handled internally by ProcessItems
	return nil
}

// Shutdown gracefully shuts down the worker pool
func (p *DefaultWorkerPool) Shutdown() error {
	// No specific shutdown needed for this implementation
	return nil
}

// ProcessItems processes a batch of subtitle items concurrently
// This is an abstracted, testable version of the Supervisor function
func (p *DefaultWorkerPool) ProcessItems(ctx context.Context, items []*astisub.Item) ([]ProcessedItem, error) {
	var (
		subLineChan = make(chan IndexedSubItem)                // Channel for distributing work
		itemChan    = make(chan ProcessedItem, len(items))    // Buffered channel for processed items
		errChan     = make(chan *ProcessingError, p.maxWorkers) // Channel for worker errors
		wg          sync.WaitGroup
		skipped     int
		skippedIndexes = make(map[int]bool)
		totalItems = len(items)
		processedItems = make([]ProcessedItem, 0, totalItems)
	)

	// Setup resumption service if available
	var toCheckChan chan string
	var isAlreadyChan chan bool
	
	if p.resumptionService != nil {
		toCheckChan = make(chan string)
		isAlreadyChan = make(chan bool)
		
		go func() {
			for searchString := range toCheckChan {
				alreadyProcessed, _ := p.resumptionService.IsAlreadyProcessed(searchString)
				isAlreadyChan <- alreadyProcessed
			}
		}()
	}

	// Progress update function
	updateProgress := func(item *ProcessedItem) {
		if p.progressTracker != nil && !item.AlreadyDone {
			p.progressTracker.UpdateProgress(item.Index+1, totalItems, "Processing subtitle items")
		}
	}

	// Create a cancellable context
	poolCtx, poolCancel := context.WithCancel(ctx)
	defer poolCancel()

	p.handler.ZeroLog().Debug().
		Int("totalItems", totalItems).
		Int("maxWorkers", p.maxWorkers).
		Msg("WorkerPool initialized, starting workers")

	// Start worker pool
	for i := 1; i <= p.maxWorkers; i++ {
		wg.Add(1)
		go p.startWorker(WorkerConfig{
			ctx:         poolCtx,
			id:          i,
			subLineChan: subLineChan,
			itemChan:    itemChan,
			errChan:     errChan,
			wg:          &wg,
		})
	}

	// Producer goroutine
	go func() {
		defer close(subLineChan)
		if toCheckChan != nil {
			defer close(toCheckChan)
		}
		
		for i, astitem := range items {
			if p.resumptionService != nil {
				searchString := timePosition(astitem.StartAt)
				
				// Send the search string to check for duplicates
				select {
				case <-poolCtx.Done():
					return
				case toCheckChan <- searchString:
				}
				
				// Wait for the result
				var already bool
				select {
				case <-poolCtx.Done():
					return
				case already = <-isAlreadyChan:
				}
				
				if already {
					// Skip already processed items
					p.handler.ZeroLog().Trace().
						Int("idx", i).
						Str("text", getSubLineText(*astitem)).
						Msg("Skipping previously processed subtitle line")
					skipped++
					skippedIndexes[i] = true
					continue
				}
			}
			
			// Dispatch this item for processing
			select {
			case <-poolCtx.Done():
				return
			case subLineChan <- IndexedSubItem{Index: i, Item: astitem}:
			}
		}
		
		if skipped > 0 {
			p.handler.ZeroLog().Info().Msgf("%.1f%% of items were already done and skipped (%d/%d)",
				float64(skipped)/float64(len(items))*100, skipped, len(items))
		}
	}()

	// Closer goroutine
	go func() {
		wg.Wait()
		close(itemChan)
		close(errChan)
	}()

	// Error handling
	var finalErr *ProcessingError
	var errOnce sync.Once
	go func() {
		if procErr, ok := <-errChan; ok {
			errOnce.Do(func() {
				finalErr = procErr
				poolCancel()
			})
		}
	}()

	// Collector goroutine - collects results into the result slice
	resultWg := sync.WaitGroup{}
	resultWg.Add(1)
	go func() {
		defer resultWg.Done()
		
		waitingRoom := make(map[int]ProcessedItem)
		nextIndex := 0
		
		for {
			// Skip any indexes that were already processed
			for skippedIndexes[nextIndex] {
				nextIndex++
			}
			
			// Check if we can get an item from the waiting room
			if item, exists := waitingRoom[nextIndex]; exists {
				processedItems = append(processedItems, item)
				updateProgress(&item)
				delete(waitingRoom, nextIndex)
				nextIndex++
				continue
			}
			
			// Otherwise, wait for a new processed item
			select {
			case <-poolCtx.Done():
				return
			case item, ok := <-itemChan:
				if !ok {
					// Process any remaining items in the waiting room in order
					for {
						// Skip any indexes that were already processed
						for skippedIndexes[nextIndex] {
							nextIndex++
						}
						
						if nextItem, exists := waitingRoom[nextIndex]; exists {
							processedItems = append(processedItems, nextItem)
							updateProgress(&nextItem)
							delete(waitingRoom, nextIndex)
							nextIndex++
						} else {
							return
						}
					}
				}
				
				if item.Index == nextIndex {
					// If this is the next item in sequence, add it directly
					processedItems = append(processedItems, item)
					updateProgress(&item)
					nextIndex++
					
					// Check if we can now process items from the waiting room
					for {
						// Skip any indexes that were already processed
						for skippedIndexes[nextIndex] {
							nextIndex++
						}
						
						if nextItem, exists := waitingRoom[nextIndex]; exists {
							processedItems = append(processedItems, nextItem)
							updateProgress(&nextItem)
							delete(waitingRoom, nextIndex)
							nextIndex++
						} else {
							break
						}
					}
				} else {
					// Store out-of-order items in the waiting room
					waitingRoom[item.Index] = item
				}
			}
		}
	}()

	// Wait for all results to be collected
	resultWg.Wait()

	if finalErr != nil {
		p.handler.ZeroLog().Error().
			Err(finalErr.Err).
			Msg("Processing error occurred, cancelling all workers")
		return nil, finalErr.Err
	}
	
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	
	return processedItems, nil
}

// startWorker starts a worker goroutine for processing subtitle items
func (p *DefaultWorkerPool) startWorker(cfg WorkerConfig) {
	defer p.handler.ZeroLog().Trace().
		Int("workerID", cfg.id).
		Msg("Terminating worker")
	defer cfg.wg.Done()
	
	p.handler.ZeroLog().Trace().
		Int("workerID", cfg.id).
		Msg("Starting worker")

	for {
		select {
		case <-cfg.ctx.Done():
			return
		case indexedSub, ok := <-cfg.subLineChan:
			if !ok {
				// No more work
				return
			}
			
			// Process the item
			item, procErr := p.task.ProcessItem(cfg.ctx, indexedSub)
			if procErr != nil {
				// Try to send error, but don't block if canceled
				select {
				case cfg.errChan <- procErr:
				case <-cfg.ctx.Done():
				}
				return
			}
			
			// Set the index
			item.Index = indexedSub.Index
			
			// Try to send result, but don't block if canceled
			select {
			case <-cfg.ctx.Done():
				return
			case cfg.itemChan <- item:
				// Item successfully sent
			}
		}
	}
}