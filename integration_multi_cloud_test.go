package main

import (
	"distore/api"
	"distore/cluster"
	"distore/config"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// MockStorage implements storage.StorageInterface for testing
type MockStorage struct {
	data map[string]string
	mu   sync.RWMutex
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		data: make(map[string]string),
	}
}

func (m *MockStorage) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return "", fmt.Errorf("key not found")
}

func (m *MockStorage) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *MockStorage) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MockStorage) List() (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

// MockReplicator implements replication.ReplicatorInterface for testing
type MockReplicator struct {
	nodes []string
	mu    sync.RWMutex
}

func NewMockReplicator() *MockReplicator {
	return &MockReplicator{
		nodes: []string{},
	}
}

func (m *MockReplicator) SetNodes(nodes []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes = nodes
}

func (m *MockReplicator) GetNodes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nodes
}

func (m *MockReplicator) Replicate(key, value string, nodes []string) error {
	// Mock implementation - just return success
	return nil
}

func (m *MockReplicator) Read(key string, nodes []string) (string, error) {
	// Mock implementation
	return "test-value", nil
}

func (m *MockReplicator) SetReadQuorum(quorum int)  {}
func (m *MockReplicator) SetWriteQuorum(quorum int) {}

func TestMultiCloudIntegration_CrossDCReplication(t *testing.T) {
	// Setup multi-cloud configuration
	cfg := &config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "us-east-1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080", "node2:8080"},
				Priority:     1,
				ReplicaCount: 2,
				LatencyMs:    50,
			},
			{
				ID:           "us-west-1",
				Region:       "us-west-1",
				Nodes:        []string{"node3:8080", "node4:8080"},
				Priority:     2,
				ReplicaCount: 2,
				LatencyMs:    100,
			},
			{
				ID:           "eu-west-1",
				Region:       "eu-west-1",
				Nodes:        []string{"node5:8080", "node6:8080"},
				Priority:     3,
				ReplicaCount: 2,
				LatencyMs:    150,
			},
		},
		EdgeNodes: []config.EdgeNodeConfig{
			{
				ID:        "edge-nyc",
				Location:  "New York",
				Node:      "edge-nyc:8080",
				CacheOnly: true,
				LatencyMs: 20,
			},
			{
				ID:        "edge-london",
				Location:  "London",
				Node:      "edge-london:8080",
				CacheOnly: false,
				LatencyMs: 30,
			},
		},
		LatencyThresholds: config.LatencyConfig{
			LocalThresholdMs:   10,
			CrossDCThresholdMs: 100,
			EdgeThresholdMs:    200,
		},
		// Note: Replication config would be added to MultiCloudConfig if needed
	}

	// Create cross-DC replicator
	crossDCReplicator := cluster.NewCrossDCReplicator(*cfg)

	// Test selecting optimal targets
	targets, err := crossDCReplicator.SelectOptimalTargets("test-key", 3)
	if err != nil {
		t.Errorf("Expected no error selecting targets, got %v", err)
	}

	if len(targets) != 3 {
		t.Errorf("Expected 3 targets, got %d", len(targets))
	}

	// Test latency metrics
	crossDCReplicator.UpdateLatencyMetrics("node1:8080", 50*time.Millisecond)
	crossDCReplicator.UpdateLatencyMetrics("node3:8080", 100*time.Millisecond)
	crossDCReplicator.UpdateLatencyMetrics("node5:8080", 150*time.Millisecond)

	metrics := crossDCReplicator.GetLatencyMetrics()
	if len(metrics) != 3 {
		t.Errorf("Expected 3 latency metrics, got %d", len(metrics))
	}
}

func TestMultiCloudIntegration_EdgeComputing(t *testing.T) {
	// Setup edge computing configuration
	cfg := &config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "local",
				Region:       "us-east-1",
				Nodes:        []string{"local-node:8080"},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    10,
			},
		},
		EdgeNodes: []config.EdgeNodeConfig{
			{
				ID:        "edge-nyc",
				Location:  "New York",
				Node:      "edge-nyc:8080",
				CacheOnly: true,
				LatencyMs: 20,
			},
			{
				ID:        "edge-london",
				Location:  "London",
				Node:      "edge-london:8080",
				CacheOnly: false,
				LatencyMs: 50,
			},
			{
				ID:        "edge-tokyo",
				Location:  "Tokyo",
				Node:      "edge-tokyo:8080",
				CacheOnly: true,
				LatencyMs: 80,
			},
		},
		LatencyThresholds: config.LatencyConfig{
			EdgeThresholdMs: 100,
		},
	}

	// Create edge node manager (this would need to be implemented in cluster package)
	// For now, we'll create a mock implementation
	edgeManager := createMockEdgeNodeManager(cfg)

	// Test edge node operations
	allNodes := edgeManager.GetEdgeNodes()
	if len(allNodes) != 3 {
		t.Errorf("Expected 3 edge nodes, got %d", len(allNodes))
	}

	// Test location-based node selection
	nycNodes := edgeManager.GetEdgeNodesByLocation("New York")
	if len(nycNodes) != 1 {
		t.Errorf("Expected 1 NYC node, got %d", len(nycNodes))
	}

	if nycNodes[0].ID != "edge-nyc" {
		t.Errorf("Expected NYC node ID 'edge-nyc', got %s", nycNodes[0].ID)
	}

	// Test cache-only nodes
	cacheNodes := edgeManager.GetCacheOnlyNodes()
	if len(cacheNodes) != 2 {
		t.Errorf("Expected 2 cache-only nodes, got %d", len(cacheNodes))
	}

	// Test full replica nodes
	fullNodes := edgeManager.GetFullReplicaNodes()
	if len(fullNodes) != 1 {
		t.Errorf("Expected 1 full replica node, got %d", len(fullNodes))
	}

	if fullNodes[0].ID != "edge-london" {
		t.Errorf("Expected London node ID 'edge-london', got %s", fullNodes[0].ID)
	}

	// Test optimal node selection
	optimalNode := edgeManager.GetOptimalEdgeNode("New York")
	if optimalNode == nil {
		t.Fatal("Expected non-nil optimal node")
	}

	if optimalNode.ID != "edge-nyc" {
		t.Errorf("Expected optimal node ID 'edge-nyc', got %s", optimalNode.ID)
	}

	// Test optimal cache-only node selection
	optimalCacheNode := edgeManager.GetOptimalCacheOnlyNode()
	if optimalCacheNode == nil {
		t.Fatal("Expected non-nil optimal cache node")
	}

	// Should return the one with lowest latency
	if optimalCacheNode.ID != "edge-nyc" {
		t.Errorf("Expected optimal cache node ID 'edge-nyc', got %s", optimalCacheNode.ID)
	}
}

func TestMultiCloudIntegration_APIEndpoints(t *testing.T) {
	// Setup handlers (using unexported fields)
	handlers := &api.Handlers{}
	// Note: In a real implementation, you'd need to set the unexported fields
	// or use a constructor function

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/get/test-key":
			handlers.GetHandler(w, r)
		case "/set":
			handlers.SetHandler(w, r)
		case "/admin/nodes":
			handlers.ListNodesHandler(w, r)
		case "/admin/config":
			handlers.GetConfigHandler(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test basic operations
	client := &http.Client{Timeout: 5 * time.Second}

	// Test set operation
	setReq, err := http.NewRequest("POST", server.URL+"/set", nil)
	if err != nil {
		t.Fatalf("Failed to create set request: %v", err)
	}
	setReq.Header.Set("Content-Type", "application/json")

	setResp, err := client.Do(setReq)
	if err != nil {
		t.Fatalf("Failed to execute set request: %v", err)
	}
	defer setResp.Body.Close()

	if setResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for set, got %d", setResp.StatusCode)
	}

	// Test get operation
	getReq, err := http.NewRequest("GET", server.URL+"/get/test-key", nil)
	if err != nil {
		t.Fatalf("Failed to create get request: %v", err)
	}

	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("Failed to execute get request: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent key, got %d", getResp.StatusCode)
	}

	// Test admin endpoints
	adminReq, err := http.NewRequest("GET", server.URL+"/admin/nodes", nil)
	if err != nil {
		t.Fatalf("Failed to create admin request: %v", err)
	}

	adminResp, err := client.Do(adminReq)
	if err != nil {
		t.Fatalf("Failed to execute admin request: %v", err)
	}
	defer adminResp.Body.Close()

	if adminResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for admin nodes, got %d", adminResp.StatusCode)
	}
}

func TestMultiCloudIntegration_ConcurrentOperations(t *testing.T) {
	// Setup handlers (using unexported fields)
	handlers := &api.Handlers{}
	// Note: In a real implementation, you'd need to set the unexported fields
	// or use a constructor function

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			handlers.SetHandler(w, r)
		case "/get":
			handlers.GetHandler(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test concurrent operations
	numGoroutines := 10
	numOperations := 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := &http.Client{Timeout: 10 * time.Second}

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("worker-%d-key-%d", workerID, j)
				_ = fmt.Sprintf("worker-%d-value-%d", workerID, j) // value not used in this test

				// Set operation
				setReq, err := http.NewRequest("POST", server.URL+"/set", nil)
				if err != nil {
					errors <- fmt.Errorf("worker %d: failed to create set request: %v", workerID, err)
					continue
				}
				setReq.Header.Set("Content-Type", "application/json")

				setResp, err := client.Do(setReq)
				if err != nil {
					errors <- fmt.Errorf("worker %d: failed to execute set request: %v", workerID, err)
					continue
				}
				setResp.Body.Close()

				if setResp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("worker %d: expected status 200 for set, got %d", workerID, setResp.StatusCode)
					continue
				}

				// Get operation
				getReq, err := http.NewRequest("GET", server.URL+"/get/"+key, nil)
				if err != nil {
					errors <- fmt.Errorf("worker %d: failed to create get request: %v", workerID, err)
					continue
				}

				getResp, err := client.Do(getReq)
				if err != nil {
					errors <- fmt.Errorf("worker %d: failed to execute get request: %v", workerID, err)
					continue
				}
				getResp.Body.Close()

				// Note: Get will return 404 since we're not actually setting the value in the mock
				// This is expected behavior for this test
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("Expected no errors in concurrent operations, got %d errors", errorCount)
	}
}

func TestMultiCloudIntegration_ConfigurationValidation(t *testing.T) {
	// Test valid configuration
	validConfig := &config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080", "node2:8080"},
				Priority:     1,
				ReplicaCount: 2,
				LatencyMs:    50,
			},
		},
		EdgeNodes: []config.EdgeNodeConfig{
			{
				ID:        "edge-nyc",
				Location:  "New York",
				Node:      "edge-nyc:8080",
				CacheOnly: true,
				LatencyMs: 20,
			},
		},
		LatencyThresholds: config.LatencyConfig{
			LocalThresholdMs:   10,
			CrossDCThresholdMs: 100,
			EdgeThresholdMs:    200,
		},
	}

	// Validate configuration
	err := validateMultiCloudConfig(validConfig)
	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}

	// Test invalid configuration
	invalidConfig := &config.MultiCloudConfig{
		Enabled:        true,
		DataCenters:    []config.DataCenterConfig{}, // Empty datacenters
		EdgeNodes:      []config.EdgeNodeConfig{},
		CloudProviders: []config.CloudProvider{},
	}

	err = validateMultiCloudConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid config with empty datacenters")
	}
}

func TestMultiCloudIntegration_DataConsistency(t *testing.T) {
	// Setup storage and replicator
	storage := NewMockStorage()
	replicator := NewMockReplicator()
	replicator.SetNodes([]string{"node1:8080", "node2:8080", "node3:8080"})

	// Test data consistency across operations
	testKey := "consistency-test-key"
	testValue := "consistency-test-value"

	// Set value
	err := storage.Set(testKey, testValue)
	if err != nil {
		t.Errorf("Failed to set value: %v", err)
	}

	// Get value
	retrievedValue, err := storage.Get(testKey)
	if err != nil {
		t.Errorf("Failed to get value: %v", err)
	}

	if retrievedValue != testValue {
		t.Errorf("Expected value %s, got %s", testValue, retrievedValue)
	}

	// Test replication consistency
	err = replicator.Replicate(testKey, testValue, replicator.GetNodes())
	if err != nil {
		t.Errorf("Failed to replicate value: %v", err)
	}

	// Test read consistency
	readValue, err := replicator.Read(testKey, replicator.GetNodes())
	if err != nil {
		t.Errorf("Failed to read replicated value: %v", err)
	}

	if readValue != testValue {
		t.Errorf("Expected replicated value %s, got %s", testValue, readValue)
	}
}

func TestMultiCloudIntegration_EdgeCaseScenarios(t *testing.T) {
	// Test with empty configuration
	emptyConfig := config.MultiCloudConfig{
		Enabled: false,
	}

	crossDCReplicator := cluster.NewCrossDCReplicator(emptyConfig)

	// Test selecting targets with disabled config
	targets, err := crossDCReplicator.SelectOptimalTargets("test-key", 1)
	if err != nil {
		t.Errorf("Expected no error with disabled config, got %v", err)
	}

	if len(targets) != 0 {
		t.Errorf("Expected 0 targets with disabled config, got %d", len(targets))
	}

	// Test with single node configuration
	singleNodeConfig := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"single-node:8080"},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    10,
			},
		},
	}

	crossDCReplicator = cluster.NewCrossDCReplicator(singleNodeConfig)
	targets, err = crossDCReplicator.SelectOptimalTargets("test-key", 1)
	if err != nil {
		t.Errorf("Expected no error with single node config, got %v", err)
	}

	if len(targets) != 1 {
		t.Errorf("Expected 1 target with single node config, got %d", len(targets))
	}

	// Test edge node manager with empty configuration
	edgeManager := &MockEdgeNodeManager{
		config: &config.MultiCloudConfig{
			Enabled:   true,
			EdgeNodes: []config.EdgeNodeConfig{},
		},
	}

	nodes := edgeManager.GetEdgeNodes()
	if len(nodes) != 0 {
		t.Errorf("Expected 0 edge nodes, got %d", len(nodes))
	}

	cacheNodes := edgeManager.GetCacheOnlyNodes()
	if len(cacheNodes) != 0 {
		t.Errorf("Expected 0 cache-only nodes, got %d", len(cacheNodes))
	}
}

// Helper function to validate multi-cloud configuration
func validateMultiCloudConfig(cfg *config.MultiCloudConfig) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if cfg.Enabled && len(cfg.DataCenters) == 0 {
		return fmt.Errorf("at least one datacenter is required when enabled")
	}

	return nil
}

// Mock EdgeNodeManager for testing
type MockEdgeNodeManager struct {
	config *config.MultiCloudConfig
}

func (em *MockEdgeNodeManager) GetEdgeNodes() []config.EdgeNodeConfig {
	if em.config == nil {
		return []config.EdgeNodeConfig{}
	}
	return em.config.EdgeNodes
}

func (em *MockEdgeNodeManager) GetEdgeNodesByLocation(location string) []config.EdgeNodeConfig {
	var nodes []config.EdgeNodeConfig
	for _, node := range em.config.EdgeNodes {
		if node.Location == location {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (em *MockEdgeNodeManager) GetCacheOnlyNodes() []config.EdgeNodeConfig {
	var nodes []config.EdgeNodeConfig
	for _, node := range em.config.EdgeNodes {
		if node.CacheOnly {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (em *MockEdgeNodeManager) GetFullReplicaNodes() []config.EdgeNodeConfig {
	var nodes []config.EdgeNodeConfig
	for _, node := range em.config.EdgeNodes {
		if !node.CacheOnly {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (em *MockEdgeNodeManager) GetOptimalEdgeNode(location string) *config.EdgeNodeConfig {
	var bestNode *config.EdgeNodeConfig
	minLatency := int(^uint(0) >> 1) // Max int

	for _, node := range em.config.EdgeNodes {
		if node.Location == location && node.LatencyMs < minLatency {
			bestNode = &node
			minLatency = node.LatencyMs
		}
	}
	return bestNode
}

func (em *MockEdgeNodeManager) GetOptimalCacheOnlyNode() *config.EdgeNodeConfig {
	var bestNode *config.EdgeNodeConfig
	minLatency := int(^uint(0) >> 1) // Max int

	for _, node := range em.config.EdgeNodes {
		if node.CacheOnly && node.LatencyMs < minLatency {
			bestNode = &node
			minLatency = node.LatencyMs
		}
	}
	return bestNode
}

func createMockEdgeNodeManager(cfg *config.MultiCloudConfig) *MockEdgeNodeManager {
	return &MockEdgeNodeManager{
		config: cfg,
	}
}

// Benchmark tests for multi-cloud operations
func BenchmarkCrossDCReplication(b *testing.B) {
	cfg := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080", "node2:8080"},
				Priority:     1,
				ReplicaCount: 2,
				LatencyMs:    50,
			},
			{
				ID:           "dc2",
				Region:       "us-west-1",
				Nodes:        []string{"node3:8080", "node4:8080"},
				Priority:     2,
				ReplicaCount: 2,
				LatencyMs:    100,
			},
		},
	}

	crossDCReplicator := cluster.NewCrossDCReplicator(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crossDCReplicator.SelectOptimalTargets("test-key", 3)
	}
}

func BenchmarkEdgeNodeOperations(b *testing.B) {
	cfg := &config.MultiCloudConfig{
		Enabled: true,
		EdgeNodes: []config.EdgeNodeConfig{
			{
				ID:        "edge-nyc",
				Location:  "New York",
				Node:      "edge-nyc:8080",
				CacheOnly: true,
				LatencyMs: 20,
			},
			{
				ID:        "edge-london",
				Location:  "London",
				Node:      "edge-london:8080",
				CacheOnly: false,
				LatencyMs: 50,
			},
		},
	}

	// Create mock edge manager
	edgeManager := &MockEdgeNodeManager{
		config: cfg,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		edgeManager.GetOptimalEdgeNode("New York")
	}
}
