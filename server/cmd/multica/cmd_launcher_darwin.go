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
	Short: "Manage macOS native project picker app (macOS only)",
}

var launcherInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install a native macOS app that shows a project picker dialog",
	RunE:  runLauncherInstall,
}

var launcherUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the macOS launcher app",
	RunE:  runLauncherUninstall,
}

func init() {
	launcherInstallCmd.Flags().String("editor", "", "Editor to open projects in (passed to 'multica open --editor')")
	launcherInstallCmd.Flags().String("shortcut", "⌃⌥M", "Suggested keyboard shortcut (displayed in instructions)")

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
	shortcut, _ := cmd.Flags().GetString("shortcut")

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

	baseCmd := multicaPath + " open" + profileFlag
	listCmd := baseCmd + " --list"
	openCmd := baseCmd + editorFlag + " --project"

	// AppleScript that:
	// 1. Calls `multica open --list` to get project names
	// 2. Shows a native macOS "choose from list" dialog
	// 3. Calls `multica open --project <selection>` to clone + open in editor
	appleScript := fmt.Sprintf(`#!/usr/bin/osascript
on run
	try
		set projectText to do shell script "%s"
	on error errMsg
		display dialog "Failed to fetch projects:" & return & return & errMsg with title "Multica" buttons {"OK"} default button "OK" with icon stop
		return
	end try

	if projectText is "" then
		display dialog "No projects found." & return & return & "Add repos to your workspace settings in the web app." with title "Multica" buttons {"OK"} default button "OK" with icon note
		return
	end if

	set oldDelims to AppleScript's text item delimiters
	set AppleScript's text item delimiters to linefeed
	set projectList to text items of projectText
	set AppleScript's text item delimiters to oldDelims

	set selectedProject to choose from list projectList with title "Multica — Open Project" with prompt "Select a project to open in your editor:" OK button name "Open" cancel button name "Cancel"

	if selectedProject is false then return

	set chosenProject to item 1 of selectedProject

	try
		do shell script "%s " & quoted form of chosenProject & " > /dev/null 2>&1 &"
	on error errMsg
		display dialog "Failed to open project:" & return & return & errMsg with title "Multica" buttons {"OK"} default button "OK" with icon stop
	end try
end run
`, listCmd, openCmd)

	// Create .app bundle
	appDir, err := applicationsDir()
	if err != nil {
		return err
	}

	// Remove old version if present
	appPath := filepath.Join(appDir, launchAppName)
	os.RemoveAll(appPath)

	scriptDir := filepath.Join(appPath, "Contents", "MacOS")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		return fmt.Errorf("create app bundle: %w", err)
	}

	scriptPath := filepath.Join(scriptDir, "launcher")
	if err := os.WriteFile(scriptPath, []byte(appleScript), 0o755); err != nil {
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

	contentsDir := filepath.Join(appPath, "Contents")
	if err := os.WriteFile(filepath.Join(contentsDir, "Info.plist"), []byte(appInfoPlist), 0o644); err != nil {
		return fmt.Errorf("write app Info.plist: %w", err)
	}

	fmt.Fprintln(os.Stderr, "✓ Installed native launcher: "+appPath)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "How to use:")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  • Spotlight (⌘Space) → type \"Multica Open\" → Enter")
	fmt.Fprintln(os.Stderr, "  • Raycast / Alfred → search \"Multica Open\"")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "To assign a global keyboard shortcut (e.g. "+shortcut+"):")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  1. Open Automator → File → New → Quick Action")
	fmt.Fprintln(os.Stderr, "  2. Add \"Launch Application\" → select \"Multica Open\" from ~/Applications")
	fmt.Fprintln(os.Stderr, "  3. Save as \"Multica Open\"")
	fmt.Fprintln(os.Stderr, "  4. System Settings → Keyboard → Keyboard Shortcuts → Services")
	fmt.Fprintln(os.Stderr, "     → find \"Multica Open\" → assign your shortcut")

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
