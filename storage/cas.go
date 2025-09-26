package storage

import (
	"fmt"
	"sync"
	"time"
)

type CASStorage struct {
	Storage
	versionMap map[string]int64
	mu         sync.RWMutex
}

func NewCASStorage(base Storage) *CASStorage {
	return &CASStorage{
		Storage:    base,
		versionMap: make(map[string]int64),
	}
}

type CASResult struct {
	Success      bool
	Version      int64
	CurrentValue string
}

func (s *CASStorage) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.versionMap[key] = time.Now().UnixNano()
	return s.Storage.Set(key, value)
}

func (s *CASStorage) Get(key string) (string, error) {
	return s.Storage.Get(key)
}

func (s *CASStorage) GetWithVersion(key string) (string, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, err := s.Storage.Get(key)
	if err != nil {
		return "", 0, err
	}

	version := s.versionMap[key]
	return value, version, nil
}

func (s *CASStorage) CompareAndSet(key string, expectedValue string, newValue string, expectedVersion int64) (*CASResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentValue, err := s.Storage.Get(key)
	if err != nil && err != ErrKeyNotFound {
		return nil, err
	}

	currentVersion := s.versionMap[key]

	// Check if key doesn't exist but we expect it to exist
	if err == ErrKeyNotFound {
		if expectedValue != "" {
			return &CASResult{
				Success: false,
				Version: currentVersion,
			}, nil
		}
		// Proceed with creation
	} else {
		// Key exists, check conditions
		if currentValue != expectedValue {
			return &CASResult{
				Success:      false,
				Version:      currentVersion,
				CurrentValue: currentValue,
			}, nil
		}

		if expectedVersion != 0 && currentVersion != expectedVersion {
			return &CASResult{
				Success:      false,
				Version:      currentVersion,
				CurrentValue: currentValue,
			}, nil
		}
	}

	// Update version and set new value
	newVersion := time.Now().UnixNano()
	s.versionMap[key] = newVersion

	if err := s.Storage.Set(key, newValue); err != nil {
		return nil, err
	}

	return &CASResult{
		Success: true,
		Version: newVersion,
	}, nil
}

func (s *CASStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.versionMap, key)
	return s.Storage.Delete(key)
}

// Lock with timeout
func (s *CASStorage) AcquireLock(lockKey string, timeout time.Duration) (bool, error) {
	expiry := time.Now().Add(timeout).UnixNano()
	lockValue := fmt.Sprintf("locked:%d", expiry)

	result, err := s.CompareAndSet(lockKey, "", lockValue, 0)
	if err != nil {
		return false, err
	}

	if result.Success {
		return true, nil
	}

	// Check if existing lock has expired
	currentValue, err := s.Storage.Get(lockKey)
	if err != nil {
		return false, err
	}

	// Try to parse existing lock
	var existingExpiry int64
	fmt.Sscanf(currentValue, "locked:%d", &existingExpiry)

	if time.Now().UnixNano() > existingExpiry {
		// Lock has expired, try to acquire it
		result, err = s.CompareAndSet(lockKey, currentValue, lockValue, 0)
		if err != nil {
			return false, err
		}
		return result.Success, nil
	}

	return false, nil // Lock is held by someone else
}

func (s *CASStorage) ReleaseLock(lockKey string) error {
	_, err := s.CompareAndSet(lockKey, "", "", 0)
	return err
}
