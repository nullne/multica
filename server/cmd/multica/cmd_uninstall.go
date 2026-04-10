package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/nullne/multica/server/internal/cli"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Multica CLI and remove all local data",
	Long: `Remove the Multica CLI binary, configuration, daemon state, and optionally workspace data.

This command will:
  1. Stop the daemon if running
  2. Remove the macOS launcher app (if installed)
  3. Remove configuration and daemon state (~/.multica/)
  4. Remove the CLI binary

Use --include-workspaces to also remove cloned repositories under ~/multica_workspaces/.`,
	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	uninstallCmd.Flags().Bool("include-workspaces", false, "Also remove workspace directories (~/multica_workspaces/)")
}

type uninstallItem struct {
	path  string
	label string
}

func runUninstall(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	includeWorkspaces, _ := cmd.Flags().GetBool("include-workspaces")
	profile := resolveProfile(cmd)

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	var items []uninstallItem

	configDir := filepath.Join(home, ".multica")
	if profile != "" {
		configDir = filepath.Join(home, ".multica", "profiles", profile)
	}
	if dirExists(configDir) {
		items = append(items, uninstallItem{path: configDir, label: "Configuration & daemon state"})
	}

	if runtime.GOOS == "darwin" {
		launcherPath := filepath.Join(home, "Applications", "Multica Open.app")
		if dirExists(launcherPath) {
			items = append(items, uninstallItem{path: launcherPath, label: "macOS launcher app"})
		}
	}

	if includeWorkspaces {
		wsRoot := filepath.Join(home, "multica_workspaces")
		if profile != "" {
			wsRoot = filepath.Join(home, "multica_workspaces_"+profile)
		}
		if dirExists(wsRoot) {
			items = append(items, uninstallItem{path: wsRoot, label: "Workspace data"})
		}
	}

	isBrew := cli.IsBrewInstall()
	exePath, _ := os.Executable()
	if exePath != "" {
		if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = resolved
		}
	}

	fmt.Fprintln(os.Stderr, "Multica uninstall will perform the following actions:")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  1. Stop the daemon (if running)")
	n := 2
	for _, item := range items {
		fmt.Fprintf(os.Stderr, "  %d. Remove %s (%s)\n", n, item.label, item.path)
		n++
	}
	if isBrew {
		fmt.Fprintf(os.Stderr, "  %d. Uninstall via Homebrew (brew uninstall nullne/tap/multica)\n", n)
	} else if exePath != "" {
		fmt.Fprintf(os.Stderr, "  %d. Remove CLI binary (%s)\n", n, exePath)
	}
	fmt.Fprintln(os.Stderr)

	if !includeWorkspaces {
		wsRoot := filepath.Join(home, "multica_workspaces")
		if profile != "" {
			wsRoot = filepath.Join(home, "multica_workspaces_"+profile)
		}
		if dirExists(wsRoot) {
			fmt.Fprintf(os.Stderr, "  Note: Workspace data at %s will be kept.\n", wsRoot)
			fmt.Fprintln(os.Stderr, "        Use --include-workspaces to remove it.")
			fmt.Fprintln(os.Stderr)
		}
	}

	if !force {
		fmt.Fprint(os.Stderr, "Continue? [y/N] ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() || !isYes(scanner.Text()) {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
		fmt.Fprintln(os.Stderr)
	}

	fmt.Fprint(os.Stderr, "Stopping daemon... ")
	stopDaemonQuiet(profile)
	fmt.Fprintln(os.Stderr, "done")

	for _, item := range items {
		fmt.Fprintf(os.Stderr, "Removing %s... ", item.label)
		if err := os.RemoveAll(item.path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "done")
		}
	}

	if profile != "" {
		profilesDir := filepath.Join(home, ".multica", "profiles")
		cleanEmptyDir(profilesDir)
		cleanEmptyDir(filepath.Join(home, ".multica"))
	}

	if isBrew {
		fmt.Fprint(os.Stderr, "Uninstalling via Homebrew... ")
		out, err := exec.Command("brew", "uninstall", "nullne/tap/multica").CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed\n  %s\n  %v\n", strings.TrimSpace(string(out)), err)
		} else {
			fmt.Fprintln(os.Stderr, "done")
		}
	} else if exePath != "" {
		fmt.Fprintf(os.Stderr, "Removing CLI binary... ")
		if err := os.Remove(exePath); err != nil {
			if os.IsPermission(err) {
				fmt.Fprintf(os.Stderr, "permission denied\n")
				fmt.Fprintf(os.Stderr, "  Run manually: sudo rm %s\n", exePath)
			} else {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
		} else {
			fmt.Fprintln(os.Stderr, "done")
		}
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Multica has been uninstalled.")
	return nil
}

func stopDaemonQuiet(profile string) {
	healthPort := healthPortForProfile(profile)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	health := checkDaemonHealthOnPort(ctx, healthPort)
	if health["status"] != "running" {
		return
	}

	pid, ok := health["pid"].(float64)
	if !ok || pid == 0 {
		return
	}

	process, err := os.FindProcess(int(pid))
	if err != nil {
		return
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return
	}

	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
		h := checkDaemonHealthOnPort(ctx2, healthPort)
		cancel2()
		if h["status"] != "running" {
			return
		}
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isYes(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "y" || s == "yes"
}

func cleanEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		os.Remove(dir)
	}
}
