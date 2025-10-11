package cluster

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"distore/config"
)

type CrossDCReplicator struct {
	mu            sync.RWMutex
	dataCenters   []config.DataCenterConfig
	edgeNodes     []config.EdgeNodeConfig
	latencyConfig config.LatencyConfig
	httpClient    *http.Client
	nodeLatencies map[string]time.Duration
}

type ReplicationTarget struct {
	Node     string
	Region   string
	Priority int
	Latency  time.Duration
	IsEdge   bool
}

func NewCrossDCReplicator(multiCloudConfig config.MultiCloudConfig) *CrossDCReplicator {
	return &CrossDCReplicator{
		dataCenters:   multiCloudConfig.DataCenters,
		edgeNodes:     multiCloudConfig.EdgeNodes,
		latencyConfig: multiCloudConfig.LatencyThresholds,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: &http.Transport{MaxIdleConnsPerHost: 10},
		},
		nodeLatencies: make(map[string]time.Duration),
	}
}

// SelectOptimalTargets selects replication targets based on latency and priority
func (cdc *CrossDCReplicator) SelectOptimalTargets(key string, replicaCount int) ([]ReplicationTarget, error) {
	cdc.mu.RLock()
	defer cdc.mu.RUnlock()

	var targets []ReplicationTarget

	// Add data center nodes
	for _, dc := range cdc.dataCenters {
		for _, node := range dc.Nodes {
			latency := cdc.nodeLatencies[node]
			if latency == 0 {
				latency = time.Duration(dc.LatencyMs) * time.Millisecond
			}

			targets = append(targets, ReplicationTarget{
				Node:     node,
				Region:   dc.Region,
				Priority: dc.Priority,
				Latency:  latency,
				IsEdge:   false,
			})
		}
	}

	// Add edge nodes for low-latency access
	for _, edge := range cdc.edgeNodes {
		latency := cdc.nodeLatencies[edge.Node]
		if latency == 0 {
			latency = time.Duration(edge.LatencyMs) * time.Millisecond
		}

		targets = append(targets, ReplicationTarget{
			Node:     edge.Node,
			Region:   edge.Location,
			Priority: 1, // Edge nodes have highest priority
			Latency:  latency,
			IsEdge:   true,
		})
	}

	// Sort by priority (ascending) and then by latency (ascending)
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Priority != targets[j].Priority {
			return targets[i].Priority < targets[j].Priority
		}
		return targets[i].Latency < targets[j].Latency
	})

	// Limit to requested replica count
	if len(targets) > replicaCount {
		targets = targets[:replicaCount]
	}

	return targets, nil
}

// ReplicateToTargets replicates data to selected targets with latency awareness
func (cdc *CrossDCReplicator) ReplicateToTargets(key, value string, targets []ReplicationTarget) error {
	var wg sync.WaitGroup
	results := make(chan error, len(targets))

	for _, target := range targets {
		wg.Add(1)
		go func(t ReplicationTarget) {
			defer wg.Done()

			// Use different timeouts based on latency
			timeout := cdc.calculateTimeout(t.Latency)
			err := cdc.replicateToNode(key, value, t.Node, timeout)
			results <- err
		}(target)
	}

	wg.Wait()
	close(results)

	// Check results
	successCount := 0
	for err := range results {
		if err == nil {
			successCount++
		}
	}

	// Require at least 50% success rate for cross-DC replication
	if successCount < len(targets)/2 {
		return fmt.Errorf("cross-DC replication failed: only %d/%d successful", successCount, len(targets))
	}

	return nil
}

func (cdc *CrossDCReplicator) calculateTimeout(latency time.Duration) time.Duration {
	baseTimeout := 2 * time.Second
	maxTimeout := 10 * time.Second

	// Increase timeout based on latency
	calculatedTimeout := baseTimeout + latency*2
	if calculatedTimeout > maxTimeout {
		return maxTimeout
	}
	return calculatedTimeout
}

func (cdc *CrossDCReplicator) replicateToNode(key, value, nodeURL string, timeout time.Duration) error {
	_ = key   // TODO: implement request body with key/value
	_ = value // TODO: implement request body with key/value
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := fmt.Sprintf("http://%s/internal/set", nodeURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	// TODO: add request body with key/value

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("replication to %s failed: %s", nodeURL, resp.Status)
	}

	return nil
}

// UpdateLatencyMetrics updates latency measurements for nodes
func (cdc *CrossDCReplicator) UpdateLatencyMetrics(node string, latency time.Duration) {
	cdc.mu.Lock()
	defer cdc.mu.Unlock()
	cdc.nodeLatencies[node] = latency
}

// GetLatencyMetrics returns current latency measurements
func (cdc *CrossDCReplicator) GetLatencyMetrics() map[string]time.Duration {
	cdc.mu.RLock()
	defer cdc.mu.RUnlock()

	metrics := make(map[string]time.Duration)
	for node, latency := range cdc.nodeLatencies {
		metrics[node] = latency
	}
	return metrics
}

// IsEdgeNode checks if a node is an edge node
func (cdc *CrossDCReplicator) IsEdgeNode(nodeURL string) bool {
	cdc.mu.RLock()
	defer cdc.mu.RUnlock()

	for _, edge := range cdc.edgeNodes {
		if edge.Node == nodeURL {
			return true
		}
	}
	return false
}

// GetDataCenterForNode returns the data center configuration for a node
func (cdc *CrossDCReplicator) GetDataCenterForNode(nodeURL string) *config.DataCenterConfig {
	cdc.mu.RLock()
	defer cdc.mu.RUnlock()

	for _, dc := range cdc.dataCenters {
		for _, node := range dc.Nodes {
			if node == nodeURL {
				return &dc
			}
		}
	}
	return nil
}
