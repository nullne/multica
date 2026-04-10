package handler

import (
	"encoding/json"
	"testing"
)

func TestRedactAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"abc", "****"},
		{"abcd", "****"},
		{"sk-ant-api03-abcdef", "***************cdef"},
		{"sk-1234567890", "********7890"},
	}
	for _, tc := range tests {
		got := redactAPIKey(tc.input)
		if got != tc.want {
			t.Errorf("redactAPIKey(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsRedacted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"****", true},
		{"********", true},
		{"sk-real-key", false},
		{"****abcd", false}, // partially masked — not all asterisks
	}
	for _, tc := range tests {
		got := isRedacted(tc.input)
		if got != tc.want {
			t.Errorf("isRedacted(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseProviderSettings_Empty(t *testing.T) {
	t.Parallel()

	ps := parseProviderSettings(nil)
	if ps.Providers != nil {
		t.Fatalf("expected nil providers, got %v", ps.Providers)
	}
	if ps.MulticaTargetVersion != "" {
		t.Fatalf("expected empty version, got %q", ps.MulticaTargetVersion)
	}
}

func TestParseProviderSettings_Valid(t *testing.T) {
	t.Parallel()

	raw, _ := json.Marshal(map[string]any{
		"providers": map[string]any{
			"claude": map[string]any{
				"enabled":        true,
				"api_key":        "sk-ant-test123",
				"target_version": "2.0.0",
			},
		},
		"multica_target_version": "0.1.15",
		"other_setting":          "preserved",
	})

	ps := parseProviderSettings(raw)
	if ps.MulticaTargetVersion != "0.1.15" {
		t.Fatalf("expected multica version 0.1.15, got %q", ps.MulticaTargetVersion)
	}
	if len(ps.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(ps.Providers))
	}
	claude, ok := ps.Providers["claude"]
	if !ok {
		t.Fatal("expected claude provider")
	}
	if !claude.Enabled {
		t.Fatal("expected claude to be enabled")
	}
	if claude.APIKey != "sk-ant-test123" {
		t.Fatalf("expected api key sk-ant-test123, got %q", claude.APIKey)
	}
	if claude.TargetVersion != "2.0.0" {
		t.Fatalf("expected target version 2.0.0, got %q", claude.TargetVersion)
	}
}

func TestParseProviderSettings_InvalidJSON(t *testing.T) {
	t.Parallel()

	ps := parseProviderSettings([]byte("not json"))
	if ps.Providers != nil {
		t.Fatalf("expected nil providers for invalid JSON, got %v", ps.Providers)
	}
}

func TestMergeProviderSettingsPreservesOtherFields(t *testing.T) {
	t.Parallel()

	existing, _ := json.Marshal(map[string]any{
		"other_setting": "should_be_preserved",
	})

	ps := WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"claude": {Enabled: true, APIKey: "sk-test"},
		},
		MulticaTargetVersion: "0.2.0",
	}

	merged, err := mergeProviderSettingsIntoWorkspace(existing, ps)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(merged, &result); err != nil {
		t.Fatalf("unmarshal merged result: %v", err)
	}

	// Check that other_setting is preserved.
	if _, ok := result["other_setting"]; !ok {
		t.Fatal("other_setting was not preserved after merge")
	}

	// Check providers were added.
	if _, ok := result["providers"]; !ok {
		t.Fatal("providers not found in merged result")
	}

	// Check multica_target_version was added.
	if _, ok := result["multica_target_version"]; !ok {
		t.Fatal("multica_target_version not found in merged result")
	}
}

func TestMergeProviderSettingsNilExisting(t *testing.T) {
	t.Parallel()

	ps := WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"codex": {Enabled: true},
		},
	}

	merged, err := mergeProviderSettingsIntoWorkspace(nil, ps)
	if err != nil {
		t.Fatalf("merge with nil existing failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(merged, &result); err != nil {
		t.Fatalf("unmarshal merged result: %v", err)
	}

	if _, ok := result["providers"]; !ok {
		t.Fatal("providers not found")
	}
}

func TestRedactProviderSettings(t *testing.T) {
	t.Parallel()

	ps := WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"claude": {Enabled: true, APIKey: "sk-ant-api03-abcdefghij", TargetVersion: "2.0.0"},
			"codex":  {Enabled: false, APIKey: "", TargetVersion: ""},
		},
		MulticaTargetVersion: "0.1.15",
	}

	redacted := redactProviderSettings(ps)

	if redacted.MulticaTargetVersion != "0.1.15" {
		t.Fatalf("multica version not preserved: %q", redacted.MulticaTargetVersion)
	}

	claude := redacted.Providers["claude"]
	if !claude.Enabled {
		t.Fatal("claude should be enabled")
	}
	if claude.TargetVersion != "2.0.0" {
		t.Fatalf("target version not preserved: %q", claude.TargetVersion)
	}
	// API key should be masked.
	if claude.APIKey == "sk-ant-api03-abcdefghij" {
		t.Fatal("API key was not redacted")
	}
	if len(claude.APIKey) == 0 {
		t.Fatal("redacted API key should not be empty")
	}

	codex := redacted.Providers["codex"]
	if codex.APIKey != "" {
		t.Fatalf("empty API key should stay empty after redaction, got %q", codex.APIKey)
	}
}
