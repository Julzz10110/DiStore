package replication

import (
	"bytes"
	"context"
	"distore/storage"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Replicator struct {
	nodes            []string
	replicaCount     int
	httpClient       *http.Client
	mu               sync.RWMutex
	quorumConfig     *QuorumConfig
	consistencyMgr   *ConsistencyManager
	hintedHandoff    *HintedHandoff
	conflictResolver *storage.ConflictResolver
}

type ReplicationRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewReplicator(nodes []string, replicaCount int) *Replicator {
	if replicaCount <= 0 {
		replicaCount = 1
	}
	if replicaCount > len(nodes) {
		replicaCount = len(nodes)
	}

	replicator := &Replicator{
		nodes:        nodes,
		replicaCount: replicaCount,
		httpClient: &http.Client{
			Timeout:   2 * time.Second,
			Transport: &http.Transport{MaxIdleConnsPerHost: 10},
		},
	}

	// Init extended functions only if there are multiple nodes
	if len(nodes) > 1 {
		replicator.quorumConfig = &QuorumConfig{
			WriteQuorum: (len(nodes) / 2) + 1, // N/2 + 1
			ReadQuorum:  (len(nodes) / 2) + 1,
			TotalNodes:  len(nodes),
		}
		replicator.consistencyMgr = NewConsistencyManager()
		replicator.hintedHandoff = NewHintedHandoff("./hints")
	}

	return replicator
}

func (r *Replicator) ReplicateSet(key, value string) error {
	// Use quorum recording (if configured)
	if r.quorumConfig != nil {
		return r.replicateSetWithQuorum(key, value)
	}

	// Previous logic for backward compatibility
	return r.replicateSetLegacy(key, value)
}

func (r *Replicator) replicateSetWithQuorum(key, value string) error {
	successful := 0
	failedNodes := make([]string, 0)

	for _, node := range r.nodes {
		err := r.replicateSetToNode(key, value, node)
		if err != nil {
			fmt.Printf("Replication to %s failed: %v\n", node, err)
			failedNodes = append(failedNodes, node)

			// Save hints for temporarily unavailable nodes
			if r.hintedHandoff != nil {
				if err := r.hintedHandoff.StoreHint(key, value, node); err != nil {
					fmt.Printf("Failed to store hint for %s: %v\n", node, err)
				}
			}
		} else {
			successful++
			// Write down the entry information for consistency
			if r.consistencyMgr != nil {
				r.consistencyMgr.RecordWrite(key, node)
			}
		}

		if successful >= r.quorumConfig.WriteQuorum {
			break // quorum reached
		}
	}

	if successful < r.quorumConfig.WriteQuorum {
		return fmt.Errorf("write quorum not reached: %d/%d",
			successful, r.quorumConfig.WriteQuorum)
	}

	return nil
}

func (r *Replicator) replicateSetToNode(key, value, nodeURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := ReplicationRequest{Key: key, Value: value}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/internal/set", nodeURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("replication failed: %s", resp.Status)
	}

	return nil
}

// Ьethod to read with consistency
func (r *Replicator) GetWithConsistency(key, clientID string) (string, error) {
	// Check read-your-writes consistency
	preferredNode, err := r.consistencyMgr.EnsureReadYourWrites(clientID, key)
	if err != nil {
		return "", err
	}

	// If there is a preferred node, try to read from there
	if preferredNode != "" {
		value, err := r.readFromNode(key, preferredNode)
		if err == nil {
			return value, nil
		}
	}

	// Read with a quorum
	return r.readWithQuorum(key)
}

func (r *Replicator) readWithQuorum(key string) (string, error) {
	results := make(chan string, len(r.nodes))
	errors := make(chan error, len(r.nodes))
	var wg sync.WaitGroup

	for _, node := range r.nodes {
		wg.Add(1)
		go func(nodeURL string) {
			defer wg.Done()

			value, err := r.readFromNode(key, nodeURL)
			if err != nil {
				errors <- err
			} else {
				results <- value
			}
		}(node)
	}

	wg.Wait()
	close(results)
	close(errors)

	// Collect the results and check the quorum
	valueCounts := make(map[string]int)
	for value := range results {
		valueCounts[value]++
		if valueCounts[value] >= r.quorumConfig.ReadQuorum {
			return value, nil
		}
	}

	return "", fmt.Errorf("read quorum not reached")
}

func (r *Replicator) readFromNode(key, nodeURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://%s/internal/get/%s", nodeURL, key)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", storage.ErrKeyNotFound
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var response struct {
		Value string `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	return response.Value, nil
}

func (r *Replicator) replicateSetLegacy(key, value string) error {
	replicated := 0
	errors := make(chan error, len(r.nodes))
	var wg sync.WaitGroup

	for _, node := range r.nodes {
		if replicated >= r.replicaCount {
			break
		}

		wg.Add(1)
		go func(nodeURL string) {
			defer wg.Done()

			err := r.replicateSetToNode(key, value, nodeURL)
			if err != nil {
				errors <- err
			} else {
				replicated++
			}
		}(node)
	}

	wg.Wait()
	close(errors)

	if replicated < r.replicaCount {
		return fmt.Errorf("only %d out of %d replications succeeded", replicated, r.replicaCount)
	}

	return nil
}

func (r *Replicator) ReplicateDelete(key string) error {
	r.mu.RLock()
	nodes := make([]string, len(r.nodes))
	copy(nodes, r.nodes)
	replicaCount := r.replicaCount
	r.mu.RUnlock()

	if len(nodes) == 0 {
		return nil // there are no nodes for replication
	}

	replicated := 0
	errors := make(chan error, len(nodes))
	var wg sync.WaitGroup

	for _, node := range nodes {
		if replicated >= replicaCount {
			break
		}

		wg.Add(1)
		go func(nodeURL string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			url := fmt.Sprintf("http://%s/internal/delete/%s", nodeURL, key)
			httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
			if err != nil {
				errors <- fmt.Errorf("failed to create request for %s: %w", nodeURL, err)
				return
			}

			resp, err := r.httpClient.Do(httpReq)
			if err != nil {
				errors <- fmt.Errorf("request to %s failed: %w", nodeURL, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				errors <- fmt.Errorf("replication to %s failed: %s", nodeURL, resp.Status)
				return
			}

			errors <- nil
		}(node)
	}

	// Don't block the main thread - return success immediately
	// Replication occurs asynchronously
	go func() {
		wg.Wait()
		close(errors)

		timeout := time.After(3 * time.Second)
		successful := 0

		for i := 0; i < len(nodes) && successful < replicaCount; i++ {
			select {
			case err, ok := <-errors:
				if !ok {
					break
				}
				if err == nil {
					successful++
				} else {
					// Log replication errors but do not interrupt execution
					log.Printf("Replication warning: %v", err)
				}
			case <-timeout:
				log.Printf("Replication timeout for delete key %s", key)
				return
			}
		}

		if successful < replicaCount {
			log.Printf("Replication failed for delete key %s: only %d successful", key, successful)
		}
	}()

	return nil
}

func (r *Replicator) UpdateNodes(newNodes []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodes = make([]string, len(newNodes))
	copy(r.nodes, newNodes)

	// Update replicaCount if necessary
	if r.replicaCount > len(r.nodes) {
		r.replicaCount = len(r.nodes)
	}
}

func (r *Replicator) GetNodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]string, len(r.nodes))
	copy(nodes, r.nodes)
	return nodes
}

func (r *Replicator) GetReplicaCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.replicaCount
}

func (r *Replicator) SetReplicaCount(count int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if count <= 0 {
		count = 1
	}
	if count > len(r.nodes) {
		count = len(r.nodes)
	}

	r.replicaCount = count
}
