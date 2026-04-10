package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	return key, pemBlock
}

func TestNewApp_Valid(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	app, err := NewApp(12345, pemBytes)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}
	if app.AppID() != 12345 {
		t.Errorf("AppID() = %d, want 12345", app.AppID())
	}
}

func TestNewApp_ZeroID(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	_, err := NewApp(0, pemBytes)
	if err == nil {
		t.Fatal("expected error for zero app ID")
	}
}

func TestNewApp_EmptyKey(t *testing.T) {
	_, err := NewApp(12345, nil)
	if err == nil {
		t.Fatal("expected error for nil key")
	}
}

func TestNewApp_InvalidPEM(t *testing.T) {
	_, err := NewApp(12345, []byte("not-a-pem"))
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestCreateJWT(t *testing.T) {
	key, pemBytes := generateTestKey(t)
	app, err := NewApp(42, pemBytes)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	tokenStr, err := app.createJWT()
	if err != nil {
		t.Fatalf("createJWT: %v", err)
	}

	parsed, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &key.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("parse JWT: %v", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("claims not MapClaims")
	}
	if claims["iss"] != "42" {
		t.Errorf("iss = %v, want 42", claims["iss"])
	}
}

func TestCreateInstallationToken_Success(t *testing.T) {
	_, pemBytes := generateTestKey(t)

	mockResp := InstallationToken{
		Token:     "ghs_test_token_abc123",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	var capturedPerms map[string]string
	var capturedRepos []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/app/installations/99/access_tokens" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if auth == "" || auth[:7] != "Bearer " {
			t.Errorf("missing Bearer token in Authorization header")
		}

		var body tokenRequest
		json.NewDecoder(r.Body).Decode(&body)
		capturedPerms = body.Permissions
		capturedRepos = body.Repositories

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.baseURL = server.URL

	perms := PermissionsForCodeAccess(CodeAccessWrite)
	repos := []string{"my-repo"}

	token, err := app.CreateInstallationToken(context.Background(), 99, perms, repos)
	if err != nil {
		t.Fatalf("CreateInstallationToken: %v", err)
	}
	if token.Token != "ghs_test_token_abc123" {
		t.Errorf("token = %q, want ghs_test_token_abc123", token.Token)
	}
	if capturedPerms["contents"] != "write" {
		t.Errorf("captured perms contents = %q, want write", capturedPerms["contents"])
	}
	if len(capturedRepos) != 1 || capturedRepos[0] != "my-repo" {
		t.Errorf("captured repos = %v, want [my-repo]", capturedRepos)
	}
}

func TestCreateInstallationToken_APIError(t *testing.T) {
	_, pemBytes := generateTestKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.baseURL = server.URL

	_, err = app.CreateInstallationToken(context.Background(), 99, nil, nil)
	if err == nil {
		t.Fatal("expected error for 422 response")
	}
}

func TestCreateInstallationToken_NoRepos(t *testing.T) {
	_, pemBytes := generateTestKey(t)

	var capturedBody tokenRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(InstallationToken{Token: "tok", ExpiresAt: time.Now()})
	}))
	defer server.Close()

	app, _ := NewApp(1, pemBytes)
	app.baseURL = server.URL

	_, err := app.CreateInstallationToken(context.Background(), 1, PermissionsForCodeAccess("read"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBody.Repositories != nil {
		t.Errorf("expected nil repositories, got %v", capturedBody.Repositories)
	}
}

func TestListInstallationRepositories_Success(t *testing.T) {
	_, pemBytes := generateTestKey(t)

	getCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/77/access_tokens":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(InstallationToken{Token: "inst_tok", ExpiresAt: time.Now().Add(1 * time.Hour)})
		case r.Method == http.MethodGet && r.URL.Path == "/installation/repositories":
			getCalls++
			page := r.URL.Query().Get("page")
			if page == "1" {
				repos := make([]InstallationRepository, 100)
				for i := range repos {
					idx := i + 1
					repos[i] = InstallationRepository{
						ID:       int64(idx),
						Name:     fmt.Sprintf("repo-%d", idx),
						FullName: fmt.Sprintf("org/repo-%d", idx),
						HTMLURL:  fmt.Sprintf("https://github.com/org/repo-%d", idx),
					}
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(listInstallationRepositoriesResponse{Repositories: repos})
				return
			}
			if page == "2" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(listInstallationRepositoriesResponse{
					Repositories: []InstallationRepository{
						{
							ID:       101,
							Name:     "repo-101",
							FullName: "org/repo-101",
							HTMLURL:  "https://github.com/org/repo-101",
						},
					},
				})
				return
			}
			t.Fatalf("unexpected page query: %s", page)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.baseURL = server.URL

	repos, err := app.ListInstallationRepositories(context.Background(), 77)
	if err != nil {
		t.Fatalf("ListInstallationRepositories: %v", err)
	}

	if getCalls != 2 {
		t.Fatalf("expected 2 repository page calls, got %d", getCalls)
	}
	if len(repos) != 101 {
		t.Fatalf("expected 101 repositories, got %d", len(repos))
	}
	if repos[0].FullName != "org/repo-1" {
		t.Errorf("first repo full_name = %q, want org/repo-1", repos[0].FullName)
	}
	if repos[100].FullName != "org/repo-101" {
		t.Errorf("last repo full_name = %q, want org/repo-101", repos[100].FullName)
	}
}

func TestListInstallationRepositories_APIError(t *testing.T) {
	_, pemBytes := generateTestKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/88/access_tokens":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(InstallationToken{Token: "inst_tok", ExpiresAt: time.Now().Add(1 * time.Hour)})
		case r.Method == http.MethodGet && r.URL.Path == "/installation/repositories":
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"forbidden"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	app, err := NewApp(1, pemBytes)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.baseURL = server.URL

	_, err = app.ListInstallationRepositories(context.Background(), 88)
	if err == nil {
		t.Fatal("expected error for non-200 repositories response")
	}
	if !strings.Contains(err.Error(), "GitHub API returned 403") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepoNamesFromURLs(t *testing.T) {
	urls := []string{
		"https://github.com/org/backend",
		"https://github.com/org/frontend.git",
		"https://github.com/org/repo/",
		"not-a-github-url",
	}
	names := RepoNamesFromURLs(urls)

	expected := []string{"backend", "frontend", "repo"}
	if len(names) != len(expected) {
		t.Fatalf("got %d names, want %d: %v", len(names), len(expected), names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/org/repo", "repo"},
		{"https://github.com/org/repo.git", "repo"},
		{"https://github.com/org/my-repo/", "my-repo"},
		{"https://example.com/foo/bar", ""},
	}
	for _, tt := range tests {
		if got := repoNameFromURL(tt.url); got != tt.want {
			t.Errorf("repoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
