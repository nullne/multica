package daemon

import (
	"testing"
	"time"
)

func TestNormalizeServerBaseURL_WSS(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("wss://app.example.com/ws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://app.example.com" {
		t.Errorf("got %q, want %q", got, "https://app.example.com")
	}
}

func TestNormalizeServerBaseURL_HTTP(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("http://localhost:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Errorf("got %q, want %q", got, "http://localhost:8080")
	}
}

func TestNormalizeServerBaseURL_HTTPS(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("https://api.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://api.example.com" {
		t.Errorf("got %q, want %q", got, "https://api.example.com")
	}
}

func TestNormalizeServerBaseURL_StripTrailingSlash(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("http://localhost:8080/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Errorf("got %q, want %q", got, "http://localhost:8080")
	}
}

func TestNormalizeServerBaseURL_StripWSPath(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("wss://app.example.com/ws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://app.example.com" {
		t.Errorf("got %q, want %q (should strip /ws path)", got, "https://app.example.com")
	}
}

func TestNormalizeServerBaseURL_PreservesNonWSPath(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("https://api.example.com/api/v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://api.example.com/api/v1" {
		t.Errorf("got %q, want %q", got, "https://api.example.com/api/v1")
	}
}

func TestNormalizeServerBaseURL_InvalidScheme(t *testing.T) {
	t.Parallel()

	_, err := NormalizeServerBaseURL("ftp://example.com")
	if err == nil {
		t.Error("expected error for invalid scheme")
	}
}

func TestNormalizeServerBaseURL_Whitespace(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("  ws://localhost:8080/ws  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Errorf("got %q, want %q", got, "http://localhost:8080")
	}
}

func TestDefaultConstants(t *testing.T) {
	t.Parallel()

	if DefaultPollInterval != 3*time.Second {
		t.Errorf("DefaultPollInterval = %v, want %v", DefaultPollInterval, 3*time.Second)
	}
	if DefaultHeartbeatInterval != 15*time.Second {
		t.Errorf("DefaultHeartbeatInterval = %v, want %v", DefaultHeartbeatInterval, 15*time.Second)
	}
	if DefaultAgentTimeout != 2*time.Hour {
		t.Errorf("DefaultAgentTimeout = %v, want %v", DefaultAgentTimeout, 2*time.Hour)
	}
	if DefaultMaxConcurrentTasks != 20 {
		t.Errorf("DefaultMaxConcurrentTasks = %d, want %d", DefaultMaxConcurrentTasks, 20)
	}
	if DefaultHealthPort != 19514 {
		t.Errorf("DefaultHealthPort = %d, want %d", DefaultHealthPort, 19514)
	}
}

func TestLoadConfig_CodexModelEmptyByDefault(t *testing.T) {
	t.Setenv("MULTICA_CODEX_PATH", "sh")
	t.Setenv("MULTICA_CODEX_MODEL", "")

	cfg, err := LoadConfig(Overrides{
		ServerURL:      "http://localhost:8080",
		WorkspacesRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	codex, ok := cfg.Agents["codex"]
	if !ok {
		t.Fatal("expected codex agent to be configured")
	}
	if codex.Model != "" {
		t.Errorf("codex model = %q, want empty (let codex CLI pick its own default)", codex.Model)
	}
}

func TestLoadConfig_CodexModelEnvOverride(t *testing.T) {
	t.Setenv("MULTICA_CODEX_PATH", "sh")
	t.Setenv("MULTICA_CODEX_MODEL", "gpt-5")

	cfg, err := LoadConfig(Overrides{
		ServerURL:      "http://localhost:8080",
		WorkspacesRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	codex, ok := cfg.Agents["codex"]
	if !ok {
		t.Fatal("expected codex agent to be configured")
	}
	if codex.Model != "gpt-5" {
		t.Errorf("codex model = %q, want %q", codex.Model, "gpt-5")
	}
}

func TestLoadConfig_OverridesApplied(t *testing.T) {
	t.Parallel()

	overrides := Overrides{
		ServerURL:          "http://localhost:9090",
		WorkspacesRoot:     t.TempDir(),
		PollInterval:       5 * time.Second,
		HeartbeatInterval:  30 * time.Second,
		AgentTimeout:       1 * time.Hour,
		MaxConcurrentTasks: 10,
		DaemonID:           "test-daemon",
		DeviceName:         "test-device",
		RuntimeName:        "Test Runtime",
		HealthPort:         9999,
	}

	cfg, err := LoadConfig(overrides)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.ServerBaseURL != "http://localhost:9090" {
		t.Errorf("ServerBaseURL = %q, want %q", cfg.ServerBaseURL, "http://localhost:9090")
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, 5*time.Second)
	}
	if cfg.HeartbeatInterval != 30*time.Second {
		t.Errorf("HeartbeatInterval = %v, want %v", cfg.HeartbeatInterval, 30*time.Second)
	}
	if cfg.AgentTimeout != 1*time.Hour {
		t.Errorf("AgentTimeout = %v, want %v", cfg.AgentTimeout, 1*time.Hour)
	}
	if cfg.MaxConcurrentTasks != 10 {
		t.Errorf("MaxConcurrentTasks = %d, want %d", cfg.MaxConcurrentTasks, 10)
	}
	if cfg.DaemonID != "test-daemon" {
		t.Errorf("DaemonID = %q, want %q", cfg.DaemonID, "test-daemon")
	}
	if cfg.DeviceName != "test-device" {
		t.Errorf("DeviceName = %q, want %q", cfg.DeviceName, "test-device")
	}
	if cfg.RuntimeName != "Test Runtime" {
		t.Errorf("RuntimeName = %q, want %q", cfg.RuntimeName, "Test Runtime")
	}
	if cfg.HealthPort != 9999 {
		t.Errorf("HealthPort = %d, want %d", cfg.HealthPort, 9999)
	}
}

func TestLoadConfig_ProfileSuffixesDaemonID(t *testing.T) {
	t.Parallel()

	cfg, err := LoadConfig(Overrides{
		ServerURL:      "http://localhost:8080",
		WorkspacesRoot: t.TempDir(),
		DaemonID:       "my-daemon",
		Profile:        "staging",
	})
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.DaemonID != "my-daemon-staging" {
		t.Errorf("DaemonID = %q, want %q (should append profile suffix)", cfg.DaemonID, "my-daemon-staging")
	}
}

func TestLoadConfig_ProfileAlreadySuffixed(t *testing.T) {
	t.Parallel()

	cfg, err := LoadConfig(Overrides{
		ServerURL:      "http://localhost:8080",
		WorkspacesRoot: t.TempDir(),
		DaemonID:       "my-daemon-staging",
		Profile:        "staging",
	})
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	// Should not double-suffix
	if cfg.DaemonID != "my-daemon-staging" {
		t.Errorf("DaemonID = %q, want %q (should not double-suffix)", cfg.DaemonID, "my-daemon-staging")
	}
}

func TestLoadConfig_InvalidServerURL(t *testing.T) {
	t.Parallel()

	_, err := LoadConfig(Overrides{
		ServerURL: "ftp://invalid",
	})
	if err == nil {
		t.Error("expected error for invalid server URL scheme")
	}
}
