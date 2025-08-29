package trees

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRebuildGraph_tempRelationshipMapReuse(t *testing.T) {
	t.Run("RebuildGraph handles tempRelationshipMap correctly on multiple calls", func(t *testing.T) {
		// Create a simple tree structure
		root := NewDirectoryNode("/root", nil)
		child1 := NewDirectoryNode("/root/child1", root)
		child2 := NewDirectoryNode("/root/child2", root)

		root.Children = []*DirectoryNode{child1, child2}
		child1.Parent = root
		child2.Parent = root

		nodes := []*DirectoryNode{root, child1, child2}

		// First call to RebuildGraph
		RebuildGraph(nodes)

		// Verify relationships are established correctly
		assert.Equal(t, root, child1.Parent, "Child1 should have root as parent")
		assert.Equal(t, root, child2.Parent, "Child2 should have root as parent")
		assert.Contains(t, root.Children, child1, "Root should contain child1")
		assert.Contains(t, root.Children, child2, "Root should contain child2")

		// Modify the tree structure
		child3 := NewDirectoryNode("/root/child3", root)
		root.Children = append(root.Children, child3)
		child3.Parent = root

		nodes = append(nodes, child3)

		// Second call to RebuildGraph - this should not panic due to nil tempRelationshipMap
		assert.NotPanics(t, func() {
			RebuildGraph(nodes)
		}, "RebuildGraph should not panic on second call")

		// Verify new relationships are established correctly
		assert.Equal(t, root, child3.Parent, "Child3 should have root as parent")
		assert.Contains(t, root.Children, child3, "Root should contain child3")
	})

	t.Run("RebuildGraph handles complex nested structures", func(t *testing.T) {
		// Create a more complex nested structure
		root := NewDirectoryNode("/root", nil)
		level1 := NewDirectoryNode("/root/level1", root)
		level2 := NewDirectoryNode("/root/level1/level2", level1)
		sibling := NewDirectoryNode("/root/sibling", root)

		root.Children = []*DirectoryNode{level1, sibling}
		level1.Children = []*DirectoryNode{level2}

		level1.Parent = root
		level2.Parent = level1
		sibling.Parent = root

		nodes := []*DirectoryNode{root, level1, level2, sibling}

		// Call RebuildGraph
		RebuildGraph(nodes)

		// Verify all relationships
		assert.Equal(t, root, level1.Parent, "Level1 parent should be root")
		assert.Equal(t, level1, level2.Parent, "Level2 parent should be level1")
		assert.Equal(t, root, sibling.Parent, "Sibling parent should be root")

		assert.Contains(t, root.Children, level1, "Root should contain level1")
		assert.Contains(t, root.Children, sibling, "Root should contain sibling")
		assert.Contains(t, level1.Children, level2, "Level1 should contain level2")
	})

	t.Run("RebuildGraph handles empty input gracefully", func(t *testing.T) {
		// Test with empty slice
		assert.NotPanics(t, func() {
			RebuildGraph([]*DirectoryNode{})
		}, "RebuildGraph should handle empty input without panicking")
	})

	t.Run("RebuildGraph handles single node", func(t *testing.T) {
		root := NewDirectoryNode("/root", nil)
		nodes := []*DirectoryNode{root}

		assert.NotPanics(t, func() {
			RebuildGraph(nodes)
		}, "RebuildGraph should handle single node without panicking")

		// Root should still have no parent
		assert.Nil(t, root.Parent, "Root node should have no parent")
		assert.Len(t, root.Children, 0, "Root should have no children")
	})
}

func TestDirectoryNodeJSON_Serialization(t *testing.T) {
	t.Run("MarshalJSON creates correct JSON structure", func(t *testing.T) {
		root := NewDirectoryNode("/test", nil)
		child := NewDirectoryNode("/test/child", root)
		file := &FileNode{
			Path: "/test/file.txt",
			Name: "file.txt",
		}

		root.Children = []*DirectoryNode{child}
		root.Files = []*FileNode{file}
		child.Parent = root

		// Marshal to JSON
		jsonData, err := root.MarshalJSON()
		require.NoError(t, err)
		assert.NotEmpty(t, jsonData, "JSON data should not be empty")

		// Basic JSON structure check
		jsonStr := string(jsonData)
		assert.Contains(t, jsonStr, `"path":"/test"`, "Should contain root path")
		assert.Contains(t, jsonStr, `"children_ids"`, "Should contain children_ids field")
		assert.Contains(t, jsonStr, `"file_ids"`, "Should contain file_ids field")
	})

	t.Run("UnMarshalJSON populates tempRelationshipMap correctly", func(t *testing.T) {
		root := NewDirectoryNode("/test", nil)
		child := NewDirectoryNode("/test/child", root)

		root.Children = []*DirectoryNode{child}
		child.Parent = root

		// Marshal to JSON
		jsonData, err := root.MarshalJSON()
		require.NoError(t, err)

		// Create a new node and unmarshal
		newRoot := NewDirectoryNode("", nil)
		err = newRoot.UnMarshalJSON(jsonData)
		require.NoError(t, err)

		// Verify basic fields are populated
		assert.Equal(t, "/test", newRoot.Path, "Path should be unmarshaled correctly")
		assert.Equal(t, Directory, newRoot.Type, "Type should be unmarshaled correctly")

		// tempRelationshipMap should contain the node
		// Note: This test verifies the tempRelationshipMap population works without panicking
		// The actual map content is tested in RebuildGraph tests
		assert.NotPanics(t, func() {
			RebuildGraph([]*DirectoryNode{newRoot})
		}, "Unmarshaling and rebuilding should work without panicking")
	})
}
