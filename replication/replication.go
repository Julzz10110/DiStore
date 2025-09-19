package replication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Replicator struct {
	nodes        []string
	replicaCount int
	httpClient   *http.Client
	mu           sync.RWMutex
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

	return &Replicator{
		nodes:        nodes,
		replicaCount: replicaCount,
		httpClient: &http.Client{
			Timeout:   2 * time.Second,
			Transport: &http.Transport{MaxIdleConnsPerHost: 10},
		},
	}
}

func (r *Replicator) ReplicateSet(key, value string) error {
	r.mu.RLock()
	nodes := make([]string, len(r.nodes))
	copy(nodes, r.nodes)
	replicaCount := r.replicaCount
	r.mu.RUnlock()

	if len(nodes) == 0 {
		return nil // there are no nodes for replication
	}

	req := ReplicationRequest{Key: key, Value: value}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal replication request: %w", err)
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

			url := fmt.Sprintf("http://%s/internal/set", nodeURL)
			httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
			if err != nil {
				errors <- fmt.Errorf("failed to create request for %s: %w", nodeURL, err)
				return
			}
			httpReq.Header.Set("Content-Type", "application/json")

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

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(errors)
	}()

	// Collect results
	timeout := time.After(3 * time.Second)
	for i := 0; i < len(nodes) && replicated < replicaCount; i++ {
		select {
		case err, ok := <-errors:
			if !ok {
				break
			}
			if err == nil {
				replicated++
			}
		case <-timeout:
			return ErrReplicationFailed
		}
	}

	if replicated < replicaCount {
		return fmt.Errorf("%w: only %d out of %d replications succeeded", ErrQuorumNotReached, replicated, replicaCount)
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
