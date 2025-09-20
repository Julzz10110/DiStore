package replication

import (
	"sync"
	"time"
)

type WriteTimestamp struct {
	Timestamp int64
	NodeID    string
}

type ConsistencyManager struct {
	mu                  sync.RWMutex
	lastWriteTimestamps map[string]WriteTimestamp // key -> last write info
	clientSessions      map[string]time.Time      // client_id -> last activity
}

func NewConsistencyManager() *ConsistencyManager {
	return &ConsistencyManager{
		lastWriteTimestamps: make(map[string]WriteTimestamp),
		clientSessions:      make(map[string]time.Time),
	}
}

func (cm *ConsistencyManager) RecordWrite(key, nodeID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.lastWriteTimestamps[key] = WriteTimestamp{
		Timestamp: time.Now().UnixNano(),
		NodeID:    nodeID,
	}
}

func (cm *ConsistencyManager) GetLastWriteTime(key string) (WriteTimestamp, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	timestamp, exists := cm.lastWriteTimestamps[key]
	return timestamp, exists
}

func (cm *ConsistencyManager) EnsureReadYourWrites(clientID, key string) (string, error) {
	cm.mu.RLock()
	lastActivity, hasSession := cm.clientSessions[clientID]
	lastWrite, hasWrite := cm.lastWriteTimestamps[key]
	cm.mu.RUnlock()

	preferredNode := ""

	if hasSession && hasWrite {
		// If the client recently wrote, try to read from the same replica
		if time.Since(lastActivity) < 5*time.Second {
			preferredNode = lastWrite.NodeID
			// TODO: add logic for sticky reading here
		}
	}

	return preferredNode, nil
}

func (cm *ConsistencyManager) UpdateClientSession(clientID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.clientSessions[clientID] = time.Now()
}

func (cm *ConsistencyManager) CleanupOldSessions(maxAge time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for clientID, lastActivity := range cm.clientSessions {
		if now.Sub(lastActivity) > maxAge {
			delete(cm.clientSessions, clientID)
		}
	}
}
