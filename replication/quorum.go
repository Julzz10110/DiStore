package replication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type QuorumConfig struct {
	WriteQuorum int
	ReadQuorum  int
	TotalNodes  int
}

type QuorumReplicator struct {
	*Replicator
	quorumConfig QuorumConfig
}

func NewQuorumReplicator(nodes []string, writeQuorum, readQuorum int) *QuorumReplicator {
	replicaCount := writeQuorum // use write quorum for backward compatibility
	if replicaCount > len(nodes) {
		replicaCount = len(nodes)
	}

	return &QuorumReplicator{
		Replicator: NewReplicator(nodes, replicaCount),
		quorumConfig: QuorumConfig{
			WriteQuorum: writeQuorum,
			ReadQuorum:  readQuorum,
			TotalNodes:  len(nodes),
		},
	}
}

func (q *QuorumReplicator) ReplicateSetWithQuorum(key, value string) error {
	if q.quorumConfig.WriteQuorum > q.quorumConfig.TotalNodes {
		return fmt.Errorf("write quorum %d exceeds total nodes %d",
			q.quorumConfig.WriteQuorum, q.quorumConfig.TotalNodes)
	}

	successful := 0
	errors := make(chan error, len(q.nodes))
	var wg sync.WaitGroup

	for _, node := range q.nodes {
		wg.Add(1)
		go func(nodeURL string) {
			defer wg.Done()

			err := q.replicateSetToNode(key, value, nodeURL)
			if err != nil {
				errors <- err
			} else {
				successful++
			}
		}(node)
	}

	wg.Wait()
	close(errors)

	if successful < q.quorumConfig.WriteQuorum {
		return fmt.Errorf("write quorum not reached: %d/%d successful",
			successful, q.quorumConfig.WriteQuorum)
	}

	return nil
}

func (q *QuorumReplicator) replicateSetToNode(key, value, nodeURL string) error {
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

	resp, err := q.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("replication failed: %s", resp.Status)
	}

	return nil
}
