package storage

import (
	"sync"
)

type MemoryStorage struct {
	data map[string]string
	mu   sync.RWMutex
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data: make(map[string]string),
	}
}

func (s *MemoryStorage) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

func (s *MemoryStorage) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.data[key]
	if !exists {
		return "", ErrKeyNotFound
	}
	return value, nil
}

func (s *MemoryStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[key]; !exists {
		return ErrKeyNotFound
	}
	delete(s.data, key)
	return nil
}

func (s *MemoryStorage) GetAll() ([]KeyValue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]KeyValue, 0, len(s.data))
	for k, v := range s.data {
		result = append(result, KeyValue{Key: k, Value: v})
	}
	return result, nil
}

func (s *MemoryStorage) Close() error {
	return nil
}
