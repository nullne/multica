package handler

import (
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
