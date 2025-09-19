package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"distore/config"
)

func generateTestKeys() (string, string) {
	privateKey, _ := rsa.GenerateKey(nil, 2048)

	// Private key
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Public key
	publicKeyBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return string(privateKeyPEM), string(publicKeyPEM)
}

func TestAuthService_GenerateAndValidateToken(t *testing.T) {
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

	// Test valid token
	token, err := authService.GenerateToken("user123", "tenant1", []string{"read", "write"})
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	claims, err := authService.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected userID 'user123', got '%s'", claims.UserID)
	}
	if claims.TenantID != "tenant1" {
		t.Errorf("Expected tenantID 'tenant1', got '%s'", claims.TenantID)
	}
	if len(claims.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(claims.Roles))
	}
}

func TestAuthService_InvalidToken(t *testing.T) {
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

	// Test invalid token
	_, err = authService.ValidateToken("invalid.token.here")
	if err == nil {
		t.Error("Expected error for invalid token, got nil")
	}
}

func TestAuthService_ExpiredToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys()

	cfg := &config.AuthConfig{
		Enabled:       true,
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		TokenDuration: -1, // Negative duration for expired token
	}

	authService, err := NewAuthService(cfg)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	token, err := authService.GenerateToken("user123", "tenant1", []string{"read"})
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Wait a bit to ensure token is expired
	time.Sleep(100 * time.Millisecond)

	_, err = authService.ValidateToken(token)
	if err == nil {
		t.Error("Expected error for expired token, got nil")
	}
}
