package cluster

import (
	"testing"
)

func TestReadOnlyManager(t *testing.T) {
	t.Run("QuorumCalculation", func(t *testing.T) {
		rom := NewReadOnlyManager(3) // Кворум = 3

		rom.UpdateNodeCount(4) // 4 активных ноды
		if rom.IsReadOnly() {
			t.Error("Should not be read-only with 4/3 nodes")
		}

		rom.UpdateNodeCount(2) // 2 активных ноды
		if !rom.IsReadOnly() {
			t.Error("Should be read-only with 2/3 nodes")
		}
	})

	t.Run("CanWriteCheck", func(t *testing.T) {
		rom := NewReadOnlyManager(2)

		rom.UpdateNodeCount(3)
		if !rom.CanWrite() {
			t.Error("Should be able to write with quorum")
		}

		rom.UpdateNodeCount(1)
		if rom.CanWrite() {
			t.Error("Should not be able to write without quorum")
		}
	})

	t.Run("StatusReporting", func(t *testing.T) {
		rom := NewReadOnlyManager(3)
		rom.UpdateNodeCount(2)

		status := rom.GetStatus()
		if status["read_only"] != true {
			t.Error("Status should report read-only")
		}
		if status["active_nodes"] != 2 {
			t.Error("Status should report correct node count")
		}
	})
}
