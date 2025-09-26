package storage

import (
	"fmt"
	"strconv"
	"sync"
)

type AtomicStorage struct {
	Storage
	mu sync.Mutex
}

func NewAtomicStorage(base Storage) *AtomicStorage {
	return &AtomicStorage{Storage: base}
}

func (s *AtomicStorage) Increment(key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.Storage.Get(key)
	if err != nil && err != ErrKeyNotFound {
		return 0, err
	}

	var currentValue int64
	if err == ErrKeyNotFound {
		currentValue = 0
	} else {
		currentValue, err = strconv.ParseInt(current, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("value is not an integer: %w", err)
		}
	}

	newValue := currentValue + delta
	newValueStr := strconv.FormatInt(newValue, 10)

	if err := s.Storage.Set(key, newValueStr); err != nil {
		return 0, err
	}

	return newValue, nil
}

func (s *AtomicStorage) Decrement(key string, delta int64) (int64, error) {
	return s.Increment(key, -delta)
}

func (s *AtomicStorage) CompareAndSwap(key string, oldValue, newValue string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.Storage.Get(key)
	if err != nil && err != ErrKeyNotFound {
		return false, err
	}

	// Handle case where key doesn't exist but oldValue is empty string
	if err == ErrKeyNotFound {
		if oldValue == "" {
			// Key doesn't exist and we expect it not to exist - proceed
			if setErr := s.Storage.Set(key, newValue); setErr != nil {
				return false, setErr
			}
			return true, nil
		}
		return false, nil // Expected value doesn't match
	}

	// Key exists, compare values
	if current != oldValue {
		return false, nil
	}

	if err := s.Storage.Set(key, newValue); err != nil {
		return false, err
	}

	return true, nil
}

func (s *AtomicStorage) GetInt64(key string) (int64, error) {
	value, err := s.Storage.Get(key)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(value, 10, 64)
}

func (s *AtomicStorage) GetFloat64(key string) (float64, error) {
	value, err := s.Storage.Get(key)
	if err != nil {
		return 0, err
	}

	return strconv.ParseFloat(value, 64)
}
