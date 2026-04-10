package daemon

import (
	"testing"
)

func TestInstallCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider      string
		targetVersion string
		wantBin       string
		wantNil       bool
	}{
		{"claude", "", "npm", false},
		{"claude", "2.1.0", "npm", false},
		{"codex", "", "npm", false},
		{"codex", "0.5.0", "npm", false},
		{"opencode", "", "npm", false},
		{"cursor", "", "", true},     // cursor not supported for auto-install
		{"unknown", "", "", true},    // unknown provider
	}
	for _, tc := range tests {
		bin, args := installCommand(tc.provider, tc.targetVersion)
		if tc.wantNil {
			if bin != "" {
				t.Errorf("installCommand(%q, %q): expected empty bin, got %q", tc.provider, tc.targetVersion, bin)
			}
			continue
		}
		if bin != tc.wantBin {
			t.Errorf("installCommand(%q, %q): expected bin %q, got %q", tc.provider, tc.targetVersion, tc.wantBin, bin)
		}
		if len(args) == 0 {
			t.Errorf("installCommand(%q, %q): expected non-empty args", tc.provider, tc.targetVersion)
		}
	}
}

func TestInstallCommandVersionInPackage(t *testing.T) {
	t.Parallel()

	// When a version is specified, it should appear in the package name.
	_, args := installCommand("claude", "2.1.0")
	found := false
	for _, arg := range args {
		if arg == "@anthropic-ai/claude-code@2.1.0" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected '@anthropic-ai/claude-code@2.1.0' in args, got %v", args)
	}

	// Without version, package name should not have @version suffix.
	_, args = installCommand("claude", "")
	for _, arg := range args {
		if arg == "@anthropic-ai/claude-code" {
			return // OK
		}
	}
	t.Errorf("expected '@anthropic-ai/claude-code' (no version) in args, got %v", args)
}

func TestProviderBinaryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider string
		want     string
	}{
		{"claude", "claude"},
		{"codex", "codex"},
		{"opencode", "opencode"},
		{"cursor", "cursor-agent"},
		{"unknown", "unknown"},
	}
	for _, tc := range tests {
		got := providerBinaryName(tc.provider)
		if got != tc.want {
			t.Errorf("providerBinaryName(%q) = %q, want %q", tc.provider, got, tc.want)
		}
	}
}
