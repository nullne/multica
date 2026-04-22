package auth

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const firebaseCertsURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"

var ErrFirebaseNotConfigured = errors.New("firebase auth is not configured")

type FirebaseIdentity struct {
	Email      string
	Name       string
	PictureURL *string
}

type FirebaseVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*FirebaseIdentity, error)
}

type firebaseVerifier struct {
	projectID string
	client    *http.Client

	mu         sync.RWMutex
	publicKeys map[string]any
	expiresAt  time.Time
}

func NewFirebaseVerifierFromEnv() FirebaseVerifier {
	projectID := strings.TrimSpace(os.Getenv("FIREBASE_PROJECT_ID"))
	if projectID == "" {
		return nil
	}

	return &firebaseVerifier{
		projectID: projectID,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *firebaseVerifier) VerifyIDToken(ctx context.Context, idToken string) (*FirebaseIdentity, error) {
	if strings.TrimSpace(v.projectID) == "" {
		return nil, ErrFirebaseNotConfigured
	}

	token, err := jwt.Parse(idToken, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("firebase token is missing kid header")
		}
		return v.publicKeyFor(ctx, kid)
	}, jwt.WithAudience(v.projectID), jwt.WithIssuer("https://securetoken.google.com/"+v.projectID), jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid firebase token")
	}

	sub, _ := claims["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return nil, errors.New("firebase token subject is missing")
	}

	email, _ := claims["email"].(string)
	if strings.TrimSpace(email) == "" {
		return nil, errors.New("firebase token email is missing")
	}

	if verified, ok := claims["email_verified"].(bool); ok && !verified {
		return nil, errors.New("firebase email is not verified")
	}

	name, _ := claims["name"].(string)
	picture, _ := claims["picture"].(string)

	identity := &FirebaseIdentity{
		Email: strings.ToLower(strings.TrimSpace(email)),
		Name:  strings.TrimSpace(name),
	}
	if picture != "" {
		identity.PictureURL = &picture
	}
	return identity, nil
}

func (v *firebaseVerifier) publicKeyFor(ctx context.Context, kid string) (any, error) {
	if key := v.cachedKey(kid); key != nil {
		return key, nil
	}
	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}
	if key := v.cachedKey(kid); key != nil {
		return key, nil
	}
	return nil, fmt.Errorf("firebase signing key %q not found", kid)
}

func (v *firebaseVerifier) cachedKey(kid string) any {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if time.Now().After(v.expiresAt) {
		return nil
	}
	return v.publicKeys[kid]
}

func (v *firebaseVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, firebaseCertsURL, nil)
	if err != nil {
		return err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("firebase cert request failed with status %d", resp.StatusCode)
	}

	var raw map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return err
	}

	keys := make(map[string]any, len(raw))
	for kid, certPEM := range raw {
		block, _ := pem.Decode([]byte(certPEM))
		if block == nil {
			return fmt.Errorf("failed to decode firebase cert for key %q", kid)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse firebase cert for key %q: %w", kid, err)
		}
		keys[kid] = cert.PublicKey
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.publicKeys = keys
	v.expiresAt = time.Now().Add(cacheMaxAge(resp.Header.Get("Cache-Control")))
	return nil
}

func cacheMaxAge(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "max-age=") {
			continue
		}
		seconds, err := strconv.Atoi(strings.TrimPrefix(part, "max-age="))
		if err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return time.Hour
}
