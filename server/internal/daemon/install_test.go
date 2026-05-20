package daemon

import (
	"strings"
	"testing"

	"github.com/nullne/multica/server/internal/codeagent"
)

func TestInstallCommandUsesVerifiedVersion(t *testing.T) {
	t.Parallel()

	claudeVersion, _ := codeagent.Version("claude")
	bin, args, err := installCommand("claude", claudeVersion)
	if err != nil {
		t.Fatalf("installCommand: %v", err)
	}
	if bin != "npm" {
		t.Fatalf("expected npm, got %q", bin)
	}
	if got := strings.Join(args, " "); !strings.Contains(got, "@anthropic-ai/claude-code@"+claudeVersion) {
		t.Fatalf("expected claude install to pin %q, got %q", claudeVersion, got)
	}
}

func TestResolveInstallVersionUsesRepoDefault(t *testing.T) {
	t.Parallel()

	version, err := resolveInstallVersion("codex", "")
	if err != nil {
		t.Fatalf("resolveInstallVersion: %v", err)
	}
	want, _ := codeagent.Version("codex")
	if version != want {
		t.Fatalf("expected %q, got %q", want, version)
	}
}

func TestResolveInstallVersionRejectsOverride(t *testing.T) {
	t.Parallel()

	_, err := resolveInstallVersion("codex", "9.9.9")
	if err == nil {
		t.Fatal("expected error for non-repo version")
	}
}

func TestCursorInstallCommandPinsVerifiedVersion(t *testing.T) {
	t.Parallel()

	version, _ := codeagent.Version("cursor")
	bin, args, err := installCommand("cursor", version)
	if err != nil {
		t.Fatalf("installCommand: %v", err)
	}
	if bin != "bash" {
		t.Fatalf("expected bash, got %q", bin)
	}
	if got := strings.Join(args, " "); !strings.Contains(got, `version="`+version+`"`) {
		t.Fatalf("expected cursor install script to pin %q, got %q", version, got)
	}
}
