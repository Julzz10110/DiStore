package api

import (
	"distore/auth"
	"distore/replication"
	"distore/storage"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type Handlers struct {
	storage     storage.Storage
	replicator  replication.ReplicatorInterface
	authService *auth.AuthService
}

func NewHandlers(storage storage.Storage, replicator replication.ReplicatorInterface, authService *auth.AuthService) *Handlers {
	return &Handlers{
		storage:     storage,
		replicator:  replicator,
		authService: authService,
	}
}

// Handler for setting the value
func (h *Handlers) SetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var kv storage.KeyValue
	if err := json.NewDecoder(r.Body).Decode(&kv); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if kv.Key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	// Add tenant prefix if necessary
	tenantKey := h.getTenantKey(r, kv.Key)

	if err := h.storage.Set(tenantKey, kv.Value); err != nil {
		log.Printf("Error setting key %s: %v", tenantKey, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Asynchronous replication
	go func() {
		if err := h.replicator.ReplicateSet(tenantKey, kv.Value); err != nil {
			log.Printf("Replication error for key %s: %v", tenantKey, err)
			// Not fatal - just logging
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"key":    kv.Key,
	})
}

// Handler for getting the value
func (h *Handlers) GetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract the key from the path /get/{key}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}
	key := pathParts[2]

	// Add tenant prefix if necessary
	tenantKey := h.getTenantKey(r, key)

	value, err := h.storage.Get(tenantKey)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			http.Error(w, "Key not found", http.StatusNotFound)
		} else {
			log.Printf("Error getting key %s: %v", tenantKey, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"key":   key,
		"value": value,
	})
}

// Handler for deleting a value
func (h *Handlers) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}
	key := pathParts[2]

	// Add tenant prefix if necessary
	tenantKey := h.getTenantKey(r, key)

	if err := h.storage.Delete(tenantKey); err != nil {
		if err == storage.ErrKeyNotFound {
			http.Error(w, "Key not found", http.StatusNotFound)
		} else {
			log.Printf("Error deleting key %s: %v", tenantKey, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Asynchronous replication without waiting
	go func() {
		if err := h.replicator.ReplicateDelete(tenantKey); err != nil {
			log.Printf("Replication warning for delete key %s: %v", tenantKey, err)
			// Not fatal - just logging
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"key":    key,
	})
}

// Helper function for getting a key given a tenant
func (h *Handlers) getTenantKey(r *http.Request, key string) string {
	if h.authService == nil {
		return key
	}

	claims, ok := r.Context().Value("claims").(*auth.Claims)
	if ok && claims.TenantID != "" {
		return claims.TenantID + ":" + key
	}
	return key
}

// Handler for receiving a token
func (h *Handlers) TokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID   string   `json:"user_id"`
		Password string   `json:"password"`
		TenantID string   `json:"tenant_id"`
		Roles    []string `json:"roles"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// There should be a real credential check here
	// Simply generate a token for the demo
	token, err := h.authService.GenerateToken(req.UserID, req.TenantID, req.Roles)
	if err != nil {
		http.Error(w, "Error generating token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":      token,
		"token_type": "Bearer",
		"expires_in": "3600",
	})
}

// Internal handler for replication set
func (h *Handlers) InternalSetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var kv storage.KeyValue
	if err := json.NewDecoder(r.Body).Decode(&kv); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.storage.Set(kv.Key, kv.Value); err != nil {
		log.Printf("Internal error setting key %s: %v", kv.Key, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// Internal handler for delete replication
func (h *Handlers) InternalDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract the key from the path /internal/delete/{key}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}
	key := pathParts[3]

	if err := h.storage.Delete(key); err != nil && err != storage.ErrKeyNotFound {
		log.Printf("Internal error deleting key %s: %v", key, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Health check processor
func (h *Handlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simple storage availability check
	if _, err := h.storage.Get("__health_check__"); err != nil && err != storage.ErrKeyNotFound {
		http.Error(w, "Storage unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"storage": "available",
	})
}

// New handler for getting all keys
func (h *Handlers) GetAllHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	items, err := h.storage.GetAll()
	if err != nil {
		log.Printf("Error getting all items: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(items),
		"items": items,
	})
}
