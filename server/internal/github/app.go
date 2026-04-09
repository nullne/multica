package github

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// App holds the configuration for a GitHub App and can generate
// installation access tokens with scoped permissions.
type App struct {
	appID      int64
	privateKey *rsa.PrivateKey
	httpClient *http.Client
	baseURL    string
}

// NewApp creates a GitHub App client from the given app ID and PEM-encoded
// private key bytes. Returns nil if either value is zero/empty.
func NewApp(appID int64, privateKeyPEM []byte) (*App, error) {
	if appID == 0 || len(privateKeyPEM) == 0 {
		return nil, fmt.Errorf("app ID and private key are required")
	}

	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from private key")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 as fallback.
		parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("parse private key: %w (pkcs8: %w)", err, err2)
		}
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}

	return &App{
		appID:      appID,
		privateKey: key,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    "https://api.github.com",
	}, nil
}

// NewAppFromEnv creates a GitHub App from environment variables.
// Returns nil (not an error) if the env vars are not set, allowing
// the feature to be opt-in.
func NewAppFromEnv() *App {
	appIDStr := os.Getenv("MULTICA_GITHUB_APP_ID")
	if appIDStr == "" {
		return nil
	}
	var appID int64
	fmt.Sscanf(appIDStr, "%d", &appID)
	if appID == 0 {
		slog.Warn("MULTICA_GITHUB_APP_ID is set but not a valid integer")
		return nil
	}

	keyPEM := []byte(os.Getenv("MULTICA_GITHUB_APP_PRIVATE_KEY"))
	if len(keyPEM) == 0 {
		keyFile := os.Getenv("MULTICA_GITHUB_APP_PRIVATE_KEY_FILE")
		if keyFile != "" {
			var err error
			keyPEM, err = os.ReadFile(keyFile)
			if err != nil {
				slog.Warn("failed to read MULTICA_GITHUB_APP_PRIVATE_KEY_FILE", "path", keyFile, "error", err)
				return nil
			}
		}
	}
	if len(keyPEM) == 0 {
		slog.Warn("MULTICA_GITHUB_APP_ID is set but no private key provided (set MULTICA_GITHUB_APP_PRIVATE_KEY or MULTICA_GITHUB_APP_PRIVATE_KEY_FILE)")
		return nil
	}

	app, err := NewApp(appID, keyPEM)
	if err != nil {
		slog.Warn("failed to initialize GitHub App", "error", err)
		return nil
	}
	slog.Info("GitHub App initialized", "app_id", appID)
	return app
}

// createJWT generates a short-lived JWT for authenticating as the GitHub App.
func (a *App) createJWT() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    fmt.Sprintf("%d", a.appID),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(a.privateKey)
}

// InstallationToken is the response from GitHub when creating an
// installation access token.
type InstallationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// tokenRequest is the request body for creating an installation token.
type tokenRequest struct {
	Permissions  map[string]string `json:"permissions,omitempty"`
	Repositories []string          `json:"repositories,omitempty"`
}

// CreateInstallationToken generates a short-lived access token for a GitHub
// App installation, scoped to the given permissions and optional repository list.
func (a *App) CreateInstallationToken(ctx context.Context, installationID int64, permissions map[string]string, repositories []string) (*InstallationToken, error) {
	jwtToken, err := a.createJWT()
	if err != nil {
		return nil, fmt.Errorf("create JWT: %w", err)
	}

	reqBody := tokenRequest{
		Permissions: permissions,
	}
	if len(repositories) > 0 {
		reqBody.Repositories = repositories
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", a.baseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var token InstallationToken
	if err := json.Unmarshal(respBody, &token); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &token, nil
}

// InstallURL returns the URL where a user can install or configure the
// GitHub App for their organization/account. The state parameter is used
// to pass context back through the callback.
func (a *App) InstallURL(state string) string {
	return fmt.Sprintf("https://github.com/apps/%s/installations/new?state=%s", os.Getenv("MULTICA_GITHUB_APP_SLUG"), state)
}

// AppID returns the configured GitHub App ID.
func (a *App) AppID() int64 {
	return a.appID
}

// RepoNamesFromURLs extracts repository names (owner/repo format) from
// full GitHub HTTPS URLs for use in scoped installation tokens.
func RepoNamesFromURLs(urls []string) []string {
	var names []string
	for _, u := range urls {
		name := repoNameFromURL(u)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// repoNameFromURL extracts "repo" (just the repo name, not owner/repo)
// from a GitHub URL like "https://github.com/org/repo" or
// "https://github.com/org/repo.git".
func repoNameFromURL(rawURL string) string {
	rawURL = strings.TrimRight(rawURL, "/")
	rawURL = strings.TrimSuffix(rawURL, ".git")

	// HTTPS format: https://github.com/owner/repo
	if strings.Contains(rawURL, "github.com") {
		parts := strings.Split(rawURL, "/")
		if len(parts) >= 2 {
			return parts[len(parts)-1]
		}
	}
	return ""
}
