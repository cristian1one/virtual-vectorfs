//go:build linux && fanotify

package watcher

// newFanotifyWatcher is a helper function available when fanotify is supported
func newFanotifyWatcher(config WatcherConfig) (Watcher, error) {
	return NewFanotifyWatcher(config)
}
