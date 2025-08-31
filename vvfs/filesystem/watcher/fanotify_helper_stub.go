//go:build !linux || !fanotify

package watcher

import "errors"

// newFanotifyWatcher is a stub for when fanotify is not available
func newFanotifyWatcher(config WatcherConfig) (Watcher, error) {
	return nil, errors.New("fanotify not available on this platform")
}
