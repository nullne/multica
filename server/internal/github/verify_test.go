package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func computeSignature(secret, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature_Valid(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("test-webhook-secret")
	app.SetWebhookSecret(secret)

	payload := []byte(`{"action":"opened"}`)
	sig := computeSignature(secret, payload)

	if !app.VerifySignature(payload, sig) {
		t.Error("expected valid signature to pass verification")
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	app.SetWebhookSecret([]byte("correct-secret"))

	payload := []byte(`{"action":"opened"}`)
	wrongSig := computeSignature([]byte("wrong-secret"), payload)

	if app.VerifySignature(payload, wrongSig) {
		t.Error("expected invalid signature to fail verification")
	}
}

func TestVerifySignature_NoSecret(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte(`{"action":"opened"}`)
	sig := computeSignature([]byte("any"), payload)

	if app.VerifySignature(payload, sig) {
		t.Error("expected verification to fail when no secret is configured")
	}
}

func TestVerifySignature_EmptyHeader(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	app.SetWebhookSecret([]byte("secret"))

	if app.VerifySignature([]byte("body"), "") {
		t.Error("expected verification to fail with empty header")
	}
}

func TestVerifySignature_NoPrefix(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	app.SetWebhookSecret([]byte("secret"))

	if app.VerifySignature([]byte("body"), "deadbeef") {
		t.Error("expected verification to fail without sha256= prefix")
	}
}

func TestVerifySignature_InvalidHex(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	app.SetWebhookSecret([]byte("secret"))

	if app.VerifySignature([]byte("body"), "sha256=not-valid-hex") {
		t.Error("expected verification to fail with invalid hex")
	}
}
