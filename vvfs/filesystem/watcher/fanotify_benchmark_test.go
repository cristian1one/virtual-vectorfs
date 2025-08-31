//go:build linux && fanotify

package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// BenchmarkFanotifyInit benchmarks fanotify initialization
func BenchmarkFanotifyInit(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fd, err := FanotifyInit(FAN_CLOEXEC|FAN_CLASS_CONTENT, O_RDONLY)
		if err != nil {
			b.Skip("Fanotify not available:", err)
			return
		}
		syscall.Close(fd)
	}
}

// BenchmarkFanotifyMark benchmarks fanotify mark operations
func BenchmarkFanotifyMark(b *testing.B) {
	fd, err := FanotifyInit(FAN_CLOEXEC|FAN_CLASS_CONTENT, O_RDONLY)
	if err != nil {
		b.Skip("Fanotify not available:", err)
		return
	}
	defer syscall.Close(fd)

	tempDir := b.TempDir()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = doFanotifyMark(fd, FAN_MARK_ADD, FAN_ACCESS, -1, tempDir)
		if err != nil {
			b.Fatal("Fanotify mark failed:", err)
		}

		err = doFanotifyMark(fd, FAN_MARK_REMOVE, FAN_ACCESS, -1, tempDir)
		if err != nil {
			b.Fatal("Fanotify mark removal failed:", err)
		}
	}
}

// BenchmarkFanotifyWatcherCreation benchmarks watcher creation
func BenchmarkFanotifyWatcherCreation(b *testing.B) {
	config := WatcherConfig{
		DebounceDelay:      50 * time.Millisecond,
		MaxDebounceDelay:   200 * time.Millisecond,
		BatchSize:          10,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		watcher, err := NewFanotifyWatcher(config)
		if err != nil {
			b.Skip("Fanotify watcher creation failed:", err)
			return
		}
		watcher.Close()
	}
}

// BenchmarkFanotifyWatcherOperations benchmarks basic watcher operations
func BenchmarkFanotifyWatcherOperations(b *testing.B) {
	config := WatcherConfig{
		DebounceDelay:      50 * time.Millisecond,
		MaxDebounceDelay:   200 * time.Millisecond,
		BatchSize:          10,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		b.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	tempDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = watcher.Add(tempDir)
		if err != nil {
			b.Fatal("Add operation failed:", err)
		}

		err = watcher.Remove(tempDir)
		if err != nil {
			b.Fatal("Remove operation failed:", err)
		}
	}
}

// BenchmarkFanotifyEventProcessing benchmarks event processing
func BenchmarkFanotifyEventProcessing(b *testing.B) {
	watcher := &FanotifyWatcher{}

	// Create test event
	event := &fanotifyEventMetadata{
		Event_len: 32,
		Mask:      FAN_CREATE,
		Fd:        5,
		Pid:       1234,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := watcher.convertFanotifyEvent(event)
		if result == nil {
			b.Fatal("Event conversion returned nil")
		}
	}
}

// BenchmarkFanotifyPathResolution benchmarks path resolution
func BenchmarkFanotifyPathResolution(b *testing.B) {
	watcher := &FanotifyWatcher{}

	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "bench_test")
	if err != nil {
		b.Skip("Could not create temp file:", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := watcher.getPathFromFd(int(tempFile.Fd()))
		// We don't check the error since path resolution may fail on some systems
		_ = err
	}
}

// BenchmarkFanotifyRecursivePathAddition benchmarks recursive path addition
func BenchmarkFanotifyRecursivePathAddition(b *testing.B) {
	config := WatcherConfig{
		DebounceDelay:      50 * time.Millisecond,
		MaxDebounceDelay:   200 * time.Millisecond,
		BatchSize:          10,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		b.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Create a directory structure for testing
	baseDir := b.TempDir()
	testDirs := make([]string, 10)
	for i := 0; i < 10; i++ {
		dir := filepath.Join(baseDir, fmt.Sprintf("dir_%d", i))
		os.MkdirAll(dir, 0755)
		testDirs[i] = dir
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		err = watcher.Start(ctx, testDirs)
		if err != nil {
			b.Fatal("Recursive path addition failed:", err)
		}
	}
}

// BenchmarkFanotifyHighFrequencyEvents benchmarks handling of high-frequency events
func BenchmarkFanotifyHighFrequencyEvents(b *testing.B) {
	config := WatcherConfig{
		DebounceDelay:      100 * time.Millisecond,
		MaxDebounceDelay:   500 * time.Millisecond,
		BatchSize:          20,
		WorkerCount:        4,
		QueueCapacity:      200,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		b.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	testDir := b.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = watcher.Start(ctx, []string{testDir})
	if err != nil {
		b.Skip("Watcher start failed:", err)
		return
	}

	// Start event consumer
	eventCount := 0
	go func() {
		for {
			select {
			case <-watcher.Events():
				eventCount++
			case <-ctx.Done():
				return
			}
		}
	}()

	b.ResetTimer()

	// Generate many file operations
	for i := 0; i < b.N; i++ {
		filename := filepath.Join(testDir, fmt.Sprintf("bench_file_%d.txt", i))
		content := fmt.Sprintf("content %d", i)
		os.WriteFile(filename, []byte(content), 0644)
	}

	// Wait for events to be processed
	time.Sleep(500 * time.Millisecond)

	b.Logf("Processed %d events for %d operations", eventCount, b.N)
}

// BenchmarkFanotifyMemoryUsage benchmarks memory usage patterns
func BenchmarkFanotifyMemoryUsage(b *testing.B) {
	config := WatcherConfig{
		DebounceDelay:      50 * time.Millisecond,
		MaxDebounceDelay:   200 * time.Millisecond,
		BatchSize:          10,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		watcher, err := NewFanotifyWatcher(config)
		if err != nil {
			b.Skip("Fanotify watcher creation failed:", err)
			return
		}

		// Perform some operations
		tempDir := b.TempDir()
		err = watcher.Add(tempDir)
		if err != nil {
			watcher.Close()
			continue
		}

		watcher.Close()
	}
}

// BenchmarkFanotifyConcurrentOperations benchmarks concurrent watcher operations
func BenchmarkFanotifyConcurrentOperations(b *testing.B) {
	config := WatcherConfig{
		DebounceDelay:      50 * time.Millisecond,
		MaxDebounceDelay:   200 * time.Millisecond,
		BatchSize:          10,
		WorkerCount:        4,
		QueueCapacity:      200,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		b.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	testDir := b.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = watcher.Start(ctx, []string{testDir})
	if err != nil {
		b.Skip("Watcher start failed:", err)
		return
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		localDir := filepath.Join(testDir, fmt.Sprintf("goroutine_%d", b.N))
		os.MkdirAll(localDir, 0755)

		for pb.Next() {
			// Add path
			err := watcher.Add(localDir)
			if err != nil {
				continue // Skip errors in benchmark
			}

			// Create file
			filename := filepath.Join(localDir, "bench_file.txt")
			os.WriteFile(filename, []byte("bench content"), 0644)

			// Remove path
			watcher.Remove(localDir)
		}
	})
}

// BenchmarkFanotifyLargeDirectoryTree benchmarks performance with large directory trees
func BenchmarkFanotifyLargeDirectoryTree(b *testing.B) {
	config := WatcherConfig{
		DebounceDelay:      100 * time.Millisecond,
		MaxDebounceDelay:   500 * time.Millisecond,
		BatchSize:          50,
		WorkerCount:        4,
		QueueCapacity:      500,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		b.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Create a large directory tree
	baseDir := b.TempDir()
	var allDirs []string

	// Create 5 levels deep with 3 dirs each level
	var createDirs func(base string, depth int)
	createDirs = func(base string, depth int) {
		if depth <= 0 {
			return
		}
		for i := 0; i < 3; i++ {
			dir := filepath.Join(base, fmt.Sprintf("level_%d_%d", depth, i))
			os.MkdirAll(dir, 0755)
			allDirs = append(allDirs, dir)
			createDirs(dir, depth-1)
		}
	}

	createDirs(baseDir, 3) // Creates ~40 directories total

	b.Logf("Created %d directories for benchmark", len(allDirs))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = watcher.Start(ctx, []string{baseDir})
		if err != nil {
			b.Fatal("Large directory tree monitoring failed:", err)
		}
	}
}

// BenchmarkFanotifyEventThroughput benchmarks event processing throughput
func BenchmarkFanotifyEventThroughput(b *testing.B) {
	// Create a mock event buffer for testing
	eventSize := int(unsafe.Sizeof(fanotifyEventMetadata{}))
	buffer := make([]byte, eventSize*1000) // Space for 1000 events

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate parsing events from buffer
		offset := 0
		eventCount := 0

		for offset < len(buffer)-eventSize {
			event := (*fanotifyEventMetadata)(unsafe.Pointer(&buffer[offset]))
			// Simulate basic event processing
			if event.Mask > 0 {
				eventCount++
			}
			offset += eventSize
		}

		if eventCount == 0 {
			b.Fatal("No events processed")
		}
	}
}
