package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nullne/multica/server/internal/codeagent"
)

func TestRedactProviderSettingsReturnsRepoVersions(t *testing.T) {
	t.Parallel()

	out := redactProviderSettings(WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"codex": {Enabled: true, APIKey: "sk-test-1234"},
		},
	})

	codex := out.Providers["codex"]
	if !codex.Enabled {
		t.Fatal("expected codex enabled")
	}
	if !strings.Contains(codex.APIKey, "1234") {
		t.Fatalf("expected redacted API key to retain suffix, got %q", codex.APIKey)
	}
	want, _ := codeagent.Version("codex")
	if codex.TargetVersion != want {
		t.Fatalf("expected codex target_version %q, got %q", want, codex.TargetVersion)
	}

	claude := out.Providers["claude"]
	want, _ = codeagent.Version("claude")
	if claude.TargetVersion != want {
		t.Fatalf("expected claude target_version %q, got %q", want, claude.TargetVersion)
	}
}

func TestRejectConfiguredTargetVersions(t *testing.T) {
	t.Parallel()

	err := rejectConfiguredTargetVersions(WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"codex": {Enabled: true, TargetVersion: "0.1.0"},
		},
	})
	if err == nil {
		t.Fatal("expected target_version to be rejected")
	}
}

func TestMergeProviderSettingsRemovesMulticaTargetVersion(t *testing.T) {
	t.Parallel()

	merged, err := mergeProviderSettingsIntoWorkspace(
		[]byte(`{"multica_target_version":"0.1.0","other":true}`),
		WorkspaceProviderSettings{Providers: map[string]ProviderConfig{}},
	)
	if err != nil {
		t.Fatalf("mergeProviderSettingsIntoWorkspace: %v", err)
	}
	if strings.Contains(string(merged), "multica_target_version") {
		t.Fatalf("expected multica_target_version to be removed, got %s", merged)
	}
}

func TestValidateProviderAPIKeyMapsStatuses(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		status   int
		want     ProviderValidationStatus
	}{
		{name: "valid", provider: "codex", status: http.StatusOK, want: ProviderValidationValid},
		{name: "invalid", provider: "codex", status: http.StatusUnauthorized, want: ProviderValidationInvalid},
		{name: "unavailable", provider: "claude", status: http.StatusTooManyRequests, want: ProviderValidationUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.provider == "claude" {
					if r.Header.Get("x-api-key") != "test-key" {
						t.Fatalf("expected anthropic API key header")
					}
					if r.Header.Get("anthropic-version") == "" {
						t.Fatalf("expected anthropic-version header")
					}
				} else if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Fatalf("expected bearer token header, got %q", got)
				}
				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			oldURLs := providerValidationURLs
			oldClient := providerValidationHTTPClient
			providerValidationURLs = map[string]string{tt.provider: server.URL}
			providerValidationHTTPClient = server.Client()
			defer func() {
				providerValidationURLs = oldURLs
				providerValidationHTTPClient = oldClient
			}()

			got := validateProviderAPIKey(t.Context(), tt.provider, "test-key")
			if got.Status != tt.want {
				t.Fatalf("expected status %q, got %q", tt.want, got.Status)
			}
		})
	}
}

func TestValidateProviderAPIKeyUnsupportedAndEmpty(t *testing.T) {
	unsupported := validateProviderAPIKey(t.Context(), "cursor", "test-key")
	if unsupported.Status != ProviderValidationUnsupported {
		t.Fatalf("expected unsupported status, got %q", unsupported.Status)
	}

	empty := validateProviderAPIKey(t.Context(), "codex", "")
	if empty.Status != ProviderValidationInvalid {
		t.Fatalf("expected invalid status for empty key, got %q", empty.Status)
	}
}
