package cluster

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type NodeStatus struct {
	URL      string
	LastSeen time.Time
	Online   bool
	Latency  time.Duration
}

type FailoverManager struct {
	mu            sync.RWMutex
	nodes         []string
	nodeStatus    map[string]*NodeStatus
	checkInterval time.Duration
	timeout       time.Duration
}

func NewFailoverManager(nodes []string, checkInterval, timeout time.Duration) *FailoverManager {
	fm := &FailoverManager{
		nodes:         nodes,
		nodeStatus:    make(map[string]*NodeStatus),
		checkInterval: checkInterval,
		timeout:       timeout,
	}

	for _, node := range nodes {
		fm.nodeStatus[node] = &NodeStatus{
			URL:    node,
			Online: true,
		}
	}

	go fm.startHealthChecks()
	return fm
}

func (fm *FailoverManager) startHealthChecks() {
	ticker := time.NewTicker(fm.checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		fm.checkAllNodes()
	}
}

func (fm *FailoverManager) checkAllNodes() {
	var wg sync.WaitGroup

	// Take a snapshot of current nodes under read lock to avoid races
	fm.mu.RLock()
	nodesSnapshot := make([]string, len(fm.nodes))
	copy(nodesSnapshot, fm.nodes)
	fm.mu.RUnlock()

	for _, node := range nodesSnapshot {
		wg.Add(1)
		go func(nodeURL string) {
			defer wg.Done()
			fm.checkNode(nodeURL)
		}(node)
	}

	wg.Wait()
}

func (fm *FailoverManager) checkNode(nodeURL string) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), fm.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/health", nodeURL), nil)
	if err != nil {
		fm.updateNodeStatus(nodeURL, false, 0)
		return
	}

	client := &http.Client{Timeout: fm.timeout}
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil || resp.StatusCode != 200 {
		fm.updateNodeStatus(nodeURL, false, latency)
		return
	}

	fm.updateNodeStatus(nodeURL, true, latency)
}

func (fm *FailoverManager) updateNodeStatus(nodeURL string, online bool, latency time.Duration) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	status := fm.nodeStatus[nodeURL]
	status.Online = online
	status.LastSeen = time.Now()
	status.Latency = latency
}

func (fm *FailoverManager) GetActiveNodes() []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	activeNodes := make([]string, 0)
	for _, status := range fm.nodeStatus {
		if status.Online {
			activeNodes = append(activeNodes, status.URL)
		}
	}
	return activeNodes
}

func (fm *FailoverManager) GetNodeStatus() map[string]*NodeStatus {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	statusCopy := make(map[string]*NodeStatus)
	for k, v := range fm.nodeStatus {
		statusCopy[k] = &NodeStatus{
			URL:      v.URL,
			LastSeen: v.LastSeen,
			Online:   v.Online,
			Latency:  v.Latency,
		}
	}
	return statusCopy
}

// SetNodes replaces the current node list and initializes statuses for new nodes
func (fm *FailoverManager) SetNodes(nodes []string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.nodes = make([]string, len(nodes))
	copy(fm.nodes, nodes)

	// Ensure status entries exist for all nodes; remove stale ones
	newStatus := make(map[string]*NodeStatus, len(nodes))
	for _, n := range nodes {
		if s, ok := fm.nodeStatus[n]; ok {
			newStatus[n] = s
		} else {
			newStatus[n] = &NodeStatus{URL: n, Online: true}
		}
	}
	fm.nodeStatus = newStatus
}

// AddNode appends a node if missing
func (fm *FailoverManager) AddNode(node string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	for _, n := range fm.nodes {
		if n == node {
			return
		}
	}
	fm.nodes = append(fm.nodes, node)
	if _, exists := fm.nodeStatus[node]; !exists {
		fm.nodeStatus[node] = &NodeStatus{URL: node, Online: true}
	}
}

// RemoveNode removes a node if present
func (fm *FailoverManager) RemoveNode(node string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	filtered := make([]string, 0, len(fm.nodes))
	for _, n := range fm.nodes {
		if n != node {
			filtered = append(filtered, n)
		}
	}
	fm.nodes = filtered
	delete(fm.nodeStatus, node)
}

// GetNodes returns a snapshot of all known nodes
func (fm *FailoverManager) GetNodes() []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	nodes := make([]string, len(fm.nodes))
	copy(nodes, fm.nodes)
	return nodes
}
