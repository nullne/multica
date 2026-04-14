package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestGeneratePATToken_Format(t *testing.T) {
	token, err := GeneratePATToken()
	if err != nil {
		t.Fatalf("GeneratePATToken returned error: %v", err)
	}
	if !strings.HasPrefix(token, "mul_") {
		t.Errorf("token should start with 'mul_', got %q", token)
	}
	// "mul_" + 40 hex chars = 44 total
	if len(token) != 44 {
		t.Errorf("token length = %d, want 44", len(token))
	}
}

func TestGeneratePATToken_Uniqueness(t *testing.T) {
	token1, _ := GeneratePATToken()
	token2, _ := GeneratePATToken()
	if token1 == token2 {
		t.Error("two generated PAT tokens should be different")
	}
}

func TestGeneratePATToken_HexChars(t *testing.T) {
	token, _ := GeneratePATToken()
	hexPart := token[4:] // strip "mul_"
	if _, err := hex.DecodeString(hexPart); err != nil {
		t.Errorf("hex part of token is not valid hex: %v", err)
	}
}

func TestGenerateDaemonToken_Format(t *testing.T) {
	token, err := GenerateDaemonToken()
	if err != nil {
		t.Fatalf("GenerateDaemonToken returned error: %v", err)
	}
	if !strings.HasPrefix(token, "mdt_") {
		t.Errorf("token should start with 'mdt_', got %q", token)
	}
	// "mdt_" + 40 hex chars = 44 total
	if len(token) != 44 {
		t.Errorf("token length = %d, want 44", len(token))
	}
}

func TestGenerateDaemonToken_Uniqueness(t *testing.T) {
	token1, _ := GenerateDaemonToken()
	token2, _ := GenerateDaemonToken()
	if token1 == token2 {
		t.Error("two generated daemon tokens should be different")
	}
}

func TestHashToken(t *testing.T) {
	token := "mul_abcdef1234567890abcdef1234567890abcdef12"
	hash := HashToken(token)

	// Verify it's a valid hex-encoded SHA-256 hash (64 hex chars)
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
	if _, err := hex.DecodeString(hash); err != nil {
		t.Errorf("hash is not valid hex: %v", err)
	}

	// Verify deterministic
	hash2 := HashToken(token)
	if hash != hash2 {
		t.Error("HashToken should be deterministic")
	}
}

func TestHashToken_MatchesSHA256(t *testing.T) {
	token := "test-token"
	expected := sha256.Sum256([]byte(token))
	expectedHex := hex.EncodeToString(expected[:])

	got := HashToken(token)
	if got != expectedHex {
		t.Errorf("HashToken = %q, want %q", got, expectedHex)
	}
}

func TestHashToken_DifferentInputs(t *testing.T) {
	hash1 := HashToken("token1")
	hash2 := HashToken("token2")
	if hash1 == hash2 {
		t.Error("different tokens should produce different hashes")
	}
}
