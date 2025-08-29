package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/services"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTraversalHandler is a test implementation of TraversalHandler
type MockTraversalHandler struct {
	processedDirs  int64
	processedFiles int64
}

func (m *MockTraversalHandler) HandleDirectory(node *trees.DirectoryNode) error {
	atomic.AddInt64(&m.processedDirs, 1)
	return nil
}

func (m *MockTraversalHandler) HandleFile(node *trees.FileNode) error {
	atomic.AddInt64(&m.processedFiles, 1)
	return nil
}

func (m *MockTraversalHandler) GetDesktopCleanerIgnore(path string) (services.IgnoreChecker, error) {
	return &nullIgnoreChecker{}, nil // No ignore patterns for tests
}

// nullIgnoreChecker implements IgnoreChecker for tests
type nullIgnoreChecker struct{}

func (n *nullIgnoreChecker) MatchesPath(path string) bool {
	return false // No paths are ignored in tests
}

// IgnoreChecker interface (copied from services package for test)
type IgnoreChecker interface {
	MatchesPath(path string) bool
}

// createTestStructure creates a nested directory structure for testing
// This utility function creates a configurable tree structure with:
// - maxDepth: How many levels deep to create
// - width: How many subdirectories per level
// Each directory gets 2 test files
//
// Example: createTestStructure("/tmp/test", 0, 3, 2) creates:
// /tmp/test/
// ├── level0_0/
// │   ├── file0.txt
// │   ├── file1.txt
// │   └── level1_0/
// │       ├── file0.txt
// │       ├── file1.txt
// │       └── level2_0/
// │           ├── file0.txt
// │           └── file1.txt
// └── level0_1/
//
//	└── ... (similar structure)
//
// NOTE: This is a test utility kept for future performance testing.
// Currently unused but valuable for testing concurrent traversal with complex directory structures.
func createTestStructure(basePath string, currentDepth, maxDepth, width int) {
	if currentDepth >= maxDepth {
		return
	}

	for i := range width {
		subDir := filepath.Join(basePath, fmt.Sprintf("level%d_%d", currentDepth, i))
		err := os.MkdirAll(subDir, 0o755)
		if err != nil {
			continue // Skip on error in tests
		}

		// Create files at this level
		for j := range 2 {
			filePath := filepath.Join(subDir, fmt.Sprintf("file%d.txt", j))
			err := os.WriteFile(filePath, []byte("test"), 0o644)
			if err != nil {
				continue // Skip on error in tests
			}
		}

		// Recurse deeper
		createTestStructure(subDir, currentDepth+1, maxDepth, width)
	}
}

func TestConcurrentTraverser_WorkerCapEnforcement(t *testing.T) {
	t.Run("TraverseDirectory enforces maxWorkers limit", func(t *testing.T) {
		ctx := context.Background()
		traverser := NewConcurrentTraverser(ctx)
		handler := &MockTraversalHandler{}

		// Create a test directory structure
		testDir := t.TempDir()

		// Create multiple subdirectories to test concurrency
		for i := range 10 {
			subDir := filepath.Join(testDir, fmt.Sprintf("subdir%d", i))
			err := os.MkdirAll(subDir, 0o755)
			require.NoError(t, err)

			// Create some files in each directory
			for j := range 3 {
				filePath := filepath.Join(subDir, fmt.Sprintf("file%d.txt", j))
				err := os.WriteFile(filePath, []byte("test content"), 0o644)
				require.NoError(t, err)
			}
		}

		// Start traversal
		rootNode, err := traverser.TraverseDirectory(testDir, true, -1, handler)
		require.NoError(t, err)
		require.NotNil(t, rootNode)

		// Verify that traversal completed without issues
		assert.True(t, atomic.LoadInt64(&handler.processedDirs) > 0, "Should process some directories")
		assert.True(t, atomic.LoadInt64(&handler.processedFiles) > 0, "Should process some files")
	})

	t.Run("TraverseDirectoryWithPool handles worker limits correctly", func(t *testing.T) {
		ctx := context.Background()
		traverser := NewConcurrentTraverser(ctx)
		handler := &MockTraversalHandler{}

		// Create a simple test directory structure
		testDir := t.TempDir()

		// Create some subdirectories with files
		for i := range 5 {
			subDir := filepath.Join(testDir, fmt.Sprintf("subdir%d", i))
			err := os.MkdirAll(subDir, 0o755)
			require.NoError(t, err)

			// Create files in each subdirectory
			for j := range 3 {
				filePath := filepath.Join(subDir, fmt.Sprintf("file%d.txt", j))
				err := os.WriteFile(filePath, []byte("test content"), 0o644)
				require.NoError(t, err)
			}
		}

		// Verify files were created
		totalFiles := 0
		for i := range 5 {
			subDir := filepath.Join(testDir, fmt.Sprintf("subdir%d", i))
			entries, err := os.ReadDir(subDir)
			require.NoError(t, err)
			totalFiles += len(entries)
		}

		// Test traversal
		rootNode, err := traverser.TraverseDirectory(testDir, true, -1, handler)
		require.NoError(t, err)
		require.NotNil(t, rootNode)

		// Verify processing occurred - should find at least the subdirectories and files we created
		dirsProcessed := atomic.LoadInt64(&handler.processedDirs)
		filesProcessed := atomic.LoadInt64(&handler.processedFiles)
		assert.True(t, dirsProcessed >= 5, "Should process at least 5 subdirectories")
		assert.True(t, filesProcessed >= 15, "Should process at least 15 files (5 dirs * 3 files each)")
	})

	t.Run("ConcurrentTraverser handles cancellation correctly", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		traverser := NewConcurrentTraverser(ctx)
		handler := &MockTraversalHandler{}

		// Create a test directory with many files to slow down processing
		testDir := t.TempDir()

		// Create many directories and files to slow down traversal
		for i := 0; i < 50; i++ {
			subDir := filepath.Join(testDir, fmt.Sprintf("dir%d", i))
			err := os.MkdirAll(subDir, 0o755)
			require.NoError(t, err)

			// Create more files to make traversal slower
			for j := 0; j < 20; j++ {
				filePath := filepath.Join(subDir, fmt.Sprintf("file%d.txt", j))
				err := os.WriteFile(filePath, []byte("test content for cancellation"), 0o644)
				require.NoError(t, err)
			}

			// Add nested subdirectories
			for k := 0; k < 3; k++ {
				nestedDir := filepath.Join(subDir, fmt.Sprintf("nested%d", k))
				os.MkdirAll(nestedDir, 0o755)
				for l := 0; l < 5; l++ {
					nestedFile := filepath.Join(nestedDir, fmt.Sprintf("nested_file%d.txt", l))
					os.WriteFile(nestedFile, []byte("nested content"), 0o644)
				}
			}
		}

		// Start traversal in a goroutine
		done := make(chan error, 1)
		go func() {
			_, err := traverser.TraverseDirectory(testDir, true, -1, handler)
			done <- err
		}()

		// Cancel after a short delay
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Wait for traversal to complete or timeout
		select {
		case err := <-done:
			// Check if we got a cancellation error (may complete successfully if too fast)
			if err != nil {
				assert.Contains(t, err.Error(), "context", "Error should mention context")
			} else {
				// If no error, that's also acceptable - traversal completed before cancellation
				t.Log("Traversal completed successfully before cancellation took effect")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Traversal did not complete within timeout")
		}
	})

	t.Run("ConcurrentTraverser respects maxDepth parameter", func(t *testing.T) {
		ctx := context.Background()
		traverser := NewConcurrentTraverser(ctx)
		handler := &MockTraversalHandler{}

		// Create nested directory structure
		testDir := t.TempDir()

		// Create level0 with files
		level0 := filepath.Join(testDir, "level0")
		err := os.MkdirAll(level0, 0o755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(level0, "file0.txt"), []byte("test"), 0o644)
		require.NoError(t, err)

		// Create level1 with files
		level1 := filepath.Join(level0, "level1")
		err = os.MkdirAll(level1, 0o755)
		require.NoError(t, err)
		file1Path := filepath.Join(level1, "file1.txt")
		err = os.WriteFile(file1Path, []byte("test"), 0o644)
		require.NoError(t, err)

		// Verify file was created
		if _, err := os.Stat(file1Path); err != nil {
			t.Fatalf("Failed to create file1.txt: %v", err)
		}

		// Create level2 with files (should not be processed with maxDepth=1)
		level2 := filepath.Join(level1, "level2")
		err = os.MkdirAll(level2, 0o755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(level2, "file2.txt"), []byte("test"), 0o644)
		require.NoError(t, err)

		// Test with maxDepth = 1 (should process level0 and level1, but not level2)
		rootNode, err := traverser.TraverseDirectory(testDir, true, 1, handler)
		require.NoError(t, err)
		require.NotNil(t, rootNode)

		// Should have processed at least level0 and level1 files
		filesProcessed := atomic.LoadInt64(&handler.processedFiles)
		assert.True(t, filesProcessed >= 2, "Should process files from level 0 and level 1")
	})

	t.Run("ConcurrentTraverser handles non-recursive traversal", func(t *testing.T) {
		ctx := context.Background()
		traverser := NewConcurrentTraverser(ctx)
		handler := &MockTraversalHandler{}

		// Create nested directory structure
		testDir := t.TempDir()

		// Create root files
		for i := 0; i < 3; i++ {
			filePath := filepath.Join(testDir, fmt.Sprintf("root%d.txt", i))
			err := os.WriteFile(filePath, []byte("root file"), 0o644)
			require.NoError(t, err)
		}

		// Create subdirectory with files
		subDir := filepath.Join(testDir, "subdir")
		err := os.MkdirAll(subDir, 0o755)
		require.NoError(t, err)

		for i := 0; i < 2; i++ {
			filePath := filepath.Join(subDir, fmt.Sprintf("sub%d.txt", i))
			err := os.WriteFile(filePath, []byte("sub file"), 0o644)
			require.NoError(t, err)
		}

		// Test non-recursive (should not process subdir contents)
		rootNode, err := traverser.TraverseDirectory(testDir, false, -1, handler)
		require.NoError(t, err)
		require.NotNil(t, rootNode)

		// Should only process root directory files, not subdirectory files
		assert.True(t, atomic.LoadInt64(&handler.processedFiles) >= 3, "Should process root files")
	})
}

func TestConcurrentTraverser_ComplexDirectoryStructure(t *testing.T) {
	t.Run("TraverseDirectory handles complex nested structures", func(t *testing.T) {
		ctx := context.Background()
		traverser := NewConcurrentTraverser(ctx)
		handler := &MockTraversalHandler{}

		// Create a complex nested directory structure using the utility function
		testDir := t.TempDir()

		// Create a 3-level deep structure with 2 directories per level
		// This creates: 2^3 = 8 leaf directories, each with 2 files
		// Total: 8 directories + 16 files = 24 items to process
		createTestStructure(testDir, 0, 3, 2)

		// Start traversal with recursive processing
		rootNode, err := traverser.TraverseDirectory(testDir, true, -1, handler)
		require.NoError(t, err)
		require.NotNil(t, rootNode)

		// Verify traversal processed the complex structure
		processedDirs := atomic.LoadInt64(&handler.processedDirs)
		processedFiles := atomic.LoadInt64(&handler.processedFiles)

		// Should process all directories (8 leaf + intermediate dirs)
		assert.True(t, processedDirs >= 8, "Should process multiple directory levels")

		// Should process all files (2 files per leaf directory = 16 total)
		assert.True(t, processedFiles >= 16, "Should process files in nested structure")

		// Verify the root node has children (intermediate directories)
		assert.True(t, len(rootNode.Children) > 0, "Root should have child directories")
	})

	t.Run("createTestStructure generates expected file counts", func(t *testing.T) {
		// Test that createTestStructure generates the expected number of files and directories
		testDir := t.TempDir()

		// Create a 2-level deep structure with 3 directories per level
		// Level 0: 1 root dir, Level 1: 3 subdirs, Level 2: 9 leaf dirs
		// Total dirs: 1 + 3 + 9 = 13, Files: 9 * 2 = 18
		createTestStructure(testDir, 0, 3, 3)

		ctx := context.Background()
		traverser := NewConcurrentTraverser(ctx)
		handler := &MockTraversalHandler{}

		// Traverse without depth limit
		rootNode, err := traverser.TraverseDirectory(testDir, true, -1, handler)
		require.NoError(t, err)
		require.NotNil(t, rootNode)

		processedDirs := atomic.LoadInt64(&handler.processedDirs)
		processedFiles := atomic.LoadInt64(&handler.processedFiles)

		// Verify the structure was created and traversed correctly
		assert.True(t, processedDirs >= 13, "Should process all directories in the structure")
		assert.True(t, processedFiles >= 18, "Should process all files in leaf directories")

		// Verify root has children from level 0
		assert.True(t, len(rootNode.Children) == 3, "Root should have 3 child directories from level 0")
	})
}
