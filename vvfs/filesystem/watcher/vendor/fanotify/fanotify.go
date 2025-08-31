//go:build linux && fanotify

package vfanotify

/*
#include <linux/fanotify.h>
#include <sys/fanotify.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>

// Note: Constants are defined in the header files above
// FAN_CLOEXEC, FAN_CLASS_CONTENT, O_RDONLY, FAN_MARK_ADD, etc.
// are available from the included headers

// Wrapper functions for C fanotify API
int c_fanotify_init(unsigned int flags, unsigned int event_f_flags) {
    return fanotify_init(flags, event_f_flags);
}

int c_fanotify_mark(int fanotify_fd, unsigned int flags, uint64_t mask,
                   int dirfd, const char *pathname) {
    return fanotify_mark(fanotify_fd, flags, mask, dirfd, pathname);
}

int c_fanotify_read(int fd, void *buf, size_t count) {
    return read(fd, buf, count);
}

// Memory management functions are available from standard C library
// C.CString and C.free are automatically available when including <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

// FanotifyEvent represents a fanotify event
type FanotifyEvent struct {
	Wd     int32
	Mask   uint64
	Cookie uint32
	Len    uint32
	Pid    int32
	Path   string
	FileFd int32
}

// Watcher provides fanotify file system monitoring
type Watcher struct {
	fd           int
	eventChan    chan FanotifyEvent
	errorChan    chan error
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.RWMutex
	watchedPaths map[string]bool
}

// NewWatcher creates a new fanotify watcher
func NewWatcher() (*Watcher, error) {
	// Initialize fanotify
	fd, err := C.c_fanotify_init(C.FAN_CLOEXEC|C.FAN_CLASS_CONTENT, C.O_RDONLY)
	if fd < 0 {
		return nil, fmt.Errorf("failed to initialize fanotify: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &Watcher{
		fd:           int(fd),
		eventChan:    make(chan FanotifyEvent, 1000),
		errorChan:    make(chan error, 10),
		ctx:          ctx,
		cancel:       cancel,
		watchedPaths: make(map[string]bool),
	}

	return w, nil
}

// AddPath adds a path to be monitored
func (w *Watcher) AddPath(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Mark for notification
	pathC := C.CString(path)
	defer C.free(unsafe.Pointer(pathC))

	ret, err := C.c_fanotify_mark(C.int(w.fd), C.FAN_MARK_ADD,
		C.uint64_t(C.FAN_MODIFY|C.FAN_CLOSE_WRITE|C.FAN_CREATE|C.FAN_DELETE|C.FAN_MOVED_FROM|C.FAN_MOVED_TO),
		C.AT_FDCWD, pathC)
	if ret < 0 {
		return fmt.Errorf("fanotify_mark failed for %s: %v", path, err)
	}

	w.watchedPaths[path] = true
	slog.Debug("Added path to fanotify watcher", "path", path)
	return nil
}

// RemovePath removes a path from monitoring
func (w *Watcher) RemovePath(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	pathC := C.CString(path)
	defer C.free(unsafe.Pointer(pathC))

	ret, err := C.c_fanotify_mark(C.int(w.fd), C.FAN_MARK_REMOVE, 0, C.AT_FDCWD, pathC)
	if ret < 0 {
		return fmt.Errorf("fanotify_mark remove failed for %s: %v", path, err)
	}

	delete(w.watchedPaths, path)
	slog.Debug("Removed path from fanotify watcher", "path", path)
	return nil
}

// Start begins the event monitoring loop
func (w *Watcher) Start() error {
	w.wg.Add(1)
	go w.eventLoop()
	return nil
}

// Events returns the event channel
func (w *Watcher) Events() <-chan FanotifyEvent {
	return w.eventChan
}

// Errors returns the error channel
func (w *Watcher) Errors() <-chan error {
	return w.errorChan
}

// Close stops monitoring and cleans up resources
func (w *Watcher) Close() error {
	w.mu.Lock()
	w.cancel()
	w.mu.Unlock()

	// Wait for goroutines
	w.wg.Wait()

	// Close file descriptor
	if err := syscall.Close(w.fd); err != nil {
		slog.Warn("Error closing fanotify fd", "error", err)
	}

	close(w.eventChan)
	close(w.errorChan)

	slog.Info("Fanotify watcher closed")
	return nil
}

// eventLoop is the main event processing loop
func (w *Watcher) eventLoop() {
	defer w.wg.Done()

	eventBuf := make([]byte, 4096)

	for {
		select {
		case <-w.ctx.Done():
			return

		default:
			// Read fanotify events
			n, err := syscall.Read(w.fd, eventBuf)
			if err != nil {
				select {
				case w.errorChan <- err:
				case <-w.ctx.Done():
					return
				default:
					slog.Warn("Error channel full, dropping error", "error", err)
				}
				continue
			}

			// Process events
			w.processEvents(eventBuf[:n])
		}
	}
}

// processEvents processes raw fanotify events
func (w *Watcher) processEvents(buf []byte) {
	const eventSize = 24 // Size of fanotify_event struct

	for len(buf) >= eventSize {
		event := (*C.struct_fanotify_event_metadata)(unsafe.Pointer(&buf[0]))

		fanEvent := FanotifyEvent{
			Wd:     0, // Not available in event_metadata
			Mask:   uint64(event.mask),
			Cookie: 0, // Not available in event_metadata
			Len:    uint32(event.event_len),
			Pid:    int32(event.pid),
			FileFd: int32(event.fd),
		}

		// Get path from file descriptor if available
		if event.fd >= 0 {
			path, err := w.getPathFromFd(int(event.fd))
			if err == nil {
				fanEvent.Path = path
			}
		}

		// Send event
		select {
		case w.eventChan <- fanEvent:
		case <-w.ctx.Done():
			return
		default:
			slog.Warn("Event channel full, dropping event", "fd", event.fd)
		}

		buf = buf[uint32(event.event_len):]
	}
}

// getPathFromFd gets the path from a file descriptor
func (w *Watcher) getPathFromFd(fd int) (string, error) {
	// Use /proc/self/fd to get the path
	linkPath := fmt.Sprintf("/proc/self/fd/%d", fd)

	path, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("failed to readlink %s: %w", linkPath, err)
	}

	// Clean the path
	return filepath.Clean(path), nil
}

// IsAvailable returns true if fanotify is available on this system
func IsAvailable() bool {
	fd, _ := C.c_fanotify_init(C.FAN_CLOEXEC|C.FAN_CLASS_CONTENT, C.O_RDONLY)
	if fd < 0 {
		return false
	}
	syscall.Close(int(fd))
	return true
}

// HasPermission checks if the current process has permission to use fanotify
func HasPermission() bool {
	fd, _ := C.c_fanotify_init(C.FAN_CLOEXEC|C.FAN_CLASS_CONTENT, C.O_RDONLY)
	if fd < 0 {
		return false
	}
	defer syscall.Close(int(fd))

	// Try to mark a directory we know exists (current directory)
	cwd := C.CString(".")
	defer C.free(unsafe.Pointer(cwd))

	ret, _ := C.c_fanotify_mark(C.int(fd), C.FAN_MARK_ADD, C.uint64_t(C.FAN_ACCESS), C.AT_FDCWD, cwd)
	return ret >= 0
}
