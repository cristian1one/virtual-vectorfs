//go:build !linux || !fanotify

package vfanotify

import (
	"errors"
	"log/slog"
)

// FanotifyEvent represents a fanotify event (stub)
type FanotifyEvent struct {
	Wd     int32
	Mask   uint64
	Cookie uint32
	Len    uint32
	Pid    int32
	Path   string
	FileFd int32
}

// Watcher provides fanotify file system monitoring (stub)
type Watcher struct{}

// NewWatcher creates a new fanotify watcher (stub)
func NewWatcher() (*Watcher, error) {
	return nil, errors.New("fanotify not available on this platform")
}

// AddPath adds a path to be monitored (stub)
func (w *Watcher) AddPath(path string) error {
	return errors.New("fanotify not available on this platform")
}

// RemovePath removes a path from monitoring (stub)
func (w *Watcher) RemovePath(path string) error {
	return errors.New("fanotify not available on this platform")
}

// Start begins the event monitoring loop (stub)
func (w *Watcher) Start() error {
	return errors.New("fanotify not available on this platform")
}

// Events returns the event channel (stub)
func (w *Watcher) Events() <-chan FanotifyEvent {
	return nil
}

// Errors returns the error channel (stub)
func (w *Watcher) Errors() <-chan error {
	return nil
}

// Close stops monitoring and cleans up resources (stub)
func (w *Watcher) Close() error {
	return errors.New("fanotify not available on this platform")
}

// IsAvailable returns true if fanotify is available on this system (stub)
func IsAvailable() bool {
	slog.Debug("Fanotify not available - using stub implementation")
	return false
}

// HasPermission checks if the current process has permission to use fanotify (stub)
func HasPermission() bool {
	slog.Debug("Fanotify permission check - stub implementation")
	return false
}
