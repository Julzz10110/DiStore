package api

import (
	"bytes"
	"distore/auth"
	"distore/config"
	"distore/replication"
	"distore/storage"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockReplicator implements replication.ReplicatorInterface
type MockReplicator struct {
	setCalls         []string
	deleteCalls      []string
	nodes            []string
	replicaCount     int
	shouldFailSet    bool
	shouldFailDelete bool
}

func NewMockReplicator() *MockReplicator {
	return &MockReplicator{
		setCalls:     make([]string, 0),
		deleteCalls:  make([]string, 0),
		nodes:        []string{"mock-node-1", "mock-node-2"},
		replicaCount: 2,
	}
}

func (m *MockReplicator) ReplicateSet(key, value string) error {
	if m.shouldFailSet {
		return replication.ErrReplicationFailed
	}
	m.setCalls = append(m.setCalls, key)
	return nil
}

func (m *MockReplicator) ReplicateDelete(key string) error {
	if m.shouldFailDelete {
		return replication.ErrReplicationFailed
	}
	m.deleteCalls = append(m.deleteCalls, key)
	return nil
}

func (m *MockReplicator) UpdateNodes(newNodes []string) {
	m.nodes = make([]string, len(newNodes))
	copy(m.nodes, newNodes)
}

func (m *MockReplicator) GetNodes() []string {
	return m.nodes
}

func (m *MockReplicator) GetReplicaCount() int {
	return m.replicaCount
}

func (m *MockReplicator) SetReplicaCount(count int) {
	m.replicaCount = count
}

// Ensure MockReplicator implements ReplicatorInterface
var _ replication.ReplicatorInterface = (*MockReplicator)(nil)

// MockStorage implements storage.Storage
type MockStorage struct {
	data             map[string]string
	shouldFailSet    bool
	shouldFailGet    bool
	shouldFailDelete bool
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		data: make(map[string]string),
	}
}

func (m *MockStorage) Set(key, value string) error {
	if m.shouldFailSet {
		return storage.ErrKeyExists
	}
	m.data[key] = value
	return nil
}

func (m *MockStorage) Get(key string) (string, error) {
	if m.shouldFailGet {
		return "", storage.ErrKeyNotFound
	}
	value, exists := m.data[key]
	if !exists {
		return "", storage.ErrKeyNotFound
	}
	return value, nil
}

func (m *MockStorage) Delete(key string) error {
	if m.shouldFailDelete {
		return storage.ErrKeyNotFound
	}
	delete(m.data, key)
	return nil
}

func (m *MockStorage) GetAll() ([]storage.KeyValue, error) {
	result := make([]storage.KeyValue, 0, len(m.data))
	for k, v := range m.data {
		result = append(result, storage.KeyValue{Key: k, Value: v})
	}
	return result, nil
}

func (m *MockStorage) Close() error {
	return nil
}

// Ensure MockStorage implements storage.Storage
var _ storage.Storage = (*MockStorage)(nil)

func TestHandlers(t *testing.T) {
	mockStorage := NewMockStorage()
	mockReplicator := NewMockReplicator()
	handlers := NewHandlers(mockStorage, mockReplicator, nil)

	t.Run("SetHandler", func(t *testing.T) {
		kv := storage.KeyValue{Key: "test-key", Value: "test-value"}
		body, _ := json.Marshal(kv)

		req := httptest.NewRequest("POST", "/set", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handlers.SetHandler(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rr.Code)
		}

		// Verify value was stored
		value, err := mockStorage.Get("test-key")
		if err != nil {
			t.Fatalf("Failed to get stored value: %v", err)
		}
		if value != "test-value" {
			t.Errorf("Expected 'test-value', got '%s'", value)
		}

		// Verify replication was called
		if len(mockReplicator.setCalls) != 1 {
			t.Errorf("Expected 1 replication call, got %d", len(mockReplicator.setCalls))
		}
		if mockReplicator.setCalls[0] != "test-key" {
			t.Errorf("Expected replication for key 'test-key', got '%s'", mockReplicator.setCalls[0])
		}
	})

	t.Run("GetHandler", func(t *testing.T) {
		// First set a value
		mockStorage.Set("get-test", "get-value")

		req := httptest.NewRequest("GET", "/get/get-test", nil)
		rr := httptest.NewRecorder()

		// Need to use mux to extract path parameters
		handler := http.HandlerFunc(handlers.GetHandler)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]string
		json.NewDecoder(rr.Body).Decode(&response)

		if response["value"] != "get-value" {
			t.Errorf("Expected 'get-value', got '%s'", response["value"])
		}
		if response["key"] != "get-test" {
			t.Errorf("Expected key 'get-test', got '%s'", response["key"])
		}
	})

	t.Run("GetHandler_NotFound", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/get/non-existent", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(handlers.GetHandler)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", rr.Code)
		}
	})

	t.Run("DeleteHandler", func(t *testing.T) {
		mockStorage.Set("delete-test", "delete-value")

		req := httptest.NewRequest("DELETE", "/delete/delete-test", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(handlers.DeleteHandler)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		// Verify value was deleted
		_, err := mockStorage.Get("delete-test")
		if err != storage.ErrKeyNotFound {
			t.Error("Expected key to be deleted")
		}

		// Verify replication was called
		if len(mockReplicator.deleteCalls) != 1 {
			t.Errorf("Expected 1 replication call, got %d", len(mockReplicator.deleteCalls))
		}
		if mockReplicator.deleteCalls[0] != "delete-test" {
			t.Errorf("Expected replication for key 'delete-test', got '%s'", mockReplicator.deleteCalls[0])
		}
	})

	t.Run("HealthHandler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()

		handlers.HealthHandler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	t.Run("SetHandler_StorageError", func(t *testing.T) {
		mockStorage.shouldFailSet = true
		defer func() { mockStorage.shouldFailSet = false }()

		kv := storage.KeyValue{Key: "error-key", Value: "error-value"}
		body, _ := json.Marshal(kv)

		req := httptest.NewRequest("POST", "/set", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handlers.SetHandler(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}
	})

	t.Run("DeleteHandler_NotFound", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/delete/non-existent", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(handlers.DeleteHandler)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", rr.Code)
		}
	})
}

func TestHandlersWithAuth(t *testing.T) {
	privateKey, publicKey := generateTestKeys()
	authService, _ := auth.NewAuthService(&config.AuthConfig{
		Enabled:       true,
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		TokenDuration: 3600,
	})

	mockStorage := NewMockStorage()
	mockReplicator := NewMockReplicator()
	handlers := NewHandlers(mockStorage, mockReplicator, authService)

	t.Run("SetHandler with tenant isolation", func(t *testing.T) {
		token, _ := authService.GenerateToken("user1", "tenant1", []string{"write"})

		kv := storage.KeyValue{Key: "tenant-key", Value: "tenant-value"}
		body, _ := json.Marshal(kv)

		req := httptest.NewRequest("POST", "/set", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		// Create auth middleware
		authMiddleware := auth.AuthMiddleware(authService)
		handler := authMiddleware(http.HandlerFunc(handlers.SetHandler))

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rr.Code)
		}

		// Verify value was stored with tenant prefix
		value, err := mockStorage.Get("tenant1:tenant-key")
		if err != nil {
			t.Fatalf("Failed to get stored value: %v", err)
		}
		if value != "tenant-value" {
			t.Errorf("Expected 'tenant-value', got '%s'", value)
		}

		// Verify replication was called with tenant prefix
		if len(mockReplicator.setCalls) != 1 {
			t.Errorf("Expected 1 replication call, got %d", len(mockReplicator.setCalls))
		}
		if mockReplicator.setCalls[0] != "tenant1:tenant-key" {
			t.Errorf("Expected replication for key 'tenant1:tenant-key', got '%s'", mockReplicator.setCalls[0])
		}
	})
}

// Helper function for generating test keys
func generateTestKeys() (string, string) {
	return "test-private-key", "test-public-key"
}

func TestMockReplicatorMethods(t *testing.T) {
	replicator := NewMockReplicator()

	t.Run("GetNodes", func(t *testing.T) {
		nodes := replicator.GetNodes()
		if len(nodes) != 2 {
			t.Errorf("Expected 2 nodes, got %d", len(nodes))
		}
	})

	t.Run("GetReplicaCount", func(t *testing.T) {
		count := replicator.GetReplicaCount()
		if count != 2 {
			t.Errorf("Expected replica count 2, got %d", count)
		}
	})

	t.Run("SetReplicaCount", func(t *testing.T) {
		replicator.SetReplicaCount(3)
		if replicator.GetReplicaCount() != 3 {
			t.Error("Failed to set replica count")
		}
	})

	t.Run("UpdateNodes", func(t *testing.T) {
		newNodes := []string{"node1", "node2", "node3"}
		replicator.UpdateNodes(newNodes)

		nodes := replicator.GetNodes()
		if len(nodes) != 3 {
			t.Errorf("Expected 3 nodes after update, got %d", len(nodes))
		}
	})
}

func TestReplicationErrors(t *testing.T) {
	mockStorage := NewMockStorage()
	mockReplicator := NewMockReplicator()
	handlers := NewHandlers(mockStorage, mockReplicator, nil)

	t.Run("SetHandler_ReplicationError", func(t *testing.T) {
		mockReplicator.shouldFailSet = true
		defer func() { mockReplicator.shouldFailSet = false }()

		kv := storage.KeyValue{Key: "repl-error-key", Value: "repl-error-value"}
		body, _ := json.Marshal(kv)

		req := httptest.NewRequest("POST", "/set", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handlers.SetHandler(rr, req)

		// Replication occurs asynchronously, so the primary request must be successful
		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201 despite replication error, got %d", rr.Code)
		}

		// But the value must be preserved
		value, err := mockStorage.Get("repl-error-key")
		if err != nil {
			t.Fatalf("Value should be stored despite replication error: %v", err)
		}
		if value != "repl-error-value" {
			t.Errorf("Expected 'repl-error-value', got '%s'", value)
		}
	})

	t.Run("DeleteHandler_ReplicationError", func(t *testing.T) {
		// Firstly set the value
		mockStorage.Set("delete-repl-error-key", "delete-repl-error-value")

		mockReplicator.shouldFailDelete = true
		defer func() { mockReplicator.shouldFailDelete = false }()

		req := httptest.NewRequest("DELETE", "/delete/delete-repl-error-key", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(handlers.DeleteHandler)
		handler.ServeHTTP(rr, req)

		// Replication occurs asynchronously, so the primary request must be successful
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200 despite replication error, got %d", rr.Code)
		}

		// But the value must be removed from storage
		_, err := mockStorage.Get("delete-repl-error-key")
		if err != storage.ErrKeyNotFound {
			t.Error("Value should be deleted from storage despite replication error")
		}
	})

	t.Run("SetHandler_JSONError", func(t *testing.T) {
		// Invalid JSON
		body := bytes.NewReader([]byte("{invalid json}"))

		req := httptest.NewRequest("POST", "/set", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handlers.SetHandler(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid JSON, got %d", rr.Code)
		}
	})

	t.Run("SetHandler_MissingKey", func(t *testing.T) {
		kv := storage.KeyValue{Key: "", Value: "value-without-key"} // empty key
		body, _ := json.Marshal(kv)

		req := httptest.NewRequest("POST", "/set", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handlers.SetHandler(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for missing key, got %d", rr.Code)
		}
	})

	t.Run("GetHandler_InvalidPath", func(t *testing.T) {
		// The path without a key
		req := httptest.NewRequest("GET", "/get/", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(handlers.GetHandler)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid path, got %d", rr.Code)
		}
	})

	t.Run("DeleteHandler_InvalidPath", func(t *testing.T) {
		// The path without a key
		req := httptest.NewRequest("DELETE", "/delete/", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(handlers.DeleteHandler)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid path, got %d", rr.Code)
		}
	})

	t.Run("SetHandler_StorageFailure", func(t *testing.T) {
		mockStorage.shouldFailSet = true
		defer func() { mockStorage.shouldFailSet = false }()

		kv := storage.KeyValue{Key: "storage-fail-key", Value: "storage-fail-value"}
		body, _ := json.Marshal(kv)

		req := httptest.NewRequest("POST", "/set", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handlers.SetHandler(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 for storage failure, got %d", rr.Code)
		}

		// Replication should not be called when a storage error occurs
		if len(mockReplicator.setCalls) > 0 {
			t.Error("Replication should not be called when storage fails")
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		// Test concurrent access
		concurrency := 10
		done := make(chan bool, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(index int) {
				key := fmt.Sprintf("concurrent-key-%d", index)
				value := fmt.Sprintf("concurrent-value-%d", index)

				kv := storage.KeyValue{Key: key, Value: value}
				body, _ := json.Marshal(kv)

				req := httptest.NewRequest("POST", "/set", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				rr := httptest.NewRecorder()

				handlers.SetHandler(rr, req)

				if rr.Code != http.StatusCreated {
					t.Errorf("Concurrent set failed for key %s: status %d", key, rr.Code)
				}

				// Check that the value has been saved
				storedValue, err := mockStorage.Get(key)
				if err != nil || storedValue != value {
					t.Errorf("Concurrent get failed for key %s", key)
				}

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < concurrency; i++ {
			<-done
		}

		// Check if all values ​​are saved
		items, err := mockStorage.GetAll()
		if err != nil {
			t.Fatalf("GetAll failed: %v", err)
		}

		// There must be at least concurrency items (plus possible previous tests)
		foundCount := 0
		for _, item := range items {
			if strings.HasPrefix(item.Key, "concurrent-key-") {
				foundCount++
			}
		}

		if foundCount < concurrency {
			t.Errorf("Expected at least %d concurrent items, found %d", concurrency, foundCount)
		}
	})
}
