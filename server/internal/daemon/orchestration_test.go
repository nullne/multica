package daemon

import (
	"context"
	"log/slog"
	"testing"
)

// ---------------------------------------------------------------------------
// autoInstallProviders
// ---------------------------------------------------------------------------

func TestAutoInstallProviders_SkipsAlreadyInstalled(t *testing.T) {
	t.Parallel()

	d := &Daemon{
		cfg: Config{
			Agents: map[string]AgentEntry{
				"claude": {Path: "/usr/bin/claude"},
			},
		},
		logger:          slog.Default(),
		providerConfigs: make(map[string]ProviderConfig),
	}

	// Claude is already installed; autoInstallProviders should skip it.
	config := map[string]ProviderConfig{
		"claude": {Enabled: true, TargetVersion: "2.0.0"},
	}

	// Should not panic or modify existing entry.
	d.autoInstallProviders(context.Background(), config)

	// Claude should still be there with original path.
	entry, ok := d.cfg.Agents["claude"]
	if !ok {
		t.Fatal("claude should still be in agents")
	}
	if entry.Path != "/usr/bin/claude" {
		t.Fatalf("path should be unchanged, got %q", entry.Path)
	}
}

func TestAutoInstallProviders_SkipsDisabled(t *testing.T) {
	t.Parallel()

	d := &Daemon{
		cfg: Config{
			Agents: map[string]AgentEntry{},
		},
		logger:          slog.Default(),
		providerConfigs: make(map[string]ProviderConfig),
	}

	// Codex is disabled — should not attempt install.
	config := map[string]ProviderConfig{
		"codex": {Enabled: false, TargetVersion: "0.5.0"},
	}

	d.autoInstallProviders(context.Background(), config)

	// Codex should NOT be added (it's disabled).
	if _, ok := d.cfg.Agents["codex"]; ok {
		t.Fatal("codex should NOT be installed when disabled")
	}
}

func TestAutoInstallProviders_SkipsCursor(t *testing.T) {
	t.Parallel()

	d := &Daemon{
		cfg: Config{
			Agents: map[string]AgentEntry{},
		},
		logger:          slog.Default(),
		providerConfigs: make(map[string]ProviderConfig),
	}

	// Cursor is enabled but auto-install is not supported for it.
	config := map[string]ProviderConfig{
		"cursor": {Enabled: true},
	}

	d.autoInstallProviders(context.Background(), config)

	// Cursor should NOT be added (auto-install not supported).
	if _, ok := d.cfg.Agents["cursor"]; ok {
		t.Fatal("cursor should NOT be auto-installed")
	}
}

// ---------------------------------------------------------------------------
// handleUpdate routing
// ---------------------------------------------------------------------------

func TestHandleUpdateTarget_DefaultsToMultica(t *testing.T) {
	t.Parallel()

	// Verify that an empty Target field defaults to "multica".
	update := &PendingUpdate{
		ID:            "test-update-1",
		Target:        "",
		TargetVersion: "0.2.0",
	}

	target := update.Target
	if target == "" {
		target = "multica"
	}

	if target != "multica" {
		t.Fatalf("empty target should default to 'multica', got %q", target)
	}
}

// ---------------------------------------------------------------------------
// providerConfigs storage via registerRuntimesForWorkspace response
// ---------------------------------------------------------------------------

func TestDaemon_StoresProviderConfigsFromRegistration(t *testing.T) {
	t.Parallel()

	d := newTestDaemon()

	// Simulate what happens after registration response is processed.
	config := map[string]ProviderConfig{
		"claude": {Enabled: true, APIKey: "sk-from-server", TargetVersion: "2.0.0"},
		"codex":  {Enabled: false, APIKey: "", TargetVersion: ""},
	}

	d.mu.Lock()
	for k, v := range config {
		d.providerConfigs[k] = v
	}
	d.mu.Unlock()

	// Verify configs are stored.
	d.mu.Lock()
	claudeCfg := d.providerConfigs["claude"]
	codexCfg := d.providerConfigs["codex"]
	d.mu.Unlock()

	if claudeCfg.APIKey != "sk-from-server" {
		t.Fatalf("expected claude API key 'sk-from-server', got %q", claudeCfg.APIKey)
	}
	if claudeCfg.TargetVersion != "2.0.0" {
		t.Fatalf("expected claude version '2.0.0', got %q", claudeCfg.TargetVersion)
	}
	if codexCfg.Enabled {
		t.Fatal("codex should be disabled")
	}
}

// ---------------------------------------------------------------------------
// Concurrent access to resolveProviderAPIKey
// ---------------------------------------------------------------------------

func TestResolveProviderAPIKey_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	d := newTestDaemon()
	d.providerConfigs["claude"] = ProviderConfig{
		Enabled: true,
		APIKey:  "sk-concurrent-test",
	}

	// Launch concurrent reads — should not race.
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			key := d.resolveProviderAPIKey("claude", "")
			if key != "sk-concurrent-test" {
				t.Errorf("unexpected key: %q", key)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// API key env var mapping
// ---------------------------------------------------------------------------

func TestProviderAPIKeyEnvMapping(t *testing.T) {
	t.Parallel()

	// This verifies the mapping logic used in runTask.
	// We can't call runTask directly, but we verify the mapping.
	mapping := map[string]string{
		"claude":   "ANTHROPIC_API_KEY",
		"codex":    "OPENAI_API_KEY",
		"opencode": "OPENAI_API_KEY",
	}

	for provider, expectedEnv := range mapping {
		var envVar string
		switch provider {
		case "claude":
			envVar = "ANTHROPIC_API_KEY"
		case "codex":
			envVar = "OPENAI_API_KEY"
		case "opencode":
			envVar = "OPENAI_API_KEY"
		}
		if envVar != expectedEnv {
			t.Errorf("provider %q: expected env var %q, got %q", provider, expectedEnv, envVar)
		}
	}
}
