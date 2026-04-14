package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// installCommand returns the shell command to install a code agent CLI.
// If targetVersion is empty, the latest version is installed.
func installCommand(provider, targetVersion string) (string, []string) {
	switch provider {
	case "claude":
		pkg := "@anthropic-ai/claude-code"
		if targetVersion != "" {
			pkg += "@" + targetVersion
		}
		return "npm", []string{"install", "-g", pkg}
	case "codex":
		pkg := "@openai/codex"
		if targetVersion != "" {
			pkg += "@" + targetVersion
		}
		return "npm", []string{"install", "-g", pkg}
	case "opencode":
		// opencode is typically installed via go install or binary download.
		// Attempt npm first as a common method.
		pkg := "opencode"
		if targetVersion != "" {
			pkg += "@" + targetVersion
		}
		return "npm", []string{"install", "-g", pkg}
	case "cursor":
		return "bash", []string{"-c", "curl https://cursor.com/install -fsS | bash"}
	default:
		return "", nil
	}
}

// InstallProvider installs a code agent CLI for the given provider.
// Returns the output and any error.
func InstallProvider(ctx context.Context, provider, targetVersion string, logger *slog.Logger) (string, error) {
	bin, args := installCommand(provider, targetVersion)
	if bin == "" {
		return "", fmt.Errorf("auto-install not supported for provider %q", provider)
	}

	logger.Info("auto-installing code agent CLI", "provider", provider, "version", targetVersion, "command", fmt.Sprintf("%s %s", bin, strings.Join(args, " ")))

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
