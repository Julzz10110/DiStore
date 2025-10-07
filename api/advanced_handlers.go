package api

import (
	"distore/storage"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// TTLHandler sets the key with TTL
func (h *Handlers) TTLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		TTL   int64  `json:"ttl"` // in seconds
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	ttlStorage, ok := h.storage.(*storage.TTLStorage)
	if !ok {
		http.Error(w, "TTL not supported", http.StatusNotImplemented)
		return
	}

	tenantKey := h.getTenantKey(r, req.Key)
	err := ttlStorage.SetWithTTL(tenantKey, req.Value, time.Duration(req.TTL)*time.Second)
	if err != nil {
		http.Error(w, "Error setting key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"key":    req.Key,
		"ttl":    strconv.FormatInt(req.TTL, 10),
	})
}

// IncrementHandler atomically increments the value
func (h *Handlers) IncrementHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Key   string `json:"key"`
		Delta int64  `json:"delta"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	atomicStorage, ok := h.storage.(*storage.AtomicStorage)
	if !ok {
		http.Error(w, "Atomic operations not supported", http.StatusNotImplemented)
		return
	}

	tenantKey := h.getTenantKey(r, req.Key)
	newValue, err := atomicStorage.Increment(tenantKey, req.Delta)
	if err != nil {
		http.Error(w, "Error incrementing key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key":   req.Key,
		"value": newValue,
	})
}

// BatchHandler performs batch operations
func (h *Handlers) BatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Operations []storage.BatchOperation `json:"operations"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	batchStorage, ok := h.storage.(*storage.BatchStorage)
	if !ok {
		http.Error(w, "Batch operations not supported", http.StatusNotImplemented)
		return
	}

	// Apply tenant prefix to all keys
	for i := range req.Operations {
		req.Operations[i].Key = h.getTenantKey(r, req.Operations[i].Key)
	}

	results := batchStorage.ExecuteBatch(req.Operations)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

// PerformanceStatsHandler returns performance statistics
func (h *Handlers) PerformanceStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := make(map[string]interface{})

	// Cache statistics
	if cacheStorage, ok := h.storage.(*storage.CacheStorage); ok {
		cacheStats := cacheStorage.GetCacheStats()
		stats["cache"] = map[string]interface{}{
			"hits":      cacheStats.Hits,
			"misses":    cacheStats.Misses,
			"evictions": cacheStats.Evictions,
			"hit_rate":  cacheStorage.GetHitRate(),
		}
	}

	// Compression statistics
	if compressedStorage, ok := h.storage.(*storage.CompressedStorage); ok {
		compressionStats := compressedStorage.GetCompressionStats()
		if compressionStats != nil {
			stats["compression"] = compressionStats
		}
	}

	// Bloom filter statistics
	if _, ok := h.storage.(*storage.OptimizedStorage); ok {
		stats["bloom_filter"] = map[string]interface{}{
			"enabled": true,
		}
	}

	// General information about the storage
	items, err := h.storage.GetAll()
	if err == nil {
		stats["storage"] = map[string]interface{}{
			"total_items": len(items),
			"total_size":  calculateTotalSize(items),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"performance_stats": stats,
		"timestamp":         time.Now().Unix(),
	})
}

// Helper function for calculating the total size
func calculateTotalSize(items []storage.KeyValue) int {
	total := 0
	for _, item := range items {
		total += len(item.Key) + len(item.Value)
	}
	return total
}

// CachePreloadHandler preloads data into the cache
func (h *Handlers) CachePreloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Keys []string `json:"keys"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if cacheStorage, ok := h.storage.(*storage.CacheStorage); ok {
		// apply tenant prefix to keys
		tenantKeys := make([]string, len(req.Keys))
		for i, key := range req.Keys {
			tenantKeys[i] = h.getTenantKey(r, key)
		}

		cacheStorage.PreloadKeys(tenantKeys)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "preloaded",
			"keys_loaded": len(req.Keys),
		})
	} else {
		http.Error(w, "Caching not supported", http.StatusNotImplemented)
	}
}
