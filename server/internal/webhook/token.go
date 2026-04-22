package webhook

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateToken creates a new webhook secret token: "whk_" + 40 random hex chars.
func GenerateToken() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate webhook token: %w", err)
	}
	return "whk_" + hex.EncodeToString(b), nil
}
