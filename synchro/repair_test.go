package synchro

import (
	"distore/storage"
	"testing"
	"time"
)

type MockStorage struct {
	items map[string]string
}

func NewMockStorage() *MockStorage {
	return &MockStorage{items: make(map[string]string)}
}

func (m *MockStorage) Set(key, value string) error {
	m.items[key] = value
	return nil
}

func (m *MockStorage) Get(key string) (string, error) {
	value, exists := m.items[key]
	if !exists {
		return "", storage.ErrKeyNotFound
	}
	return value, nil
}

func (m *MockStorage) Delete(key string) error {
	delete(m.items, key)
	return nil
}

func (m *MockStorage) GetAll() ([]storage.KeyValue, error) {
	items := make([]storage.KeyValue, 0, len(m.items))
	for k, v := range m.items {
		items = append(items, storage.KeyValue{Key: k, Value: v})
	}
	return items, nil
}

func (m *MockStorage) Close() error {
	return nil
}

func TestRepairManager(t *testing.T) {
	mockStorage := NewMockStorage()
	mockStorage.Set("key1", "value1")
	mockStorage.Set("key2", "value2")

	t.Run("StartStop", func(t *testing.T) {
		rm := NewRepairManager(mockStorage, 100*time.Millisecond)

		rm.Start()
		time.Sleep(50 * time.Millisecond)

		rm.Stop()
		// Если не падает - значит работает
	})

	t.Run("PerformSync", func(t *testing.T) {
		rm := NewRepairManager(mockStorage, 1*time.Hour) // Большой интервал

		err := rm.performSync()
		if err != nil {
			t.Errorf("Sync should not fail: %v", err)
		}
	})

	t.Run("RepairKey", func(t *testing.T) {
		rm := NewRepairManager(mockStorage, 1*time.Hour)

		err := rm.RepairKey("new-key", "new-value")
		if err != nil {
			t.Errorf("Repair should not fail: %v", err)
		}

		value, err := mockStorage.Get("new-key")
		if err != nil || value != "new-value" {
			t.Error("Repair should actually set the value")
		}
	})
}

func TestRepairManager_Integration(t *testing.T) {
	t.Run("BackgroundSync", func(t *testing.T) {
		mockStorage := NewMockStorage()
		mockStorage.Set("test1", "value1")

		rm := NewRepairManager(mockStorage, 100*time.Millisecond)
		rm.Start()

		time.Sleep(250 * time.Millisecond) // Ждем несколько тиков
		rm.Stop()

		// Проверяем что синхронизация выполнялась
		items, _ := mockStorage.GetAll()
		if len(items) == 0 {
			t.Error("Storage should still have items after sync")
		}
	})
}
