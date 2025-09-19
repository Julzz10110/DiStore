package replication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Replicator struct {
	nodes        []string
	replicaCount int
	httpClient   *http.Client
}

func NewReplicator(nodes []string, replicaCount int) *Replicator {
	return &Replicator{
		nodes:        nodes,
		replicaCount: replicaCount,
		httpClient: &http.Client{
			Timeout: 2 * time.Second, // Short timeout
		},
	}
}

type ReplicationRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (r *Replicator) ReplicateSet(key, value string) error {
	req := ReplicationRequest{Key: key, Value: value}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return err
	}

	replicated := 0
	errors := make(chan error, len(r.nodes))

	for _, node := range r.nodes {
		if replicated >= r.replicaCount {
			break
		}

		go func(nodeURL string) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			url := fmt.Sprintf("http://%s/internal/set", nodeURL)
			httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
			if err != nil {
				errors <- err
				return
			}
			httpReq.Header.Set("Content-Type", "application/json")

			resp, err := r.httpClient.Do(httpReq)
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				errors <- fmt.Errorf("replication failed: %s", resp.Status)
				return
			}

			errors <- nil
		}(node)
	}

	// Wait for successful replications or a timeout
	timeout := time.After(3 * time.Second)
	for i := 0; i < len(r.nodes) && replicated < r.replicaCount; i++ {
		select {
		case err := <-errors:
			if err == nil {
				replicated++
			}
		case <-timeout:
			return fmt.Errorf("replication timeout")
		}
	}

	if replicated < r.replicaCount {
		return fmt.Errorf("only %d out of %d replications succeeded", replicated, r.replicaCount)
	}

	return nil
}

func (r *Replicator) ReplicateDelete(key string) error {
	replicated := 0
	errors := make(chan error, len(r.nodes))

	for _, node := range r.nodes {
		if replicated >= r.replicaCount {
			break
		}

		go func(nodeURL string) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			url := fmt.Sprintf("http://%s/internal/delete/%s", nodeURL, key)
			httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
			if err != nil {
				errors <- err
				return
			}

			resp, err := r.httpClient.Do(httpReq)
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				errors <- fmt.Errorf("replication failed: %s", resp.Status)
				return
			}

			errors <- nil
		}(node)
	}

	// Don't block the main thread - return success immediately
	// Replication occurs asynchronously
	go func() {
		timeout := time.After(3 * time.Second)
		successful := 0

		for i := 0; i < len(r.nodes) && successful < r.replicaCount; i++ {
			select {
			case err := <-errors:
				if err == nil {
					successful++
				}
			case <-timeout:
				return
			}
		}
	}()

	return nil
}
