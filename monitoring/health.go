package monitoring

import (
	"distore/replication"
	"distore/storage"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type HealthStatus struct {
	Status     string                     `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Components map[string]ComponentHealth `json:"components"`
}

type ComponentHealth struct {
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
	Latency string `json:"latency,omitempty"`
}

type HealthChecker struct {
	storage    storage.Storage
	replicator *replication.Replicator
}

func NewHealthChecker(storage storage.Storage, replicator *replication.Replicator) *HealthChecker {
	return &HealthChecker{
		storage:    storage,
		replicator: replicator,
	}
}

func (h *HealthChecker) Check() HealthStatus {
	status := HealthStatus{
		Status:     "healthy",
		Timestamp:  time.Now(),
		Components: make(map[string]ComponentHealth),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Storage check
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		_, err := h.storage.Get("__health_check__")
		latency := time.Since(start)

		mu.Lock()
		if err != nil && err != storage.ErrKeyNotFound {
			status.Components["storage"] = ComponentHealth{
				Status:  "unhealthy",
				Details: err.Error(),
				Latency: latency.String(),
			}
			status.Status = "degraded"
		} else {
			status.Components["storage"] = ComponentHealth{
				Status:  "healthy",
				Latency: latency.String(),
			}
		}
		mu.Unlock()
	}()

	// Replication check
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		nodes := h.replicator.GetNodes()
		latency := time.Since(start)

		mu.Lock()
		if len(nodes) == 0 {
			status.Components["replication"] = ComponentHealth{
				Status:  "unhealthy",
				Details: "no replica nodes available",
				Latency: latency.String(),
			}
			status.Status = "degraded"
		} else {
			status.Components["replication"] = ComponentHealth{
				Status:  "healthy",
				Details: fmt.Sprintf("%d nodes available", len(nodes)),
				Latency: latency.String(),
			}
		}
		mu.Unlock()
	}()

	wg.Wait()
	return status
}

func (h *HealthChecker) Handler(w http.ResponseWriter, r *http.Request) {
	status := h.Check()

	w.Header().Set("Content-Type", "application/json")

	if status.Status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}
