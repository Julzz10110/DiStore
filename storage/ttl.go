package storage

import (
	"sync"
	"time"
)

type TTLStorage struct {
	Storage
	ttlData         map[string]time.Time
	mu              sync.RWMutex
	cleanupInterval time.Duration
}

func NewTTLStorage(base Storage, cleanupInterval time.Duration) *TTLStorage {
	ttlStorage := &TTLStorage{
		Storage:         base,
		ttlData:         make(map[string]time.Time),
		cleanupInterval: cleanupInterval,
	}

	go ttlStorage.startCleanupWorker()
	return ttlStorage
}

func (s *TTLStorage) SetWithTTL(key, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.Storage.Set(key, value); err != nil {
		return err
	}

	if ttl > 0 {
		s.ttlData[key] = time.Now().Add(ttl)
	} else {
		delete(s.ttlData, key) // remove TTL if exists
	}

	return nil
}

func (s *TTLStorage) Set(key, value string) error {
	return s.SetWithTTL(key, value, 0) // no TTL
}

func (s *TTLStorage) Get(key string) (string, error) {
	s.mu.RLock()
	expiry, hasTTL := s.ttlData[key]
	s.mu.RUnlock()

	if hasTTL && time.Now().After(expiry) {
		s.mu.Lock()
		delete(s.ttlData, key)
		s.Storage.Delete(key) // clean up expired key
		s.mu.Unlock()
		return "", ErrKeyNotFound
	}

	return s.Storage.Get(key)
}

func (s *TTLStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.ttlData, key)
	return s.Storage.Delete(key)
}

func (s *TTLStorage) startCleanupWorker() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupExpired()
	}
}

func (s *TTLStorage) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, expiry := range s.ttlData {
		if now.After(expiry) {
			delete(s.ttlData, key)
			s.Storage.Delete(key)
		}
	}
}

func (s *TTLStorage) GetTTL(key string) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	expiry, exists := s.ttlData[key]
	if !exists {
		return 0, ErrKeyNotFound
	}

	remaining := time.Until(expiry)
	if remaining <= 0 {
		return 0, ErrKeyNotFound
	}

	return remaining, nil
}
