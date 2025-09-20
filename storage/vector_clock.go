package storage

import (
	"sync"
	"time"
)

type VectorClock map[string]int64 // node_id -> timestamp

type VersionedValue struct {
	Value       string      `json:"value"`
	VectorClock VectorClock `json:"vector_clock"`
	Timestamp   int64       `json:"timestamp"`
}

func NewVectorClock() VectorClock {
	return make(VectorClock)
}

func (vc VectorClock) Increment(nodeID string) {
	vc[nodeID] = vc[nodeID] + 1
}

func (vc VectorClock) Merge(other VectorClock) {
	for nodeID, timestamp := range other {
		if current, exists := vc[nodeID]; !exists || timestamp > current {
			vc[nodeID] = timestamp
		}
	}
}

func (vc VectorClock) Compare(other VectorClock) string {
	vcGreater := false
	otherGreater := false

	for nodeID, vcTime := range vc {
		if otherTime, exists := other[nodeID]; exists {
			if vcTime > otherTime {
				vcGreater = true
			} else if vcTime < otherTime {
				otherGreater = true
			}
		} else if vcTime > 0 {
			vcGreater = true
		}
	}

	for nodeID, otherTime := range other {
		if _, exists := vc[nodeID]; !exists && otherTime > 0 {
			otherGreater = true
		}
	}

	if vcGreater && !otherGreater {
		return "greater"
	} else if !vcGreater && otherGreater {
		return "less"
	} else if vcGreater && otherGreater {
		return "concurrent"
	}
	return "equal"
}

type ConflictResolver struct {
	nodeID string
	mu     sync.Mutex
}

func NewConflictResolver(nodeID string) *ConflictResolver {
	return &ConflictResolver{nodeID: nodeID}
}

func (cr *ConflictResolver) ResolveConflict(current, incoming VersionedValue) VersionedValue {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	comparison := current.VectorClock.Compare(incoming.VectorClock)

	switch comparison {
	case "greater":
		return current
	case "less":
		return incoming
	case "equal":
		// If the vectors are equal, select by timestamp
		if current.Timestamp >= incoming.Timestamp {
			return current
		}
		return incoming
	case "concurrent":
		// Concurrent write conflict - use LWW (Last Write Wins)
		if current.Timestamp >= incoming.Timestamp {
			return current
		}
		return incoming
	default:
		return incoming
	}
}

func (cr *ConflictResolver) CreateVersionedValue(value string) VersionedValue {
	vc := NewVectorClock()
	vc.Increment(cr.nodeID)

	return VersionedValue{
		Value:       value,
		VectorClock: vc,
		Timestamp:   time.Now().UnixNano(),
	}
}
