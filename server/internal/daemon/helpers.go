package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", key, value, err)
	}
	return d, nil
}

func intFromEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid integer %q: %w", key, value, err)
	}
	return n, nil
}

// inheritLoginEnv spawns a login shell to resolve the user's full PATH
// (including entries added by ~/.bash_profile, ~/.zprofile, etc.) and sets
// it in the current process. This ensures exec.LookPath and agent child
// processes see the same PATH the user expects from an interactive session.
//
// This is necessary because non-login contexts (systemd, cron, non-login SSH
// commands) don't source profile files, so PATH may be missing user-installed
// directories like /usr/local/bin.
//
// Only runs on macOS/Linux; no-op on other platforms.
func inheritLoginEnv() {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Use a login shell (-l) to source profile files, then print PATH.
	// Timeout guards against profiles that hang (e.g. interactive prompts,
	// slow network mounts).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-l", "-c", "echo $PATH")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		slog.Debug("inheritLoginEnv: failed to resolve login shell PATH", "shell", shell, "error", err)
		return
	}

	loginPath := strings.TrimSpace(string(out))
	if loginPath == "" {
		return
	}

	currentPath := os.Getenv("PATH")
	if loginPath == currentPath {
		return
	}

	// Merge: start with the login shell PATH, then append any entries from
	// the current PATH that aren't already present (e.g. Go-added dirs).
	merged := mergePathValues(loginPath, currentPath)
	os.Setenv("PATH", merged)
	slog.Info("inherited PATH from login shell", "shell", shell, "added_dirs", pathDiff(currentPath, merged))
}

// pathDiff returns directories present in updated but not in original.
func pathDiff(original, updated string) []string {
	orig := map[string]bool{}
	for _, p := range strings.Split(original, string(os.PathListSeparator)) {
		orig[p] = true
	}
	var added []string
	for _, p := range strings.Split(updated, string(os.PathListSeparator)) {
		if p != "" && !orig[p] {
			added = append(added, p)
		}
	}
	return added
}

// mergePathValues merges two colon-separated PATH strings. Entries from
// primary appear first; entries from secondary are appended if not already
// present.
func mergePathValues(primary, secondary string) string {
	seen := map[string]bool{}
	var parts []string
	for _, p := range strings.Split(primary, string(os.PathListSeparator)) {
		if p != "" && !seen[p] {
			seen[p] = true
			parts = append(parts, p)
		}
	}
	for _, p := range strings.Split(secondary, string(os.PathListSeparator)) {
		if p != "" && !seen[p] {
			seen[p] = true
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
