package auth

import (
	"context"
	"distore/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	privateKey, publicKey := generateTestKeys()

	cfg := &config.AuthConfig{
		Enabled:       true,
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		TokenDuration: 3600,
	}

	authService, err := NewAuthService(cfg)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	token, err := authService.GenerateToken("test-user", "test-tenant", []string{"read"})
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		path           string
	}{
		{
			name:           "Valid token",
			authHeader:     "Bearer " + token,
			expectedStatus: http.StatusOK,
			path:           "/set",
		},
		{
			name:           "No authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			path:           "/set",
		},
		{
			name:           "Invalid token format",
			authHeader:     "InvalidFormat",
			expectedStatus: http.StatusUnauthorized,
			path:           "/set",
		},
		{
			name:           "Health endpoint without auth",
			authHeader:     "",
			expectedStatus: http.StatusOK,
			path:           "/health",
		},
		{
			name:           "Internal endpoint without auth",
			authHeader:     "",
			expectedStatus: http.StatusOK,
			path:           "/internal/set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			handler := AuthMiddleware(authService)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestRBACMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		roles          []string
		requiredRole   Role
		expectedStatus int
	}{
		{
			name:           "Admin has access to read",
			roles:          []string{"admin"},
			requiredRole:   RoleRead,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Read role has access to read",
			roles:          []string{"read"},
			requiredRole:   RoleRead,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Write role has access to read",
			roles:          []string{"write"},
			requiredRole:   RoleRead,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "No roles - access denied",
			roles:          []string{},
			requiredRole:   RoleRead,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Read role cannot write",
			roles:          []string{"read"},
			requiredRole:   RoleWrite,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/set", nil)
			ctx := context.WithValue(req.Context(), "claims", &Claims{Roles: tt.roles})
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler := RBACMiddleware(tt.requiredRole)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}
