package auth

import (
	"context"
	"net/http"
	"strings"
)

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleWrite      Role = "write"
	RoleRead       Role = "read"
	RoleReplicator Role = "replicator"
)

// AuthMiddleware for gorilla/mux
func AuthMiddleware(authService *AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Pass health check and internal endpoints
			if strings.HasPrefix(r.URL.Path, "/health") || strings.HasPrefix(r.URL.Path, "/internal/") {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			claims, err := authService.ValidateToken(parts[1])
			if err != nil {
				http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Add claims into context
			ctx := context.WithValue(r.Context(), "claims", claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RBACMiddleware for gorilla/mux
func RBACMiddleware(requiredRole Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value("claims").(*Claims)
			if !ok {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			hasRole := false
			for _, role := range claims.Roles {
				if Role(role) == requiredRole || Role(role) == RoleAdmin {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, "Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TenantMiddleware for gorilla/mux
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value("claims").(*Claims)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		// Skip tenant verification for administrators
		for _, role := range claims.Roles {
			if Role(role) == RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}
		}

		// TODO: Additional tenant checking logic can be added here.
		// For example, checking that the tenantID in the path matches the tenantID in the token

		next.ServeHTTP(w, r)
	})
}

// KeyAccessMiddleware for gorilla/mux
func KeyAccessMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// claims, ok := r.Context().Value("claims").(*Claims)
		_, ok := r.Context().Value("claims").(*Claims)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		// TODO: Here you can add logic for checking access to keys
		// For example, prefix check: user_* is only accessible to the owner

		next.ServeHTTP(w, r)
	})
}

// PublicMiddleware allows access without authentication
func PublicMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pass the request without checking authentication
		next.ServeHTTP(w, r)
	})
}
