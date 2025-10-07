package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"sort"
	"sync"
	"time"

	"distore/storage"
)

// Rebalancer provides basic consistent-hashing based key movement when topology changes
// NodeLister is a minimal interface for listing cluster nodes
type NodeLister interface {
	GetNodes() []string
}

type Rebalancer struct {
	mu    sync.RWMutex
	store storage.Storage
	nodes NodeLister
	self  string // this node address, e.g., host:port
}

func NewRebalancer(store storage.Storage, nodes NodeLister, self string) *Rebalancer {
	return &Rebalancer{store: store, nodes: nodes, self: self}
}

// TriggerRebalance moves keys that no longer belong to this node to their new owners
// It uses a simple consistent hash over node list
func (r *Rebalancer) TriggerRebalance() (moved int, err error) {
	r.mu.RLock()
	nodes := r.nodes.GetNodes()
	self := r.self
	r.mu.RUnlock()

	if len(nodes) == 0 {
		return 0, nil
	}

	// Build hash ring order (stable sort)
	sort.Strings(nodes)

	items, err := r.store.GetAll()
	if err != nil {
		return 0, err
	}

	client := &http.Client{Timeout: 3 * time.Second}
	movedCount := 0

	for _, item := range items {
		owner := chooseOwner(item.Key, nodes)
		if owner == self {
			continue
		}

		// send to owner via internal set
		reqBody, _ := json.Marshal(map[string]string{"key": item.Key, "value": item.Value})
		url := fmt.Sprintf("http://%s/internal/set", owner)
		req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode < 300 {
			// delete local copy
			_ = r.store.Delete(item.Key)
			movedCount++
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	return movedCount, nil
}

func chooseOwner(key string, nodes []string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	idx := int(h.Sum32()) % len(nodes)
	if idx < 0 {
		idx = -idx
	}
	return nodes[idx]
}
