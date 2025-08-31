//go:build linux && fanotify

package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelper provides utilities for fanotify testing
type TestHelper struct {
	t         *testing.T
	tempDir   string
	watchers  []*FanotifyWatcher
	cleanupMu sync.Mutex
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{
		t:       t,
		tempDir: t.TempDir(),
	}
}

// TempDir returns the temporary directory for this test
func (h *TestHelper) TempDir() string {
	return h.tempDir
}

// CreateTestDirectory creates a test directory structure
func (h *TestHelper) CreateTestDirectory(name string) string {
	dir := filepath.Join(h.tempDir, name)
	require.NoError(h.t, os.MkdirAll(dir, 0755))
	return dir
}

// CreateTestFile creates a test file with content
func (h *TestHelper) CreateTestFile(dir, name, content string) string {
	filePath := filepath.Join(dir, name)
	require.NoError(h.t, os.WriteFile(filePath, []byte(content), 0644))
	return filePath
}

// CreateTestDirectories creates multiple test directories
func (h *TestHelper) CreateTestDirectories(count int, prefix string) []string {
	var dirs []string
	for i := 0; i < count; i++ {
		dir := h.CreateTestDirectory(fmt.Sprintf("%s_%d", prefix, i))
		dirs = append(dirs, dir)
	}
	return dirs
}

// CreateNestedDirectoryStructure creates a nested directory structure
func (h *TestHelper) CreateNestedDirectoryStructure(depth, width int) string {
	baseDir := h.CreateTestDirectory("nested")

	var createDirs func(base string, currentDepth int)
	createDirs = func(base string, currentDepth int) {
		if currentDepth <= 0 {
			return
		}
		for i := 0; i < width; i++ {
			subDir := filepath.Join(base, fmt.Sprintf("level_%d_%d", currentDepth, i))
			require.NoError(h.t, os.MkdirAll(subDir, 0755))
			createDirs(subDir, currentDepth-1)
		}
	}

	createDirs(baseDir, depth)
	return baseDir
}

// CreateFanotifyWatcher creates a fanotify watcher with default config
func (h *TestHelper) CreateFanotifyWatcher() *FanotifyWatcher {
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
		h.t.Skip("Fanotify watcher creation failed:", err)
		return nil
	}

	h.cleanupMu.Lock()
	h.watchers = append(h.watchers, watcher)
	h.cleanupMu.Unlock()

	return watcher
}

// CreateFanotifyWatcherWithConfig creates a fanotify watcher with custom config
func (h *TestHelper) CreateFanotifyWatcherWithConfig(config WatcherConfig) *FanotifyWatcher {
	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		h.t.Skip("Fanotify watcher creation failed:", err)
		return nil
	}

	h.cleanupMu.Lock()
	h.watchers = append(h.watchers, watcher)
	h.cleanupMu.Unlock()

	return watcher
}

// StartWatcher starts a watcher with the given paths
func (h *TestHelper) StartWatcher(watcher *FanotifyWatcher, paths []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := watcher.Start(ctx, paths)
	require.NoError(h.t, err)
}

// CollectEvents collects events from a watcher for a specified duration
func (h *TestHelper) CollectEvents(watcher *FanotifyWatcher, duration time.Duration) []Event {
	var events []Event
	var eventsMu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	go func() {
		for {
			select {
			case event := <-watcher.Events():
				eventsMu.Lock()
				events = append(events, event)
				eventsMu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(duration)
	cancel()

	eventsMu.Lock()
	defer eventsMu.Unlock()
	return events
}

// PerformFileOperations performs various file operations to generate events
func (h *TestHelper) PerformFileOperations(dir string, operations []FileOperation) {
	for _, op := range operations {
		switch op.Type {
		case "create":
			h.CreateTestFile(dir, op.Name, op.Content)
		case "write":
			filePath := filepath.Join(dir, op.Name)
			require.NoError(h.t, os.WriteFile(filePath, []byte(op.Content), 0644))
		case "delete":
			filePath := filepath.Join(dir, op.Name)
			os.Remove(filePath) // Don't require success as file might not exist
		case "mkdir":
			subDir := filepath.Join(dir, op.Name)
			os.MkdirAll(subDir, 0755) // Don't require success
		case "rmdir":
			subDir := filepath.Join(dir, op.Name)
			os.RemoveAll(subDir) // Don't require success
		}

		if op.Delay > 0 {
			time.Sleep(op.Delay)
		}
	}
}

// Cleanup cleans up all created watchers
func (h *TestHelper) Cleanup() {
	h.cleanupMu.Lock()
	defer h.cleanupMu.Unlock()

	for _, watcher := range h.watchers {
		watcher.Close()
	}
	h.watchers = nil
}

// FileOperation represents a file operation for testing
type FileOperation struct {
	Type    string        // "create", "write", "delete", "mkdir", "rmdir"
	Name    string        // filename or dirname
	Content string        // content for create/write operations
	Delay   time.Duration // delay after operation
}

// EventCollector helps collect and analyze events
type EventCollector struct {
	events []Event
	mu     sync.Mutex
	done   chan bool
}

// NewEventCollector creates a new event collector
func NewEventCollector() *EventCollector {
	return &EventCollector{
		events: make([]Event, 0),
		done:   make(chan bool),
	}
}

// Start starts collecting events from a watcher
func (ec *EventCollector) Start(watcher *FanotifyWatcher, timeout time.Duration) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		for {
			select {
			case event := <-watcher.Events():
				ec.mu.Lock()
				ec.events = append(ec.events, event)
				ec.mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops collecting events
func (ec *EventCollector) Stop() {
	select {
	case ec.done <- true:
	default:
	}
}

// Events returns collected events
func (ec *EventCollector) Events() []Event {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	events := make([]Event, len(ec.events))
	copy(events, ec.events)
	return events
}

// CountEventsByType counts events by type
func (ec *EventCollector) CountEventsByType() map[EventType]int {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	counts := make(map[EventType]int)
	for _, event := range ec.events {
		counts[event.Type]++
	}
	return counts
}

// WaitForEvents waits for a specific number of events or timeout
func (ec *EventCollector) WaitForEvents(count int, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return false
		default:
			ec.mu.Lock()
			if len(ec.events) >= count {
				ec.mu.Unlock()
				return true
			}
			ec.mu.Unlock()
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// MockFanotifyEvent creates a mock fanotify event for testing
func MockFanotifyEvent(mask uint64, fd int32, pid int32) *fanotifyEventMetadata {
	return &fanotifyEventMetadata{
		Event_len:    32,
		Vers:         1,
		Reserved:     0,
		Metadata_len: 24,
		Mask:         mask,
		Fd:           fd,
		Pid:          pid,
	}
}

// AssertEventTypes asserts that events contain expected types
func AssertEventTypes(t *testing.T, events []Event, expectedTypes ...EventType) {
	foundTypes := make(map[EventType]bool)
	for _, event := range events {
		foundTypes[event.Type] = true
	}

	for _, expectedType := range expectedTypes {
		assert.True(t, foundTypes[expectedType], "Expected event type %v not found", expectedType)
	}
}

// AssertEventCount asserts the number of events of a specific type
func AssertEventCount(t *testing.T, events []Event, eventType EventType, expectedCount int) {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	assert.Equal(t, expectedCount, count, "Expected %d events of type %v, got %d", expectedCount, eventType, count)
}

// AssertEventPaths asserts that events contain expected paths
func AssertEventPaths(t *testing.T, events []Event, expectedPaths ...string) {
	eventPaths := make(map[string]bool)
	for _, event := range events {
		eventPaths[event.Path] = true
	}

	for _, expectedPath := range expectedPaths {
		assert.True(t, eventPaths[expectedPath], "Expected path %s not found in events", expectedPath)
	}
}

// SetupTestEnvironment sets up a common test environment
func SetupTestEnvironment(t *testing.T) (*TestHelper, *FanotifyWatcher, string) {
	helper := NewTestHelper(t)
	watcher := helper.CreateFanotifyWatcher()
	require.NotNil(t, watcher)

	testDir := helper.CreateTestDirectory("test_env")
	helper.StartWatcher(watcher, []string{testDir})

	return helper, watcher, testDir
}

// TeardownTestEnvironment cleans up test environment
func TeardownTestEnvironment(helper *TestHelper) {
	helper.Cleanup()
}

// SkipIfFanotifyUnavailable skips test if fanotify is not available
func SkipIfFanotifyUnavailable(t *testing.T) {
	err := checkFanotifyPermissions()
	if err != nil {
		t.Skip("Fanotify not available or insufficient permissions:", err)
	}
}

// CreateTestFileHierarchy creates a complex file hierarchy for testing
func CreateTestFileHierarchy(baseDir string, files []string) error {
	for _, file := range files {
		filePath := filepath.Join(baseDir, file)

		// Create directory if needed
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}

		// Create file
		content := fmt.Sprintf("Content of %s", file)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

// ValidateEventConsistency validates that events are consistent and properly formed
func ValidateEventConsistency(t *testing.T, events []Event) {
	for i, event := range events {
		// Check required fields
		assert.NotZero(t, event.Type, "Event %d has zero type", i)
		assert.NotEmpty(t, event.Path, "Event %d has empty path", i)
		assert.True(t, event.Timestamp.After(time.Time{}), "Event %d has invalid timestamp", i)

		// Check path exists (for create/write events)
		if event.Type == EventCreate || event.Type == EventWrite {
			_, err := os.Stat(event.Path)
			if err != nil {
				t.Logf("Event %d path %s may not exist (this might be expected): %v", i, event.Path, err)
			}
		}

		// Check event type is valid
		validTypes := []EventType{EventCreate, EventWrite, EventRemove, EventRename, EventChmod}
		found := false
		for _, validType := range validTypes {
			if event.Type == validType {
				found = true
				break
			}
		}
		assert.True(t, found, "Event %d has invalid type: %v", i, event.Type)
	}
}

// BenchmarkHelper provides utilities for benchmarking
type BenchmarkHelper struct {
	tempDirs []string
}

// NewBenchmarkHelper creates a new benchmark helper
func NewBenchmarkHelper(b *testing.B) *BenchmarkHelper {
	return &BenchmarkHelper{
		tempDirs: make([]string, 0),
	}
}

// CreateTempDir creates a temporary directory for benchmark
func (bh *BenchmarkHelper) CreateTempDir(b *testing.B) string {
	dir := b.TempDir()
	bh.tempDirs = append(bh.tempDirs, dir)
	return dir
}

// CreateFileHierarchy creates a file hierarchy for benchmark
func (bh *BenchmarkHelper) CreateFileHierarchy(b *testing.B, baseDir string, numFiles int) {
	for i := 0; i < numFiles; i++ {
		filename := fmt.Sprintf("file_%d.txt", i)
		filePath := filepath.Join(baseDir, filename)
		content := fmt.Sprintf("Benchmark content for file %d", i)
		os.WriteFile(filePath, []byte(content), 0644)
	}
}

// Cleanup cleans up benchmark directories
func (bh *BenchmarkHelper) Cleanup() {
	for _, dir := range bh.tempDirs {
		os.RemoveAll(dir)
	}
}
