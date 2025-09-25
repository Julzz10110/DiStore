package synchro

import (
	"testing"
)

func TestMerkleTree(t *testing.T) {
	t.Run("EmptyTree", func(t *testing.T) {
		tree := NewMerkleTree([]string{})
		if tree.RootHash() == "" {
			t.Error("Empty tree should have non-empty root hash")
		}
	})

	t.Run("SingleKeyTree", func(t *testing.T) {
		keys := []string{"key1"}
		tree := NewMerkleTree(keys)

		if tree.RootHash() == "" {
			t.Error("Single key tree should have root hash")
		}
	})

	t.Run("MultipleKeysTree", func(t *testing.T) {
		keys := []string{"key3", "key1", "key2"} // intentionally unsorted
		tree := NewMerkleTree(keys)

		if tree.RootHash() == "" {
			t.Error("Multi key tree should have root hash")
		}
	})

	t.Run("TreeDeterminism", func(t *testing.T) {
		keys1 := []string{"key1", "key2", "key3"}
		keys2 := []string{"key3", "key1", "key2"} // different order

		tree1 := NewMerkleTree(keys1)
		tree2 := NewMerkleTree(keys2)

		if tree1.RootHash() != tree2.RootHash() {
			t.Error("Merkle trees should be deterministic regardless of key order")
		}
	})
}

func TestMerkleTreeComparison(t *testing.T) {
	t.Run("IdenticalTrees", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		tree1 := NewMerkleTree(keys)
		tree2 := NewMerkleTree(keys)

		diffs := CompareTrees(tree1, tree2)
		if len(diffs) != 0 {
			t.Errorf("Identical trees should have no diffs, got %d", len(diffs))
		}
	})

	t.Run("DifferentTrees", func(t *testing.T) {
		tree1 := NewMerkleTree([]string{"key1", "key2"})
		tree2 := NewMerkleTree([]string{"key1", "key3"})

		diffs := CompareTrees(tree1, tree2)
		if len(diffs) == 0 {
			t.Error("Different trees should have diffs")
		}
	})

	t.Run("EmptyVsFullTree", func(t *testing.T) {
		tree1 := NewMerkleTree([]string{})
		tree2 := NewMerkleTree([]string{"key1"})

		diffs := CompareTrees(tree1, tree2)
		if len(diffs) == 0 {
			t.Error("Empty and full trees should have diffs")
		}
	})
}
