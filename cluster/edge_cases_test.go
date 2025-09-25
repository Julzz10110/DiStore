package cluster

import (
	"distore/synchro"
	"fmt"
	"testing"
	"time"
)

func TestEdgeCases(t *testing.T) {
	t.Run("ZeroQuorum", func(t *testing.T) {
		rom := NewReadOnlyManager(0)
		rom.UpdateNodeCount(0)

		if !rom.IsReadOnly() {
			t.Error("Zero quorum should be read-only")
		}
	})

	t.Run("SingleNodeCluster", func(t *testing.T) {
		rom := NewReadOnlyManager(1)
		rom.UpdateNodeCount(1)

		if rom.IsReadOnly() {
			t.Error("Single node with quorum 1 should be writable")
		}

		rom.UpdateNodeCount(0)
		if !rom.IsReadOnly() {
			t.Error("Single node offline should be read-only")
		}
	})

	t.Run("AllNodesFailed", func(t *testing.T) {
		fm := NewFailoverManager([]string{"node1:8080", "node2:8080"}, 100*time.Millisecond, 100*time.Millisecond)
		time.Sleep(200 * time.Millisecond)

		activeNodes := fm.GetActiveNodes()
		if len(activeNodes) != 0 {
			t.Error("All nodes should be marked as failed")
		}
	})
}

func TestMerkleTreeEdgeCases(t *testing.T) {
	t.Run("LargeKeySet", func(t *testing.T) {
		keys := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			keys[i] = fmt.Sprintf("key%d", i)
		}

		tree := synchro.NewMerkleTree(keys)
		if tree.RootHash() == "" {
			t.Error("Large tree should have root hash")
		}
	})

	t.Run("DuplicateKeys", func(t *testing.T) {
		keys := []string{"key1", "key1", "key2"}
		tree := synchro.NewMerkleTree(keys)

		// Дубликаты должны обрабатываться корректно
		if tree.RootHash() == "" {
			t.Error("Tree with duplicates should have root hash")
		}
	})
}
