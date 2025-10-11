package k8s

import (
	"distore/config"
	"sync"
	"testing"
)

func TestEdgeNodeManager_New(t *testing.T) {
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
		LatencyThresholds: config.LatencyConfig{
			EdgeThresholdMs: 200,
		},
	}

	// Create edge node manager (without Kubernetes dependencies for testing)
	enm := &EdgeNodeManager{
		kubeClient: nil, // Will be nil in test environment
		config:     cfg,
	}

	if enm == nil {
		t.Fatal("Expected non-nil EdgeNodeManager")
	}

	if enm.config == nil {
		t.Fatal("Expected non-nil config")
	}
}

func TestEdgeNodeManager_GetEdgeNodes(t *testing.T) {
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
			{
				ID:        "edge-tokyo",
				Location:  "Tokyo",
				Node:      "edge-tokyo:8080",
				CacheOnly: true,
				LatencyMs: 80,
			},
		},
	}

	enm := &EdgeNodeManager{
		kubeClient: nil,
		config:     cfg,
	}

	// Test getting all edge nodes
	nodes := enm.GetEdgeNodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 edge nodes, got %d", len(nodes))
	}

	// Test getting nodes by location
	nycNodes := enm.GetEdgeNodesByLocation("New York")
	if len(nycNodes) != 1 {
		t.Errorf("Expected 1 NYC node, got %d", len(nycNodes))
	}

	if nycNodes[0].ID != "edge-nyc" {
		t.Errorf("Expected NYC node ID 'edge-nyc', got %s", nycNodes[0].ID)
	}

	// Test getting cache-only nodes
	cacheNodes := enm.GetCacheOnlyNodes()
	if len(cacheNodes) != 2 {
		t.Errorf("Expected 2 cache-only nodes, got %d", len(cacheNodes))
	}

	// Test getting full replica nodes
	fullNodes := enm.GetFullReplicaNodes()
	if len(fullNodes) != 1 {
		t.Errorf("Expected 1 full replica node, got %d", len(fullNodes))
	}

	if fullNodes[0].ID != "edge-london" {
		t.Errorf("Expected London node ID 'edge-london', got %s", fullNodes[0].ID)
	}
}

func TestEdgeNodeManager_GetOptimalEdgeNode(t *testing.T) {
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

	enm := &EdgeNodeManager{
		kubeClient: nil,
		config:     cfg,
	}

	// Test getting optimal node by location
	optimalNode := enm.GetOptimalEdgeNode("New York")
	if optimalNode == nil {
		t.Fatal("Expected non-nil optimal node")
	}

	if optimalNode.ID != "edge-nyc" {
		t.Errorf("Expected optimal node ID 'edge-nyc', got %s", optimalNode.ID)
	}

	// Test with location that has no edge nodes
	optimalNode = enm.GetOptimalEdgeNode("Unknown Location")
	if optimalNode != nil {
		t.Error("Expected nil optimal node for unknown location")
	}

	// Test getting optimal cache-only node
	cacheNode := enm.GetOptimalCacheOnlyNode()
	if cacheNode == nil {
		t.Fatal("Expected non-nil cache-only node")
	}

	// Should return the one with lowest latency
	if cacheNode.ID != "edge-nyc" {
		t.Errorf("Expected optimal cache node ID 'edge-nyc', got %s", cacheNode.ID)
	}
}

func TestEdgeNodeManager_EdgeNodeHealth(t *testing.T) {
	cfg := &config.MultiCloudConfig{
		Enabled: true,
		EdgeNodes: []config.EdgeNodeConfig{
			{
				ID:        "edge-test",
				Location:  "Test Location",
				Node:      "edge-test:8080",
				CacheOnly: true,
				LatencyMs: 30,
			},
		},
	}

	enm := &EdgeNodeManager{
		kubeClient: nil,
		config:     cfg,
	}

	// Test edge node health
	health := enm.GetEdgeNodeHealth("edge-test")
	if health == nil {
		t.Fatal("Expected non-nil health status")
	}

	if health.NodeID != "edge-test" {
		t.Errorf("Expected node ID 'edge-test', got %s", health.NodeID)
	}

	// Test getting all edge node health
	allHealth := enm.GetAllEdgeNodeHealth()
	if len(allHealth) != 1 {
		t.Errorf("Expected 1 health status, got %d", len(allHealth))
	}
}

func TestEdgeNodeManager_ConcurrentAccess(t *testing.T) {
	cfg := &config.MultiCloudConfig{
		Enabled: true,
		EdgeNodes: []config.EdgeNodeConfig{
			{
				ID:        "edge-1",
				Location:  "Location 1",
				Node:      "edge-1:8080",
				CacheOnly: true,
				LatencyMs: 20,
			},
			{
				ID:        "edge-2",
				Location:  "Location 2",
				Node:      "edge-2:8080",
				CacheOnly: false,
				LatencyMs: 40,
			},
			{
				ID:        "edge-3",
				Location:  "Location 3",
				Node:      "edge-3:8080",
				CacheOnly: true,
				LatencyMs: 60,
			},
		},
	}

	enm := &EdgeNodeManager{
		kubeClient: nil,
		config:     cfg,
	}

	// Test concurrent access to edge node operations
	numGoroutines := 5
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Concurrent reads
			nodes := enm.GetEdgeNodes()
			if len(nodes) != 3 {
				t.Errorf("Worker %d: Expected 3 edge nodes, got %d", workerID, len(nodes))
			}

			// Concurrent location-based queries
			cacheNodes := enm.GetCacheOnlyNodes()
			if len(cacheNodes) != 2 {
				t.Errorf("Worker %d: Expected 2 cache-only nodes, got %d", workerID, len(cacheNodes))
			}
		}(i)
	}

	wg.Wait()
}

func TestEdgeNodeManager_InvalidConfiguration(t *testing.T) {
	// Test with nil config
	enm := &EdgeNodeManager{
		kubeClient: nil,
		config:     nil,
	}

	nodes := enm.GetEdgeNodes()
	if len(nodes) != 0 {
		t.Errorf("Expected 0 edge nodes with nil config, got %d", len(nodes))
	}

	// Test with empty edge nodes
	cfg := &config.MultiCloudConfig{
		Enabled:   true,
		EdgeNodes: []config.EdgeNodeConfig{},
	}

	enm.config = cfg
	nodes = enm.GetEdgeNodes()
	if len(nodes) != 0 {
		t.Errorf("Expected 0 edge nodes with empty config, got %d", len(nodes))
	}
}

// Helper function to create a test EdgeNodeManager
func createTestEdgeNodeManager(t *testing.T, cfg *config.MultiCloudConfig) *EdgeNodeManager {
	return &EdgeNodeManager{
		kubeClient: nil,
		config:     cfg,
	}
}

// Test helper to verify edge node configuration
func verifyEdgeNodeConfiguration(t *testing.T, node config.EdgeNodeConfig, expectedID, expectedLocation string, cacheOnly bool) {
	if node.ID != expectedID {
		t.Errorf("Expected edge node ID '%s', got %s", expectedID, node.ID)
	}

	if node.Location != expectedLocation {
		t.Errorf("Expected location '%s', got %s", expectedLocation, node.Location)
	}

	if node.CacheOnly != cacheOnly {
		t.Errorf("Expected cacheOnly=%t, got %t", cacheOnly, node.CacheOnly)
	}
}
