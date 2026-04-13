package agent

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

const (
	AuthStatusNotInstalled    = "not_installed"
	AuthStatusUnauthenticated = "unauthenticated"
	AuthStatusReady           = "ready"
)

// CheckAuth probes a provider's authentication state.
// Returns one of: AuthStatusNotInstalled, AuthStatusUnauthenticated, AuthStatusReady.
func CheckAuth(ctx context.Context, agentType, executablePath string) string {
	if _, err := exec.LookPath(executablePath); err != nil {
		return AuthStatusNotInstalled
	}

	switch agentType {
	case "claude":
		return checkClaudeAuth(ctx, executablePath)
	case "codex":
		return checkCodexAuth(ctx, executablePath)
	case "opencode":
		return checkOpencodeAuth(ctx, executablePath)
	case "cursor":
		return checkCursorAuth(ctx, executablePath)
	default:
		return AuthStatusUnauthenticated
	}
}

// checkClaudeAuth runs `claude auth status` and parses the JSON output.
// Authenticated output: {"loggedIn": true, "email": "...", ...}
// Unauthenticated: {"loggedIn": false, ...} or error text.
func checkClaudeAuth(ctx context.Context, execPath string) string {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, execPath, "auth", "status")
	out, _ := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	var parsed struct {
		LoggedIn bool `json:"loggedIn"`
	}
	if json.Unmarshal([]byte(output), &parsed) == nil && parsed.LoggedIn {
		return AuthStatusReady
	}
	return AuthStatusUnauthenticated
}

// checkCodexAuth runs `codex login status` and checks the output text.
// Authenticated: "Logged in using ..."
// Unauthenticated: "Not logged in"
func checkCodexAuth(ctx context.Context, execPath string) string {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, execPath, "login", "status")
	out, _ := cmd.CombinedOutput()
	output := strings.ToLower(strings.TrimSpace(string(out)))

	if strings.Contains(output, "logged in") && !strings.Contains(output, "not logged in") {
		return AuthStatusReady
	}
	return AuthStatusUnauthenticated
}

// checkOpencodeAuth runs `opencode login status` and checks the output.
// Falls back to unauthenticated if the command doesn't support auth checking.
func checkOpencodeAuth(ctx context.Context, execPath string) string {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, execPath, "login", "status")
	out, _ := cmd.CombinedOutput()
	output := strings.ToLower(strings.TrimSpace(string(out)))

	if strings.Contains(output, "logged in") && !strings.Contains(output, "not logged in") {
		return AuthStatusReady
	}
	return AuthStatusUnauthenticated
}

// checkCursorAuth runs `cursor-agent status` and checks the output.
// Authenticated: "✓ Logged in as ..."
// Unauthenticated: "Not logged in" or similar.
func checkCursorAuth(ctx context.Context, execPath string) string {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, execPath, "status")
	out, _ := cmd.CombinedOutput()
	output := strings.ToLower(strings.TrimSpace(string(out)))

	if strings.Contains(output, "logged in") && !strings.Contains(output, "not logged in") {
		return AuthStatusReady
	}
	return AuthStatusUnauthenticated
}
