package trees

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectoryTree_PathSemantics(t *testing.T) {
	t.Run("AddDirectory creates nodes with full absolute paths", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		// Test adding a directory with relative path
		node1, err := tree.AddDirectory("home/user")
		require.NoError(t, err)
		assert.Equal(t, "/home/user", node1.Path, "DirectoryNode.Path should contain full absolute path")

		// Test adding a subdirectory
		node2, err := tree.AddDirectory("home/user/documents")
		require.NoError(t, err)
		assert.Equal(t, "/home/user/documents", node2.Path, "Child directory should have full absolute path")

		// Verify parent-child relationship
		assert.Equal(t, node1, node2.Parent, "Parent relationship should be maintained")
		assert.Contains(t, node1.Children, node2, "Parent should contain child in Children slice")
	})

	t.Run("AddDirectory handles absolute paths correctly", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		// Test adding with absolute path
		node, err := tree.AddDirectory("/var/log")
		require.NoError(t, err)
		assert.Equal(t, "/var/log", node.Path, "Absolute path should be preserved")

		// Test adding subdirectory of absolute path
		node2, err := tree.AddDirectory("/var/log/apache2")
		require.NoError(t, err)
		assert.Equal(t, "/var/log/apache2", node2.Path, "Child of absolute path should be full absolute path")
	})

	t.Run("AddFile creates file with full absolute path", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		// Create parent directory first
		parent, err := tree.AddDirectory("home/user")
		require.NoError(t, err)

		// Add file to the tree
		err = tree.AddFile("home/user", "/home/user/document.txt", 1024, time.Now())
		require.NoError(t, err)

		// Verify file was added to correct directory
		assert.Len(t, parent.Files, 1, "Parent directory should contain the file")
		assert.Equal(t, "/home/user/document.txt", parent.Files[0].Path, "File should have full absolute path")
		assert.Equal(t, "document.txt", parent.Files[0].Name, "File should have correct name")
	})

	t.Run("FindOrCreatePath maintains path semantics", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		// Test with string slice representing path segments
		pathSegments := []string{"usr", "local", "bin"}
		node := tree.FindOrCreatePath(pathSegments)

		// The path should be relative to the root, not absolute
		assert.Equal(t, "bin", node.Path, "FindOrCreatePath creates relative path segments")
		assert.Equal(t, Directory, node.Type, "Node should be of Directory type")

		// Verify the full path through parent traversal
		var pathParts []string
		current := node
		for current != nil {
			if current.Path != "/" { // Skip root path separator
				pathParts = append([]string{current.Path}, pathParts...)
			}
			current = current.Parent
		}
		fullPath := "/" + strings.Join(pathParts, "/")
		assert.Equal(t, "/usr/local/bin", fullPath, "Full path should be constructed correctly")
	})

	t.Run("Flatten returns all paths with correct semantics", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		// Build a small tree structure
		tree.AddDirectory("home/user")
		tree.AddFile("home/user", "/home/user/file1.txt", 100, time.Now())
		tree.AddFile("home/user", "/home/user/file2.txt", 200, time.Now())

		paths := tree.Flatten()

		// Should contain directory and file paths
		assert.Contains(t, paths, "/", "Should contain root path")
		assert.Contains(t, paths, "/home/user", "Should contain directory path")
		assert.Contains(t, paths, "/home/user/file1.txt", "Should contain file path")
		assert.Contains(t, paths, "/home/user/file2.txt", "Should contain file path")
	})

	t.Run("Path normalization is consistent", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		// Test various path formats that should normalize to the same result
		testPaths := []string{
			"home/user/../user", // with .. components
			"home//user",        // with double slashes
			"./home/user",       // with current directory
		}

		for _, path := range testPaths {
			node, err := tree.AddDirectory(path)
			require.NoError(t, err)
			assert.Equal(t, "/home/user", node.Path, "Path should be normalized to /home/user")
		}
	})
}

func TestDirectoryTree_AddDirectory_PathValidation(t *testing.T) {
	t.Run("AddDirectory handles empty path gracefully", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		node, err := tree.AddDirectory("")
		assert.Error(t, err, "Empty path should return error")
		assert.Nil(t, node, "Empty path should not create node")
	})

	t.Run("AddDirectory handles root path correctly", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		node, err := tree.AddDirectory("/")
		require.NoError(t, err)
		assert.Equal(t, "/", node.Path, "Root path should be preserved")
		assert.Equal(t, tree.Root, node, "Root path should return root node")
	})

	t.Run("AddDirectory creates intermediate directories", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		// Add deep nested path
		node, err := tree.AddDirectory("a/b/c/d/e")
		require.NoError(t, err)
		assert.Equal(t, "/a/b/c/d/e", node.Path, "Deep nested path should be created with full path")

		// Verify intermediate directories exist
		aNode := tree.Root.Children[0]
		assert.Equal(t, "/a", aNode.Path)
		assert.Equal(t, "/a/b", aNode.Children[0].Path)
		assert.Equal(t, "/a/b/c", aNode.Children[0].Children[0].Path)
	})
}

func TestDirectoryTree_FindByPath_PathSemantics(t *testing.T) {
	t.Run("FindByPath works with full absolute paths", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		// Create a directory
		originalNode, err := tree.AddDirectory("test/path")
		require.NoError(t, err)

		// Find it back using full path
		foundNode, exists := tree.FindByPath("/test/path")
		assert.True(t, exists, "Should find existing path")
		assert.Equal(t, originalNode, foundNode, "Found node should be the same as original")
	})

	t.Run("FindByPath returns false for non-existent paths", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		foundNode, exists := tree.FindByPath("/non/existent/path")
		assert.False(t, exists, "Should not find non-existent path")
		assert.Nil(t, foundNode, "Should return nil for non-existent path")
	})

	t.Run("FindByPathPrefix works with path prefixes", func(t *testing.T) {
		tree := NewDirectoryTree(WithRoot("/"))

		// Create multiple directories with common prefix
		tree.AddDirectory("test/path1")
		tree.AddDirectory("test/path2")
		tree.AddDirectory("other/path")

		// Find by prefix - this will include "/test" as it's considered a prefix match by the radix tree
		results := tree.FindByPathPrefix("/test")

		// Should find at least path1 and path2 (and possibly /test itself)
		assert.True(t, len(results) >= 2, "Should find at least 2 directories with /test prefix")

		// Verify all results start with the prefix
		for _, result := range results {
			assert.True(t, strings.HasPrefix(result.Path, "/test"), "Path should start with /test prefix")
		}

		// Specifically check that we have the expected subdirectories
		pathMap := make(map[string]bool)
		for _, result := range results {
			pathMap[result.Path] = true
		}

		assert.True(t, pathMap["/test/path1"], "Should include /test/path1")
		assert.True(t, pathMap["/test/path2"], "Should include /test/path2")
	})
}
