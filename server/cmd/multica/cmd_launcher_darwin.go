//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var launcherCmd = &cobra.Command{
	Use:   "launcher",
	Short: "Manage macOS launcher for the project picker (macOS only)",
}

var launcherInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install a macOS app that opens the terminal-based project picker",
	RunE:  runLauncherInstall,
}

var launcherUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the macOS launcher app",
	RunE:  runLauncherUninstall,
}

func init() {
	launcherInstallCmd.Flags().String("editor", "", "Editor to open projects in (passed to 'multica open --editor')")

	launcherCmd.AddCommand(launcherInstallCmd)
	launcherCmd.AddCommand(launcherUninstallCmd)
}

const launchAppName = "Multica Open.app"

func applicationsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Applications"), nil
}

func runLauncherInstall(cmd *cobra.Command, _ []string) error {
	editor, _ := cmd.Flags().GetString("editor")

	multicaPath, err := exec.LookPath("multica")
	if err != nil {
		multicaPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("cannot find multica binary: %w", err)
		}
	}
	multicaPath, _ = filepath.Abs(multicaPath)

	profile := resolveProfile(cmd)
	editorFlag := ""
	if editor != "" {
		editorFlag = " --editor " + editor
	}
	profileFlag := ""
	if profile != "" {
		profileFlag = " --profile " + profile
	}

	openCommand := multicaPath + " open" + profileFlag + editorFlag

	launcherScript := fmt.Sprintf(`#!/bin/bash
osascript -e '
tell application "Terminal"
    activate
    do script "%s"
end tell'
`, openCommand)

	appDir, err := applicationsDir()
	if err != nil {
		return err
	}

	appPath := filepath.Join(appDir, launchAppName)
	os.RemoveAll(appPath)

	scriptDir := filepath.Join(appPath, "Contents", "MacOS")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		return fmt.Errorf("create app bundle: %w", err)
	}

	if err := os.WriteFile(filepath.Join(scriptDir, "launcher"), []byte(launcherScript), 0o755); err != nil {
		return fmt.Errorf("write launcher script: %w", err)
	}

	appInfoPlist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>launcher</string>
	<key>CFBundleIdentifier</key>
	<string>com.multica.open</string>
	<key>CFBundleName</key>
	<string>Multica Open</string>
	<key>CFBundleVersion</key>
	<string>1.0</string>
	<key>LSUIElement</key>
	<true/>
</dict>
</plist>`

	if err := os.WriteFile(filepath.Join(appPath, "Contents", "Info.plist"), []byte(appInfoPlist), 0o644); err != nil {
		return fmt.Errorf("write Info.plist: %w", err)
	}

	fmt.Fprintln(os.Stderr, "✓ Installed launcher: "+appPath)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  • Spotlight (⌘Space) → \"Multica Open\" → Enter")
	fmt.Fprintln(os.Stderr, "  • Or run 'multica open' directly in any terminal")

	return nil
}

func runLauncherUninstall(_ *cobra.Command, _ []string) error {
	appDir, err := applicationsDir()
	if err != nil {
		return err
	}

	appPath := filepath.Join(appDir, launchAppName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Nothing to remove — launcher is not installed.")
		return nil
	}

	if err := os.RemoveAll(appPath); err != nil {
		return fmt.Errorf("remove app: %w", err)
	}

	fmt.Fprintln(os.Stderr, "✓ Removed launcher: "+appPath)
	return nil
}
