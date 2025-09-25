package synchro

import (
	"distore/storage"
	"fmt"
	"log"
	"sync"
	"time"
)

type RepairManager struct {
	storage      storage.Storage
	syncInterval time.Duration
	mu           sync.Mutex
	running      bool
}

func NewRepairManager(storage storage.Storage, syncInterval time.Duration) *RepairManager {
	return &RepairManager{
		storage:      storage,
		syncInterval: syncInterval,
	}
}

func (rm *RepairManager) Start() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.running {
		return
	}

	rm.running = true
	go rm.backgroundRepair()
}

func (rm *RepairManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.running = false
}

func (rm *RepairManager) backgroundRepair() {
	ticker := time.NewTicker(rm.syncInterval)
	defer ticker.Stop()

	for range ticker.C {
		if !rm.running {
			break
		}

		err := rm.performSync()
		if err != nil {
			log.Printf("Background sync failed: %v", err)
		}
	}
}

func (rm *RepairManager) performSync() error {
	// Получаем все локальные ключи
	localItems, err := rm.storage.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get local items: %w", err)
	}

	// Здесь должна быть логика сравнения с другими нодами
	// Для демо просто логируем
	log.Printf("Background sync: %d local items", len(localItems))

	// Создаем Merkle tree для локальных данных
	localKeys := make([]string, len(localItems))
	for i, item := range localItems {
		localKeys[i] = item.Key
	}

	localTree := NewMerkleTree(localKeys)
	log.Printf("Merkle root hash: %s", localTree.RootHash())

	return nil
}

func (rm *RepairManager) SyncWithNode(nodeURL string) error {
	// Реализация синхронизации с конкретной нодой
	log.Printf("Syncing with node: %s", nodeURL)

	// 1. Обмен Merkle trees
	// 2. Выявление расхождений
	// 3. Исправление расхождений

	return nil
}

func (rm *RepairManager) RepairKey(key string, value string) error {
	// Принудительное исправление ключа
	return rm.storage.Set(key, value)
}
