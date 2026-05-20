package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/nullne/multica/server/internal/codeagent"
)

// installCommand returns the shell command to install a code agent CLI.
// Versions are repository-owned; targetVersion is resolved before this is called.
func installCommand(provider, targetVersion string) (string, []string, error) {
	if targetVersion == "" {
		return "", nil, fmt.Errorf("no verified version configured for provider %q", provider)
	}

	switch provider {
	case "claude":
		return "npm", []string{"install", "-g", "@anthropic-ai/claude-code@" + targetVersion}, nil
	case "codex":
		return "npm", []string{"install", "-g", "@openai/codex@" + targetVersion}, nil
	case "opencode":
		return "npm", []string{"install", "-g", "opencode@" + targetVersion}, nil
	case "cursor":
		return "bash", []string{"-c", cursorInstallScript(targetVersion)}, nil
	default:
		return "", nil, fmt.Errorf("auto-install not supported for provider %q", provider)
	}
}

// InstallProvider installs a code agent CLI for the given provider.
// Returns the output and any error.
func InstallProvider(ctx context.Context, provider, targetVersion string, logger *slog.Logger) (string, error) {
	resolvedVersion, err := resolveInstallVersion(provider, targetVersion)
	if err != nil {
		return "", err
	}
	bin, args, err := installCommand(provider, resolvedVersion)
	if err != nil {
		return "", err
	}

	logger.Info("auto-installing code agent CLI", "provider", provider, "version", resolvedVersion, "command", fmt.Sprintf("%s %s", bin, strings.Join(args, " ")))

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		logger.Error("auto-install failed", "provider", provider, "error", err, "output", output)
		return output, fmt.Errorf("install %s failed: %w\n%s", provider, err, output)
	}

	logger.Info("auto-install completed", "provider", provider, "output", output)
	return output, nil
}

// UpdateProvider updates a code agent CLI to the given target version.
// This is the same as installing with a specific version.
func UpdateProvider(ctx context.Context, provider, targetVersion string, logger *slog.Logger) (string, error) {
	return InstallProvider(ctx, provider, targetVersion, logger)
}

func resolveInstallVersion(provider, requested string) (string, error) {
	version, ok := codeagent.Version(provider)
	if !ok {
		return "", fmt.Errorf("no verified version configured for provider %q", provider)
	}
	if requested != "" && requested != version {
		return "", fmt.Errorf("provider %q version is repository-owned: requested %q, verified %q", provider, requested, version)
	}
	return version, nil
}

func cursorInstallScript(version string) string {
	return fmt.Sprintf(`
set -euo pipefail
version=%q
case "$(uname -s)" in
  Linux*) os=linux ;;
  Darwin*) os=darwin ;;
  *) echo "unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=x64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac
base="$HOME/.local/share/cursor-agent/versions"
tmp="$base/.tmp-$version-$$"
final="$base/$version"
mkdir -p "$tmp" "$HOME/.local/bin"
trap 'rm -rf "$tmp"' EXIT
curl -fSL "https://downloads.cursor.com/lab/$version/$os/$arch/agent-cli-package.tar.gz" \
  | tar --strip-components=1 -xzf - -C "$tmp"
rm -rf "$final"
mv "$tmp" "$final"
ln -sf "$final/cursor-agent" "$HOME/.local/bin/agent"
ln -sf "$final/cursor-agent" "$HOME/.local/bin/cursor-agent"
`, version)
}
