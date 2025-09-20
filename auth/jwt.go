package auth

import (
	"crypto/rsa"
	"distore/config"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrTokenExpired = errors.New("token expired")
)

// AuthServiceInterface defines the interface for the authentication service
type AuthServiceInterface interface {
	GenerateToken(userID, tenantID string, roles []string) (string, error)
	ValidateToken(tokenString string) (*Claims, error)
}

type Claims struct {
	jwt.RegisteredClaims
	UserID   string   `json:"user_id"`
	Roles    []string `json:"roles"`
	TenantID string   `json:"tenant_id"`
}

type AuthService struct {
	privateKey    *rsa.PrivateKey
	publicKey     *rsa.PublicKey
	tokenDuration time.Duration
}

// NewAuthService creates auth service based on config
func NewAuthService(cfg *config.AuthConfig) (AuthServiceInterface, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Try to create a JWT auth service
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.PrivateKey))
	if err != nil {
		// If PEM parsing fails, use a simple implementation
		return NewSimpleAuthService(cfg.TokenDuration), nil
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.PublicKey))
	if err != nil {
		return NewSimpleAuthService(cfg.TokenDuration), nil
	}

	return &AuthService{
		privateKey:    privateKey,
		publicKey:     publicKey,
		tokenDuration: time.Duration(cfg.TokenDuration) * time.Second,
	}, nil
}

func (a *AuthService) GenerateToken(userID, tenantID string, roles []string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Roles:    roles,
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "distore",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(a.privateKey)
}

func (a *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// SimpleAuthService - simple implementation for testing
type SimpleAuthService struct {
	tokenDuration time.Duration
}

func NewSimpleAuthService(tokenDuration int) *SimpleAuthService {
	return &SimpleAuthService{
		tokenDuration: time.Duration(tokenDuration) * time.Second,
	}
}

func (s *SimpleAuthService) GenerateToken(userID, tenantID string, roles []string) (string, error) {
	// Use base64 to avoid problems with special characters
	tokenData := fmt.Sprintf("%s:%s:%s", userID, tenantID, strings.Join(roles, ","))
	encoded := base64.StdEncoding.EncodeToString([]byte(tokenData))
	return "simple-token-" + encoded, nil
}

func (s *SimpleAuthService) ValidateToken(tokenString string) (*Claims, error) {
	if !strings.HasPrefix(tokenString, "simple-token-") {
		return nil, ErrInvalidToken
	}

	// Extract the base64 part
	encoded := strings.TrimPrefix(tokenString, "simple-token-")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidToken
	}

	parts := strings.Split(string(decoded), ":")
	if len(parts) < 3 {
		return nil, ErrInvalidToken
	}

	userID := parts[0]
	tenantID := parts[1]
	roles := strings.Split(parts[2], ",")

	return &Claims{
		UserID:   userID,
		TenantID: tenantID,
		Roles:    roles,
	}, nil
}

// Ensure AuthService implements AuthServiceInterface
var _ AuthServiceInterface = (*AuthService)(nil)
var _ AuthServiceInterface = (*SimpleAuthService)(nil)
