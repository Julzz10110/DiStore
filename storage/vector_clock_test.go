package storage

import (
	"testing"
)

func TestVectorClock(t *testing.T) {
	t.Run("BasicOperations", func(t *testing.T) {
		vc := NewVectorClock()

		vc.Increment("node1")
		if vc["node1"] != 1 {
			t.Errorf("Expected 1, got %d", vc["node1"])
		}

		vc.Increment("node1")
		if vc["node1"] != 2 {
			t.Errorf("Expected 2, got %d", vc["node1"])
		}
	})

	t.Run("ClockComparison", func(t *testing.T) {
		vc1 := NewVectorClock()
		vc2 := NewVectorClock()

		vc1.Increment("node1")
		vc1.Increment("node1")

		vc2.Increment("node1")

		result := vc1.Compare(vc2)
		if result != "greater" {
			t.Errorf("Expected greater, got %s", result)
		}
	})

	t.Run("ClockMerge", func(t *testing.T) {
		vc1 := NewVectorClock()
		vc2 := NewVectorClock()

		vc1.Increment("node1")
		vc2.Increment("node2")
		vc2.Increment("node2")

		vc1.Merge(vc2)

		if vc1["node1"] != 1 || vc1["node2"] != 2 {
			t.Errorf("Merge failed: %v", vc1)
		}
	})
}

func TestConflictResolver(t *testing.T) {
	resolver := NewConflictResolver("test-node")

	t.Run("CreateVersionedValue", func(t *testing.T) {
		vv := resolver.CreateVersionedValue("test-value")

		if vv.Value != "test-value" {
			t.Errorf("Expected test-value, got %s", vv.Value)
		}

		if vv.VectorClock["test-node"] != 1 {
			t.Errorf("Expected clock value 1, got %d", vv.VectorClock["test-node"])
		}
	})

	t.Run("ResolveConflict", func(t *testing.T) {
		vv1 := VersionedValue{
			Value:       "value1",
			VectorClock: NewVectorClock(),
			Timestamp:   1000,
		}
		vv1.VectorClock.Increment("node1")

		vv2 := VersionedValue{
			Value:       "value2",
			VectorClock: NewVectorClock(),
			Timestamp:   2000,
		}
		vv2.VectorClock.Increment("node2")

		// VV2 should win due to the later timestamp
		result := resolver.ResolveConflict(vv1, vv2)
		if result.Value != "value2" {
			t.Errorf("Expected value2, got %s", result.Value)
		}
	})
}
