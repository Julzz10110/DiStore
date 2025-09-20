package replication

import (
	"testing"
	"time"
)

func TestConsistencyManager(t *testing.T) {
	cm := NewConsistencyManager()

	t.Run("RecordAndGetWrite", func(t *testing.T) {
		cm.RecordWrite("key1", "node1")

		timestamp, exists := cm.GetLastWriteTime("key1")
		if !exists {
			t.Error("Expected to find write timestamp")
		}

		if timestamp.NodeID != "node1" {
			t.Errorf("Expected node1, got %s", timestamp.NodeID)
		}

		if timestamp.Timestamp == 0 {
			t.Error("Expected non-zero timestamp")
		}
	})

	t.Run("ReadYourWritesConsistency", func(t *testing.T) {
		cm.RecordWrite("key2", "node2")
		cm.UpdateClientSession("client1")

		preferredNode, err := cm.EnsureReadYourWrites("client1", "key2")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if preferredNode != "node2" {
			t.Errorf("Expected node2, got %s", preferredNode)
		}
	})

	t.Run("SessionCleanup", func(t *testing.T) {
		cm.UpdateClientSession("old-client")
		// Simulate session age
		cm.cleanupOldSessionsTest(1 * time.Millisecond)

		time.Sleep(2 * time.Millisecond)
		cm.CleanupOldSessions(1 * time.Millisecond)

		// Check if the session is cleared
		cm.mu.RLock()
		_, exists := cm.clientSessions["old-client"]
		cm.mu.RUnlock()

		if exists {
			t.Error("Expected old session to be cleaned up")
		}
	})
}

// Helper method for testing
func (cm *ConsistencyManager) cleanupOldSessionsTest(maxAge time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for clientID, lastActivity := range cm.clientSessions {
		if now.Sub(lastActivity) > maxAge {
			delete(cm.clientSessions, clientID)
		}
	}
}
