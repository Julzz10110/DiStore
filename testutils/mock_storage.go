package testutils

import (
	"distore/storage"
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
