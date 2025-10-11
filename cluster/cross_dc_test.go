package cluster

import (
	"distore/config"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestCrossDCReplicator_New(t *testing.T) {
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

	cdc := NewCrossDCReplicator(cfg)

	if cdc == nil {
		t.Fatal("Expected non-nil CrossDCReplicator")
	}

	if len(cdc.dataCenters) != 2 {
		t.Errorf("Expected 2 datacenters, got %d", len(cdc.dataCenters))
	}

	if cdc.dataCenters[0].Nodes[0] != "node1:8080" {
		t.Errorf("Expected node1:8080, got %s", cdc.dataCenters[0].Nodes[0])
	}

	if len(cdc.edgeNodes) != 1 {
		t.Errorf("Expected 1 edge node, got %d", len(cdc.edgeNodes))
	}
}

func TestCrossDCReplicator_SelectOptimalTargets(t *testing.T) {
	cfg := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "local",
				Region:       "us-east-1",
				Nodes:        []string{"local1:8080"},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    10,
			},
			{
				ID:           "dc2",
				Region:       "us-west-1",
				Nodes:        []string{"remote1:8080", "remote2:8080"},
				Priority:     2,
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
		},
		LatencyThresholds: config.LatencyConfig{
			CrossDCThresholdMs: 100,
		},
	}

	cdc := NewCrossDCReplicator(cfg)

	// Test selecting targets
	targets, err := cdc.SelectOptimalTargets("test-key", 3)
	if err != nil {
		t.Errorf("Expected no error selecting targets, got %v", err)
	}

	if len(targets) != 3 {
		t.Errorf("Expected 3 target nodes, got %d", len(targets))
	}

	// Should prioritize by priority first (1 is highest), then by latency within same priority
	// Both edge and local DC have priority 1, but local DC has lower latency (10ms vs 20ms)
	expectedOrder := []string{"local1:8080", "edge-nyc:8080", "remote1:8080"}
	for i, target := range targets {
		if target.Node != expectedOrder[i] {
			t.Errorf("Expected %s at position %d, got %s", expectedOrder[i], i, target.Node)
		}
	}
}

func TestCrossDCReplicator_CalculateTimeout(t *testing.T) {
	cfg := config.MultiCloudConfig{
		LatencyThresholds: config.LatencyConfig{
			CrossDCThresholdMs: 100,
		},
	}

	cdc := NewCrossDCReplicator(cfg)

	// Test timeout calculation with different latencies
	testCases := []struct {
		latency     time.Duration
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{50 * time.Millisecond, 2 * time.Second, 4 * time.Second},
		{200 * time.Millisecond, 2 * time.Second, 10 * time.Second}, // Should cap at maxTimeout
		{0, 2 * time.Second, 3 * time.Second},
	}

	for _, tc := range testCases {
		timeout := cdc.calculateTimeout(tc.latency)
		if timeout < tc.expectedMin || timeout > tc.expectedMax {
			t.Errorf("Expected timeout between %v and %v for latency %v, got %v",
				tc.expectedMin, tc.expectedMax, tc.latency, timeout)
		}
	}
}

func TestCrossDCReplicator_ReplicateToTargets(t *testing.T) {
	// Create test server to mock remote nodes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/set" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "remote",
				Region:       "us-west-1",
				Nodes:        []string{server.URL[7:]}, // remove http:// prefix
				Priority:     2,
				ReplicaCount: 1,
				LatencyMs:    100,
			},
		},
	}

	cdc := NewCrossDCReplicator(cfg)

	// Test replication to targets
	targets := []ReplicationTarget{
		{
			Node:     server.URL[7:],
			Region:   "us-west-1",
			Priority: 2,
			Latency:  100 * time.Millisecond,
			IsEdge:   false,
		},
	}

	err := cdc.ReplicateToTargets("test-key", "test-value", targets)
	if err != nil {
		t.Errorf("Expected no error replicating to targets, got %v", err)
	}
}

func TestCrossDCReplicator_UpdateLatencyMetrics(t *testing.T) {
	cfg := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080"},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    50,
			},
		},
	}

	cdc := NewCrossDCReplicator(cfg)

	// Update latency metrics
	cdc.UpdateLatencyMetrics("node1:8080", 75*time.Millisecond)

	// Get latency metrics
	metrics := cdc.GetLatencyMetrics()
	if len(metrics) != 1 {
		t.Errorf("Expected 1 latency metric, got %d", len(metrics))
	}

	if metrics["node1:8080"] != 75*time.Millisecond {
		t.Errorf("Expected latency 75ms, got %v", metrics["node1:8080"])
	}
}

func TestCrossDCReplicator_IsEdgeNode(t *testing.T) {
	cfg := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"dc-node:8080"},
				Priority:     1,
				ReplicaCount: 1,
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
	}

	cdc := NewCrossDCReplicator(cfg)

	// Test edge node detection
	if !cdc.IsEdgeNode("edge-nyc:8080") {
		t.Error("Expected edge-nyc:8080 to be identified as edge node")
	}

	if cdc.IsEdgeNode("dc-node:8080") {
		t.Error("Expected dc-node:8080 to not be identified as edge node")
	}

	if cdc.IsEdgeNode("unknown:8080") {
		t.Error("Expected unknown:8080 to not be identified as edge node")
	}
}

func TestCrossDCReplicator_GetDataCenterForNode(t *testing.T) {
	cfg := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"dc1-node1:8080", "dc1-node2:8080"},
				Priority:     1,
				ReplicaCount: 2,
				LatencyMs:    50,
			},
			{
				ID:           "dc2",
				Region:       "us-west-1",
				Nodes:        []string{"dc2-node1:8080"},
				Priority:     2,
				ReplicaCount: 1,
				LatencyMs:    100,
			},
		},
	}

	cdc := NewCrossDCReplicator(cfg)

	// Test getting datacenter for node
	dc := cdc.GetDataCenterForNode("dc1-node1:8080")
	if dc == nil {
		t.Fatal("Expected non-nil datacenter config")
	}

	if dc.ID != "dc1" {
		t.Errorf("Expected datacenter ID 'dc1', got %s", dc.ID)
	}

	dc = cdc.GetDataCenterForNode("dc2-node1:8080")
	if dc == nil {
		t.Fatal("Expected non-nil datacenter config")
	}

	if dc.ID != "dc2" {
		t.Errorf("Expected datacenter ID 'dc2', got %s", dc.ID)
	}

	// Test with non-existent node
	dc = cdc.GetDataCenterForNode("unknown:8080")
	if dc != nil {
		t.Error("Expected nil datacenter config for unknown node")
	}
}

func TestCrossDCReplicator_ConcurrentAccess(t *testing.T) {
	cfg := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080"},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    50,
			},
		},
	}

	cdc := NewCrossDCReplicator(cfg)

	// Test concurrent access to latency metrics
	numGoroutines := 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(nodeID int) {
			defer wg.Done()
			node := fmt.Sprintf("node%d:8080", nodeID)

			// Concurrent writes to latency map
			cdc.UpdateLatencyMetrics(node, time.Duration(nodeID*10)*time.Millisecond)

			// Concurrent reads
			metrics := cdc.GetLatencyMetrics()
			if _, exists := metrics[node]; !exists {
				t.Errorf("Expected node %s to exist in latency metrics", node)
			}
		}(i)
	}

	wg.Wait()

	// Verify all nodes were added
	metrics := cdc.GetLatencyMetrics()
	if len(metrics) != numGoroutines {
		t.Errorf("Expected %d nodes in latency metrics, got %d", numGoroutines, len(metrics))
	}
}

func TestCrossDCReplicator_EdgeCaseScenarios(t *testing.T) {
	// Test with empty configuration
	emptyConfig := config.MultiCloudConfig{
		Enabled: false,
	}

	cdc := NewCrossDCReplicator(emptyConfig)

	// Test selecting targets with empty config
	targets, err := cdc.SelectOptimalTargets("test-key", 3)
	if err != nil {
		t.Errorf("Expected no error with empty config, got %v", err)
	}

	if len(targets) != 0 {
		t.Errorf("Expected 0 targets with empty config, got %d", len(targets))
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

	cdc = NewCrossDCReplicator(singleNodeConfig)
	targets, err = cdc.SelectOptimalTargets("test-key", 3)
	if err != nil {
		t.Errorf("Expected no error with single node config, got %v", err)
	}

	if len(targets) != 1 {
		t.Errorf("Expected 1 target with single node config, got %d", len(targets))
	}

	if targets[0].Node != "single-node:8080" {
		t.Errorf("Expected target node 'single-node:8080', got %s", targets[0].Node)
	}
}

// Benchmark tests
func BenchmarkCrossDCReplicator_SelectOptimalTargets(b *testing.B) {
	cfg := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080", "node2:8080", "node3:8080"},
				Priority:     1,
				ReplicaCount: 3,
				LatencyMs:    50,
			},
			{
				ID:           "dc2",
				Region:       "us-west-1",
				Nodes:        []string{"node4:8080", "node5:8080", "node6:8080"},
				Priority:     2,
				ReplicaCount: 3,
				LatencyMs:    100,
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
		},
	}

	cdc := NewCrossDCReplicator(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cdc.SelectOptimalTargets("test-key", 5)
	}
}

func BenchmarkCrossDCReplicator_UpdateLatencyMetrics(b *testing.B) {
	cfg := config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080"},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    50,
			},
		},
	}

	cdc := NewCrossDCReplicator(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cdc.UpdateLatencyMetrics("node1:8080", time.Duration(i%100)*time.Millisecond)
	}
}
