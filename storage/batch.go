package storage

import (
	"fmt"
	"sync"
	"time"
)

type BatchOperation struct {
	Type  string // "set", "delete", "get"
	Key   string
	Value string
	TTL   time.Duration
}

type BatchResult struct {
	Operation BatchOperation
	Error     error
	Value     string // For get operations
}

type BatchStorage struct {
	Storage
	mu sync.Mutex
}

func NewBatchStorage(base Storage) *BatchStorage {
	return &BatchStorage{Storage: base}
}

func (s *BatchStorage) ExecuteBatch(operations []BatchOperation) []BatchResult {
	results := make([]BatchResult, len(operations))

	s.mu.Lock()
	defer s.mu.Unlock()

	for i, op := range operations {
		result := BatchResult{Operation: op}

		switch op.Type {
		case "set":
			if ttlStorage, ok := s.Storage.(*TTLStorage); ok && op.TTL > 0 {
				result.Error = ttlStorage.SetWithTTL(op.Key, op.Value, op.TTL)
			} else {
				result.Error = s.Storage.Set(op.Key, op.Value)
			}

		case "delete":
			result.Error = s.Storage.Delete(op.Key)

		case "get":
			value, err := s.Storage.Get(op.Key)
			result.Value = value
			result.Error = err

		default:
			result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
		}

		results[i] = result
	}

	return results
}

func (s *BatchStorage) MultiGet(keys []string) (map[string]string, error) {
	results := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errors []error

	for _, key := range keys {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()

			value, err := s.Storage.Get(key)
			mu.Lock()
			defer mu.Unlock()

			if err != nil && err != ErrKeyNotFound {
				errors = append(errors, err)
			} else if err == nil {
				results[key] = value
			}
		}(key)
	}

	wg.Wait()

	if len(errors) > 0 {
		return results, fmt.Errorf("multiple errors occurred: %v", errors)
	}

	return results, nil
}

func (s *BatchStorage) MultiSet(items map[string]string) error {
	operations := make([]BatchOperation, 0, len(items))

	for key, value := range items {
		operations = append(operations, BatchOperation{
			Type:  "set",
			Key:   key,
			Value: value,
		})
	}

	results := s.ExecuteBatch(operations)

	for _, result := range results {
		if result.Error != nil {
			return result.Error
		}
	}

	return nil
}
