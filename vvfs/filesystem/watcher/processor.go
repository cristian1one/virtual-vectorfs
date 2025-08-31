package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ReindexProcessor implements BatchProcessor for file system re-indexing
type ReindexProcessor struct {
	workerCount int
	workChan    chan []Event
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex

	// Callbacks for different event types
	onCreate func(ctx context.Context, path string) error
	onWrite  func(ctx context.Context, path string) error
	onRemove func(ctx context.Context, path string) error
	onRename func(ctx context.Context, oldPath, newPath string) error

	// Statistics
	processedEvents int64
	processingTime  time.Duration
}

// NewReindexProcessor creates a new batch processor for file system events
func NewReindexProcessor(workerCount int, queueCapacity int) *ReindexProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	return &ReindexProcessor{
		workerCount: workerCount,
		workChan:    make(chan []Event, queueCapacity),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// SetCallbacks sets the callback functions for different event types
func (p *ReindexProcessor) SetCallbacks(
	onCreate func(ctx context.Context, path string) error,
	onWrite func(ctx context.Context, path string) error,
	onRemove func(ctx context.Context, path string) error,
	onRename func(ctx context.Context, oldPath, newPath string) error,
) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.onCreate = onCreate
	p.onWrite = onWrite
	p.onRemove = onRemove
	p.onRename = onRename
}

// Start starts the processor workers
func (p *ReindexProcessor) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Process processes a batch of events
func (p *ReindexProcessor) Process(ctx context.Context, events []Event) error {
	start := time.Now()

	// Send events to worker pool
	select {
	case p.workChan <- events:
		slog.Debug("Queued event batch for processing", "count", len(events))
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return fmt.Errorf("processor is shutting down")
	}

	// Update statistics
	p.mu.Lock()
	p.processedEvents += int64(len(events))
	p.processingTime += time.Since(start)
	p.mu.Unlock()

	return nil
}

// worker processes events from the work channel
func (p *ReindexProcessor) worker(id int) {
	defer p.wg.Done()

	slog.Debug("Started reindex worker", "worker_id", id)

	for {
		select {
		case <-p.ctx.Done():
			slog.Debug("Worker shutting down", "worker_id", id)
			return

		case events, ok := <-p.workChan:
			if !ok {
				slog.Debug("Work channel closed, worker shutting down", "worker_id", id)
				return
			}

			p.processBatch(events)
		}
	}
}

// processBatch processes a batch of events
func (p *ReindexProcessor) processBatch(events []Event) {
	// Group events by path to handle multiple events on same file
	eventMap := make(map[string][]Event)

	for _, event := range events {
		eventMap[event.Path] = append(eventMap[event.Path], event)
	}

	// Process each unique path
	for path, pathEvents := range eventMap {
		if err := p.processPathEvents(path, pathEvents); err != nil {
			slog.Error("Error processing events for path", "path", path, "error", err)
		}
	}
}

// processPathEvents processes all events for a single path
func (p *ReindexProcessor) processPathEvents(path string, events []Event) error {
	slog.Debug("Processing events for path", "path", path, "eventCount", len(events))

	// Validate that all events belong to the expected path (for debugging)
	for i, event := range events {
		if event.Path != path && event.OldPath != path {
			slog.Warn("Event path mismatch",
				"path", path,
				"eventPath", event.Path,
				"eventOldPath", event.OldPath,
				"eventIndex", i)
		}
	}

	p.mu.RLock()
	onCreate := p.onCreate
	onWrite := p.onWrite
	onRemove := p.onRemove
	onRename := p.onRename
	p.mu.RUnlock()

	// Process events in chronological order (assuming they're already sorted)
	for i, event := range events {
		var err error

		slog.Debug("Processing event",
			"path", path,
			"eventIndex", i,
			"eventType", event.Type,
			"eventPath", event.Path)

		switch event.Type {
		case EventCreate:
			if onCreate != nil {
				err = onCreate(p.ctx, event.Path)
			}

		case EventWrite:
			if onWrite != nil {
				err = onWrite(p.ctx, event.Path)
			}

		case EventRemove:
			if onRemove != nil {
				err = onRemove(p.ctx, event.Path)
			}

		case EventRename:
			if onRename != nil {
				err = onRename(p.ctx, event.OldPath, event.Path)
			}

		case EventChmod:
			// Permission changes might require special handling
			slog.Debug("Permission change detected", "expectedPath", path, "eventPath", event.Path)

		default:
			slog.Warn("Unknown event type", "type", event.Type, "expectedPath", path, "eventPath", event.Path)
		}

		if err != nil {
			return fmt.Errorf("error processing %v event for path %s (event path: %s): %w",
				event.Type, path, event.Path, err)
		}
	}

	slog.Debug("Completed processing events for path", "path", path, "processedCount", len(events))
	return nil
}

// GetStats returns processing statistics
func (p *ReindexProcessor) GetStats() (processedEvents int64, avgProcessingTime time.Duration) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.processedEvents == 0 {
		return 0, 0
	}

	return p.processedEvents, p.processingTime / time.Duration(p.processedEvents)
}

// Close stops the processor and waits for all workers to finish
func (p *ReindexProcessor) Close() error {
	p.mu.Lock()
	p.cancel()
	close(p.workChan)
	p.mu.Unlock()

	slog.Info("Waiting for processor workers to finish")
	p.wg.Wait()

	slog.Info("Reindex processor closed")
	return nil
}

// SimpleProcessor provides a simpler interface for basic event processing
type SimpleProcessor struct {
	handler func(ctx context.Context, event Event) error
}

// NewSimpleProcessor creates a new simple processor
func NewSimpleProcessor(handler func(ctx context.Context, event Event) error) *SimpleProcessor {
	return &SimpleProcessor{
		handler: handler,
	}
}

// Process processes events one by one
func (p *SimpleProcessor) Process(ctx context.Context, events []Event) error {
	for _, event := range events {
		if err := p.handler(ctx, event); err != nil {
			return fmt.Errorf("error processing event %v: %w", event, err)
		}
	}
	return nil
}

// Close is a no-op for simple processor
func (p *SimpleProcessor) Close() error {
	return nil
}
