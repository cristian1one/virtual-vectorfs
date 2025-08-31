package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FSNotifyWatcher implements the Watcher interface using fsnotify
type FSNotifyWatcher struct {
	watcher      *fsnotify.Watcher
	eventChan    chan Event
	errorChan    chan error
	debouncer    Debouncer
	processor    BatchProcessor
	config       WatcherConfig
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.RWMutex
	watchedPaths map[string]bool
}

// NewFSNotifyWatcher creates a new fsnotify-based watcher
func NewFSNotifyWatcher(config WatcherConfig) (*FSNotifyWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &FSNotifyWatcher{
		watcher:      fsWatcher,
		eventChan:    make(chan Event, config.QueueCapacity),
		errorChan:    make(chan error, 10),
		config:       config,
		ctx:          ctx,
		cancel:       cancel,
		watchedPaths: make(map[string]bool),
	}

	// Initialize debouncer if delay is configured
	if config.DebounceDelay > 0 {
		// TODO: Implement debouncer
		// w.debouncer = NewDebouncer(config.DebounceDelay, config.MaxDebounceDelay, config.QueueCapacity)
	}

	return w, nil
}

// Start begins watching the specified paths
func (w *FSNotifyWatcher) Start(ctx context.Context, paths []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Add all paths
	for _, path := range paths {
		if err := w.addPathRecursive(path); err != nil {
			slog.Warn("Failed to add path to watcher", "path", path, "error", err)
			continue
		}
		w.watchedPaths[path] = true
	}

	// Start event processing
	w.wg.Add(1)
	go w.processEvents()

	// Start the main event loop
	w.wg.Add(1)
	go w.watchLoop()

	slog.Info("FSNotify watcher started", "paths", len(paths))
	return nil
}

// Events returns the event channel
func (w *FSNotifyWatcher) Events() <-chan Event {
	return w.eventChan
}

// Errors returns the error channel
func (w *FSNotifyWatcher) Errors() <-chan error {
	return w.errorChan
}

// Add adds paths to watch
func (w *FSNotifyWatcher) Add(paths ...string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, path := range paths {
		if err := w.addPathRecursive(path); err != nil {
			return fmt.Errorf("failed to add path %s: %w", path, err)
		}
		w.watchedPaths[path] = true
	}

	slog.Debug("Added paths to watcher", "count", len(paths))
	return nil
}

// Remove removes paths from watching
func (w *FSNotifyWatcher) Remove(paths ...string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, path := range paths {
		if err := w.watcher.Remove(path); err != nil {
			slog.Warn("Failed to remove path from watcher", "path", path, "error", err)
		}
		delete(w.watchedPaths, path)
	}

	slog.Debug("Removed paths from watcher", "count", len(paths))
	return nil
}

// Close stops watching and cleans up resources
func (w *FSNotifyWatcher) Close() error {
	w.mu.Lock()
	w.cancel()
	w.mu.Unlock()

	// Close debouncer if it exists
	if w.debouncer != nil {
		w.debouncer.Close()
	}

	// Close processor if it exists
	if w.processor != nil {
		if err := w.processor.Close(); err != nil {
			slog.Warn("Error closing processor", "error", err)
		}
	}

	// Close fsnotify watcher
	if err := w.watcher.Close(); err != nil {
		slog.Warn("Error closing fsnotify watcher", "error", err)
	}

	// Wait for goroutines to finish
	w.wg.Wait()

	// Close channels
	close(w.eventChan)
	close(w.errorChan)

	slog.Info("FSNotify watcher closed")
	return nil
}

// addPathRecursive adds a path and all its subdirectories to the watcher
func (w *FSNotifyWatcher) addPathRecursive(rootPath string) error {
	// Add the root path
	if err := w.watcher.Add(rootPath); err != nil {
		return fmt.Errorf("failed to add root path %s: %w", rootPath, err)
	}

	// Walk the directory tree and add all subdirectories
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				slog.Warn("Failed to add subdirectory to watcher", "path", path, "error", err)
				// Don't return error, continue with other directories
			}
		}

		return nil
	})
}

// watchLoop is the main event processing loop
func (w *FSNotifyWatcher) watchLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			watcherEvent := w.convertEvent(event)
			if watcherEvent == nil {
				continue
			}

			if w.debouncer != nil {
				w.debouncer.Add(*watcherEvent)
			} else {
				select {
				case w.eventChan <- *watcherEvent:
				case <-w.ctx.Done():
					return
				default:
					slog.Warn("Event channel full, dropping event", "path", watcherEvent.Path)
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}

			select {
			case w.errorChan <- err:
			case <-w.ctx.Done():
				return
			default:
				slog.Warn("Error channel full, dropping error", "error", err)
			}
		}
	}
}

// processEvents handles debounced events
func (w *FSNotifyWatcher) processEvents() {
	defer w.wg.Done()

	if w.debouncer == nil {
		return
	}

	for {
		select {
		case <-w.ctx.Done():
			return

		case events, ok := <-w.debouncer.Events():
			if !ok {
				return
			}

			// Process events in batch
			if w.processor != nil {
				if err := w.processor.Process(w.ctx, events); err != nil {
					slog.Error("Error processing events", "error", err)
				}
			} else {
				// Send individual events
				for _, event := range events {
					select {
					case w.eventChan <- event:
					case <-w.ctx.Done():
						return
					default:
						slog.Warn("Event channel full, dropping event", "path", event.Path)
					}
				}
			}
		}
	}
}

// convertEvent converts fsnotify.Event to watcher.Event
func (w *FSNotifyWatcher) convertEvent(event fsnotify.Event) *Event {
	var eventType EventType

	switch {
	case event.Has(fsnotify.Create):
		eventType = EventCreate
	case event.Has(fsnotify.Write):
		eventType = EventWrite
	case event.Has(fsnotify.Remove):
		eventType = EventRemove
	case event.Has(fsnotify.Rename):
		eventType = EventRename
	case event.Has(fsnotify.Chmod):
		eventType = EventChmod
	default:
		return nil // Ignore unknown events
	}

	return &Event{
		Type:      eventType,
		Path:      event.Name,
		Timestamp: time.Now(),
		IsDir:     false, // fsnotify doesn't provide this info directly
	}
}
