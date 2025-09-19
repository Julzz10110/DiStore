package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type DiskStorage struct {
	data     map[string]string
	dataDir  string
	dataFile string
	mu       sync.RWMutex
	saveMu   sync.Mutex // Separate mutex for saving
}

func NewDiskStorage(dataDir string) (*DiskStorage, error) {
	storage := &DiskStorage{
		data:     make(map[string]string),
		dataDir:  dataDir,
		dataFile: filepath.Join(dataDir, "data.json"),
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	if err := storage.loadFromDisk(); err != nil {
		return nil, err
	}

	return storage, nil
}

func (s *DiskStorage) loadFromDisk() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.dataFile); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(s.dataFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.data)
}

func (s *DiskStorage) saveToDisk() error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	s.mu.RLock()
	data, err := json.Marshal(s.data)
	s.mu.RUnlock()

	if err != nil {
		return err
	}

	tmpFile := s.dataFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, s.dataFile)
}

func (s *DiskStorage) Set(key, value string) error {
	s.mu.Lock()
	s.data[key] = value
	s.mu.Unlock()

	return s.saveToDisk()
}

func (s *DiskStorage) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.data[key]
	if !exists {
		return "", ErrKeyNotFound
	}
	return value, nil
}

func (s *DiskStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[key]; !exists {
		return ErrKeyNotFound
	}
	delete(s.data, key)

	// Asynchronous saving to avoid blocking the response
	go func() {
		if err := s.saveToDisk(); err != nil {
			// Log the error, but do not return it to the user
			println("Error saving to disk:", err.Error())
		}
	}()

	return nil
}

func (s *DiskStorage) GetAll() ([]KeyValue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]KeyValue, 0, len(s.data))
	for k, v := range s.data {
		result = append(result, KeyValue{Key: k, Value: v})
	}
	return result, nil
}

func (s *DiskStorage) Close() error {
	return s.saveToDisk()
}
