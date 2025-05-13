package core

// DefaultProgressTracker implements the ProgressTracker interface
type DefaultProgressTracker struct {
	handler MessageHandler
	barID   string
}

// NewProgressTracker creates a new DefaultProgressTracker
func NewProgressTracker(handler MessageHandler, barID string) ProgressTracker {
	if barID == "" {
		barID = "default-progress"
	}
	
	return &DefaultProgressTracker{
		handler: handler,
		barID:   barID,
	}
}

// UpdateProgress updates the overall progress
func (p *DefaultProgressTracker) UpdateProgress(completed, total int, description string) {
	if p.handler != nil {
		// Use advanced ETA for item-bar, simple ETA for other progress bars
		if p.barID == ProgressBarIDItem {
			p.handler.IncrementProgressAdvanced(
				p.barID,         // taskID
				1,               // increment
				total,           // total
				20,              // priority
				description,     // operation
				description,     // description
				"h-2",           // size
			)
		} else {
			p.handler.IncrementProgress(
				p.barID,         // taskID
				1,               // increment
				total,           // total
				20,              // priority
				description,     // operation
				description,     // description
				"h-2",           // size
			)
		}
	}
}

// MarkCompleted marks a specific item as completed
func (p *DefaultProgressTracker) MarkCompleted(id string) {
	if p.handler != nil {
		p.handler.ZeroLog().Debug().
			Str("itemID", id).
			Msg("Item completed")
	}
}

// MarkFailed marks a specific item as failed
func (p *DefaultProgressTracker) MarkFailed(id string, err error) {
	if p.handler != nil {
		p.handler.ZeroLog().Error().
			Str("itemID", id).
			Err(err).
			Msg("Item processing failed")
	}
}