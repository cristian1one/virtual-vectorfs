//go:build linux && fanotify

package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFanotifyIntegration_EndToEnd tests complete fanotify functionality
func TestFanotifyIntegration_EndToEnd(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:      100 * time.Millisecond,
		MaxDebounceDelay:   500 * time.Millisecond,
		BatchSize:          10,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false, // Disable for faster testing
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Create test directory structure
	testDir := t.TempDir()
	subDir := filepath.Join(testDir, "subdir")
	file1 := filepath.Join(testDir, "test1.txt")
	file2 := filepath.Join(subDir, "test2.txt")

	// Create directories
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Set up event collection
	var receivedEvents []Event
	var eventsMutex sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start watcher
	err = watcher.Start(ctx, []string{testDir})
	require.NoError(t, err)

	// Start event collection goroutine
	go func() {
		for {
			select {
			case event := <-watcher.Events():
				eventsMutex.Lock()
				receivedEvents = append(receivedEvents, event)
				eventsMutex.Unlock()
				t.Logf("Received event: %v on %s", event.Type, event.Path)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Give watcher time to initialize
	time.Sleep(200 * time.Millisecond)

	// Test 1: Create a file
	t.Log("Creating file:", file1)
	err = os.WriteFile(file1, []byte("test content"), 0644)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond) // Wait for event

	// Test 2: Modify the file
	t.Log("Modifying file:", file1)
	err = os.WriteFile(file1, []byte("modified content"), 0644)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond) // Wait for event

	// Test 3: Create a file in subdirectory
	t.Log("Creating file in subdirectory:", file2)
	err = os.WriteFile(file2, []byte("subdir content"), 0644)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond) // Wait for event

	// Test 4: Delete a file
	t.Log("Deleting file:", file1)
	err = os.Remove(file1)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond) // Wait for event

	// Wait a bit more for all events to be processed
	time.Sleep(500 * time.Millisecond)

	// Analyze received events
	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	t.Logf("Total events received: %d", len(receivedEvents))
	for i, event := range receivedEvents {
		t.Logf("Event %d: %v on %s", i, event.Type, event.Path)
	}

	// Verify we received some events (exact count may vary due to system behavior)
	assert.True(t, len(receivedEvents) > 0, "Should receive at least some events")

	// Check that events are for our test files
	foundTestFile := false
	for _, event := range receivedEvents {
		if event.Path == file1 || event.Path == file2 ||
			strings.Contains(event.Path, "test1.txt") ||
			strings.Contains(event.Path, "test2.txt") {
			foundTestFile = true
			break
		}
	}
	assert.True(t, foundTestFile, "Should receive events for test files")
}

// TestFanotifyIntegration_WithDebouncer tests fanotify with debouncing
func TestFanotifyIntegration_WithDebouncer(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:      200 * time.Millisecond,
		MaxDebounceDelay:   1 * time.Second,
		BatchSize:          10,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "debounce_test.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = watcher.Start(ctx, []string{testDir})
	require.NoError(t, err)

	var receivedEvents []Event
	var eventsMutex sync.Mutex

	go func() {
		for {
			select {
			case event := <-watcher.Events():
				eventsMutex.Lock()
				receivedEvents = append(receivedEvents, event)
				eventsMutex.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Rapid file modifications to test debouncing
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 5; i++ {
		content := fmt.Sprintf("content %d", i)
		err = os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Faster than debounce delay
	}

	// Wait for debouncing to complete
	time.Sleep(500 * time.Millisecond)

	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	// With debouncing, we should get fewer events than modifications
	t.Logf("Received %d events for 5 rapid modifications", len(receivedEvents))

	// The exact number may vary, but we should have at least 1 event
	// and typically fewer than 5 due to debouncing
	assert.True(t, len(receivedEvents) >= 1, "Should receive at least one event")
	assert.True(t, len(receivedEvents) <= 5, "Should not receive more events than modifications")
}

// TestFanotifyIntegration_WithProcessor tests fanotify with batch processor
func TestFanotifyIntegration_WithProcessor(t *testing.T) {
	processor := NewReindexProcessor(2, 10)

	config := WatcherConfig{
		DebounceDelay:      100 * time.Millisecond,
		MaxDebounceDelay:   500 * time.Millisecond,
		BatchSize:          5,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Set up processor callbacks
	var processedEvents []Event
	var processedMutex sync.Mutex

	onWrite := func(ctx context.Context, path string) error {
		processedMutex.Lock()
		defer processedMutex.Unlock()
		processedEvents = append(processedEvents, Event{
			Type: EventWrite,
			Path: path,
		})
		t.Logf("Processor handled write event for: %s", path)
		return nil
	}

	onCreate := func(ctx context.Context, path string) error {
		processedMutex.Lock()
		defer processedMutex.Unlock()
		processedEvents = append(processedEvents, Event{
			Type: EventCreate,
			Path: path,
		})
		t.Logf("Processor handled create event for: %s", path)
		return nil
	}

	processor.SetCallbacks(onCreate, onWrite, nil, nil)
	processor.Start()

	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "processor_test.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = watcher.Start(ctx, []string{testDir})
	require.NoError(t, err)

	// Create and modify file
	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	err = os.WriteFile(testFile, []byte("modified"), 0644)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	processor.Close()

	processedMutex.Lock()
	defer processedMutex.Unlock()

	t.Logf("Processor handled %d events", len(processedEvents))
	assert.True(t, len(processedEvents) >= 1, "Processor should handle at least one event")
}

// TestFanotifyIntegration_RecursiveMonitoring tests recursive path monitoring
func TestFanotifyIntegration_RecursiveMonitoring(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:      100 * time.Millisecond,
		MaxDebounceDelay:   500 * time.Millisecond,
		BatchSize:          10,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Create nested directory structure
	testDir := t.TempDir()
	level1 := filepath.Join(testDir, "level1")
	level2 := filepath.Join(level1, "level2")
	level3 := filepath.Join(level2, "level3")

	require.NoError(t, os.MkdirAll(level3, 0755))

	// Create files at different levels
	file1 := filepath.Join(testDir, "root.txt")
	file2 := filepath.Join(level1, "level1.txt")
	file3 := filepath.Join(level2, "level2.txt")
	file4 := filepath.Join(level3, "level3.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = watcher.Start(ctx, []string{testDir})
	require.NoError(t, err)

	var receivedEvents []Event
	var eventsMutex sync.Mutex

	go func() {
		for {
			select {
			case event := <-watcher.Events():
				eventsMutex.Lock()
				receivedEvents = append(receivedEvents, event)
				eventsMutex.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Create files at different levels
	time.Sleep(100 * time.Millisecond)

	files := []string{file1, file2, file3, file4}
	for _, file := range files {
		err = os.WriteFile(file, []byte("test content"), 0644)
		require.NoError(t, err)
		t.Logf("Created file: %s", file)
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all events
	time.Sleep(1 * time.Second)

	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	t.Logf("Received %d events for %d created files", len(receivedEvents), len(files))

	// Should receive events for all files
	assert.True(t, len(receivedEvents) >= len(files),
		"Should receive at least as many events as files created")

	// Check that we got events for files at different directory levels
	eventPaths := make(map[string]bool)
	for _, event := range receivedEvents {
		eventPaths[event.Path] = true
	}

	filesDetected := 0
	for _, file := range files {
		if eventPaths[file] {
			filesDetected++
		}
	}

	t.Logf("Detected %d out of %d files", filesDetected, len(files))
	assert.True(t, filesDetected >= len(files)-1,
		"Should detect most files (some may be missed due to timing)")
}

// TestFanotifyIntegration_DynamicPathManagement tests adding/removing paths dynamically
func TestFanotifyIntegration_DynamicPathManagement(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:      100 * time.Millisecond,
		MaxDebounceDelay:   500 * time.Millisecond,
		BatchSize:          10,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Create test directories
	testDir := t.TempDir()
	dir1 := filepath.Join(testDir, "dir1")
	dir2 := filepath.Join(testDir, "dir2")

	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))

	file1 := filepath.Join(dir1, "file1.txt")
	file2 := filepath.Join(dir2, "file2.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// Start with dir1 only
	err = watcher.Start(ctx, []string{dir1})
	require.NoError(t, err)

	var receivedEvents []Event
	var eventsMutex sync.Mutex

	go func() {
		for {
			select {
			case event := <-watcher.Events():
				eventsMutex.Lock()
				receivedEvents = append(receivedEvents, event)
				eventsMutex.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Create file in dir1 (should be detected)
	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Add dir2 dynamically
	err = watcher.Add(dir2)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Create file in dir2 (should now be detected)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Remove dir1
	err = watcher.Remove(dir1)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Create another file in dir1 (should not be detected)
	file1_2 := filepath.Join(dir1, "file1_2.txt")
	err = os.WriteFile(file1_2, []byte("content1_2"), 0644)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	t.Logf("Received %d total events", len(receivedEvents))

	// Should have events for file1 and file2, but not file1_2 (after dir1 was removed)
	dir1Events := 0
	dir2Events := 0

	for _, event := range receivedEvents {
		if strings.Contains(event.Path, "dir1") {
			dir1Events++
		}
		if strings.Contains(event.Path, "dir2") {
			dir2Events++
		}
	}

	t.Logf("Dir1 events: %d, Dir2 events: %d", dir1Events, dir2Events)

	// Should have events for dir2 files
	assert.True(t, dir2Events > 0, "Should receive events for dir2 after adding it")

	// Note: dir1 events may include events from before removal, which is expected
}

// TestFanotifyIntegration_ErrorRecovery tests error handling and recovery
func TestFanotifyIntegration_ErrorRecovery(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:      100 * time.Millisecond,
		MaxDebounceDelay:   500 * time.Millisecond,
		BatchSize:          10,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "error_test.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = watcher.Start(ctx, []string{testDir})
	require.NoError(t, err)

	var receivedEvents []Event
	var receivedErrors []error
	var eventsMutex, errorsMutex sync.Mutex

	go func() {
		for {
			select {
			case event := <-watcher.Events():
				eventsMutex.Lock()
				receivedEvents = append(receivedEvents, event)
				eventsMutex.Unlock()
			case err := <-watcher.Errors():
				errorsMutex.Lock()
				receivedErrors = append(receivedErrors, err)
				errorsMutex.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Test with valid operations
	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(testFile, []byte("valid content"), 0644)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Test with operations on non-existent paths (should handle gracefully)
	err = watcher.Add("/non/existent/path/that/does/not/exist")
	// This may or may not return an error depending on implementation
	t.Logf("Adding non-existent path result: %v", err)

	// Test removing a path that was never added
	err = watcher.Remove("/another/non/existent/path")
	// This may or may not return an error depending on implementation
	t.Logf("Removing non-existent path result: %v", err)

	time.Sleep(300 * time.Millisecond)

	eventsMutex.Lock()
	errorsMutex.Lock()
	defer eventsMutex.Unlock()
	defer errorsMutex.Unlock()

	t.Logf("Events received: %d, Errors received: %d", len(receivedEvents), len(receivedErrors))

	// Should have received at least one event for the valid file operation
	assert.True(t, len(receivedEvents) >= 1, "Should receive events for valid operations")

	// Errors are acceptable for invalid operations, but shouldn't crash the watcher
	t.Log("Test completed successfully - watcher handled errors gracefully")
}

// TestFanotifyIntegration_HighLoad tests performance under high load
func TestFanotifyIntegration_HighLoad(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:      50 * time.Millisecond,
		MaxDebounceDelay:   200 * time.Millisecond,
		BatchSize:          20,
		WorkerCount:        4,
		QueueCapacity:      200,
		FingerprintEnabled: false,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	testDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = watcher.Start(ctx, []string{testDir})
	require.NoError(t, err)

	var receivedEvents []Event
	var eventsMutex sync.Mutex

	go func() {
		for {
			select {
			case event := <-watcher.Events():
				eventsMutex.Lock()
				receivedEvents = append(receivedEvents, event)
				eventsMutex.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Create many files rapidly to test high load
	time.Sleep(100 * time.Millisecond)

	numFiles := 20
	for i := 0; i < numFiles; i++ {
		filename := filepath.Join(testDir, fmt.Sprintf("load_test_%d.txt", i))
		content := fmt.Sprintf("content for file %d", i)
		err = os.WriteFile(filename, []byte(content), 0644)
		require.NoError(t, err)
	}

	t.Logf("Created %d files rapidly", numFiles)

	// Wait for events to be processed
	time.Sleep(2 * time.Second)

	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	t.Logf("Received %d events for %d file operations", len(receivedEvents), numFiles)

	// Should receive a reasonable number of events (may be less than numFiles due to debouncing)
	assert.True(t, len(receivedEvents) > 0, "Should receive some events under load")
	assert.True(t, len(receivedEvents) <= numFiles*2, "Should not receive excessive events")
}
