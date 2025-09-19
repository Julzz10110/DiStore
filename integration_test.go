package main

import (
	"bytes"
	"distore/api"
	"distore/auth"
	"distore/config"
	"distore/replication"
	"distore/storage"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIntegration(t *testing.T) {
	// Setup
	cfg := &config.Config{
		// HTTPPort:     8080,
		ReplicaCount: 2,
		Nodes:        []string{"node1", "node2"},
		Auth: config.AuthConfig{
			Enabled:    true,
			PrivateKey: "test-private-key",
			PublicKey:  "test-public-key",
		},
	}

	store := storage.NewMemoryStorage()
	replicator := replication.NewReplicator(cfg.Nodes, cfg.ReplicaCount)
	authService, _ := auth.NewAuthService(&cfg.Auth)
	handlers := api.NewHandlers(store, replicator, authService)

	t.Run("Full CRUD flow with auth", func(t *testing.T) {
		// Get token first
		tokenReq := map[string]interface{}{
			"user_id":   "test-user",
			"tenant_id": "test-tenant",
			"roles":     []string{"read", "write"},
		}
		tokenBody, _ := json.Marshal(tokenReq)

		tokenResp := httptest.NewRecorder()
		tokenReqHttp := httptest.NewRequest("POST", "/auth/token", bytes.NewReader(tokenBody))
		handlers.TokenHandler(tokenResp, tokenReqHttp)

		if tokenResp.Code != http.StatusOK {
			t.Fatalf("Token request failed: %d", tokenResp.Code)
		}

		var tokenData map[string]string
		json.NewDecoder(tokenResp.Body).Decode(&tokenData)
		token := tokenData["token"]

		// Test set with auth
		setReq := map[string]string{"key": "int-key", "value": "int-value"}
		setBody, _ := json.Marshal(setReq)

		setResp := httptest.NewRecorder()
		setReqHttp := httptest.NewRequest("POST", "/set", bytes.NewReader(setBody))
		setReqHttp.Header.Set("Authorization", "Bearer "+token)
		setReqHttp.Header.Set("Content-Type", "application/json")

		// Apply auth middleware
		authMiddleware := auth.AuthMiddleware(authService)
		rbacMiddleware := auth.RBACMiddleware(auth.RoleWrite)
		handler := authMiddleware(rbacMiddleware(http.HandlerFunc(handlers.SetHandler)))
		handler.ServeHTTP(setResp, setReqHttp)

		if setResp.Code != http.StatusCreated {
			t.Errorf("Set failed: %d", setResp.Code)
		}

		// Test get with auth
		getResp := httptest.NewRecorder()
		getReqHttp := httptest.NewRequest("GET", "/get/int-key", nil)
		getReqHttp.Header.Set("Authorization", "Bearer "+token)

		getHandler := authMiddleware(rbacMiddleware(http.HandlerFunc(handlers.GetHandler)))
		getHandler.ServeHTTP(getResp, getReqHttp)

		if getResp.Code != http.StatusOK {
			t.Errorf("Get failed: %d", getResp.Code)
		}

		var getData map[string]string
		json.NewDecoder(getResp.Body).Decode(&getData)
		if getData["value"] != "int-value" {
			t.Errorf("Expected 'int-value', got '%s'", getData["value"])
		}
	})
}
