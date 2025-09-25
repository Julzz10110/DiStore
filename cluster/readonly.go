package cluster

import (
	"sync"
	"time"
)

type ReadOnlyManager struct {
	mu          sync.RWMutex
	isReadOnly  bool
	quorumSize  int
	activeNodes int
	lastCheck   time.Time
}

func NewReadOnlyManager(quorumSize int) *ReadOnlyManager {
	return &ReadOnlyManager{
		quorumSize: quorumSize,
		isReadOnly: false,
	}
}

func (rom *ReadOnlyManager) UpdateNodeCount(activeNodes int) {
	rom.mu.Lock()
	defer rom.mu.Unlock()

	rom.activeNodes = activeNodes
	rom.lastCheck = time.Now()

	// Включаем read-only mode если нет кворума
	rom.isReadOnly = activeNodes < rom.quorumSize
}

func (rom *ReadOnlyManager) IsReadOnly() bool {
	rom.mu.RLock()
	defer rom.mu.RUnlock()
	return rom.isReadOnly
}

func (rom *ReadOnlyManager) CanWrite() bool {
	rom.mu.RLock()
	defer rom.mu.RUnlock()
	return !rom.isReadOnly
}

func (rom *ReadOnlyManager) GetStatus() map[string]interface{} {
	rom.mu.RLock()
	defer rom.mu.RUnlock()

	return map[string]interface{}{
		"read_only":    rom.isReadOnly,
		"active_nodes": rom.activeNodes,
		"quorum_size":  rom.quorumSize,
		"last_check":   rom.lastCheck,
	}
}
