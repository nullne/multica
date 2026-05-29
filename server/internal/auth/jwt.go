package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"
)

const defaultJWTSecret = "multica-dev-secret-change-in-production"

// Token lifetimes. The access token (JWT) is short-lived; clients silently
// exchange a long-lived refresh token for a fresh access token via /auth/refresh.
const (
	AccessTokenTTL  = 24 * time.Hour
	RefreshTokenTTL = 60 * 24 * time.Hour
)

var (
	jwtSecret     []byte
	jwtSecretOnce sync.Once
)

func JWTSecret() []byte {
	jwtSecretOnce.Do(func() {
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = defaultJWTSecret
		}
		jwtSecret = []byte(secret)
	})

	return jwtSecret
}

// GeneratePATToken creates a new personal access token: "mul_" + 40 random hex chars.
func GeneratePATToken() (string, error) {
	b := make([]byte, 20) // 20 bytes = 40 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate PAT token: %w", err)
	}
	return "mul_" + hex.EncodeToString(b), nil
}

// GenerateRefreshToken creates a new opaque refresh token: "mrt_" + 64 random
// hex chars. It is never sent in the Authorization header (only to
// /auth/refresh), so it cannot collide with the "mul_" PAT prefix.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32) // 32 bytes = 64 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return "mrt_" + hex.EncodeToString(b), nil
}

// HashToken returns the hex-encoded SHA-256 hash of a token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
