package daemon

import (
	"log/slog"
	"testing"
)

func newTestDaemon() *Daemon {
	return &Daemon{
		cfg:             Config{},
		logger:          slog.Default(),
		workspaces:      make(map[string]*workspaceState),
		runtimeIndex:    make(map[string]Runtime),
		providerConfigs: make(map[string]ProviderConfig),
	}
}

func TestResolveProviderAPIKey_TaskLevelPriority(t *testing.T) {
	t.Parallel()

	d := newTestDaemon()
	d.providerConfigs["claude"] = ProviderConfig{
		Enabled: true,
		APIKey:  "workspace-key",
	}

	// Task-level key should take priority over workspace-level.
	got := d.resolveProviderAPIKey("claude", "task-key")
	if got != "task-key" {
		t.Fatalf("expected task-key, got %q", got)
	}
}

func TestResolveProviderAPIKey_FallbackToWorkspace(t *testing.T) {
	t.Parallel()

	d := newTestDaemon()
	d.providerConfigs["claude"] = ProviderConfig{
		Enabled: true,
		APIKey:  "workspace-key",
	}

	// Empty task key should fall back to workspace key.
	got := d.resolveProviderAPIKey("claude", "")
	if got != "workspace-key" {
		t.Fatalf("expected workspace-key, got %q", got)
	}
}

func TestResolveProviderAPIKey_NoneConfigured(t *testing.T) {
	t.Parallel()

	d := newTestDaemon()

	// No key configured anywhere — should return empty (user self-auth).
	got := d.resolveProviderAPIKey("claude", "")
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestResolveProviderAPIKey_DifferentProvider(t *testing.T) {
	t.Parallel()

	d := newTestDaemon()
	d.providerConfigs["claude"] = ProviderConfig{
		Enabled: true,
		APIKey:  "claude-key",
	}

	// Codex should not get claude's key.
	got := d.resolveProviderAPIKey("codex", "")
	if got != "" {
		t.Fatalf("expected empty for codex, got %q", got)
	}
}
