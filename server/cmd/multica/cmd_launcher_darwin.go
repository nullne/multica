//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var launcherCmd = &cobra.Command{
	Use:   "launcher",
	Short: "Manage macOS global shortcut for the project picker (macOS only)",
}

var launcherInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install a macOS Quick Action for 'multica open' with a global keyboard shortcut",
	RunE:  runLauncherInstall,
}

var launcherUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the macOS Quick Action",
	RunE:  runLauncherUninstall,
}

func init() {
	launcherInstallCmd.Flags().String("editor", "", "Editor to open projects in (passed to 'multica open --editor')")
	launcherInstallCmd.Flags().String("shortcut", "⌃⌥M", "Suggested keyboard shortcut (displayed in instructions)")

	launcherCmd.AddCommand(launcherInstallCmd)
	launcherCmd.AddCommand(launcherUninstallCmd)
}

const (
	serviceName   = "Multica Open Project"
	workflowDir   = "Multica Open Project.workflow"
	launchAppName = "Multica Open.app"
)

func servicesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Services"), nil
}

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

	profile := resolveProfile(cmd)
	editorFlag := ""
	if editor != "" {
		editorFlag = fmt.Sprintf(" --editor %s", editor)
	}
	profileFlag := ""
	if profile != "" {
		profileFlag = fmt.Sprintf(" --profile %s", profile)
	}

	// --- 1. Create Automator Quick Action (.workflow) ---
	svcDir, err := servicesDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		return fmt.Errorf("create Services dir: %w", err)
	}

	wfPath := filepath.Join(svcDir, workflowDir)
	wfContents := filepath.Join(wfPath, "Contents")
	if err := os.MkdirAll(wfContents, 0o755); err != nil {
		return fmt.Errorf("create workflow dir: %w", err)
	}

	openCommand := fmt.Sprintf("%s open%s%s", multicaPath, profileFlag, editorFlag)

	infoPlist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>NSServices</key>
	<array>
		<dict>
			<key>NSMenuItem</key>
			<dict>
				<key>default</key>
				<string>%s</string>
			</dict>
			<key>NSMessage</key>
			<string>runWorkflowAsService</string>
		</dict>
	</array>
</dict>
</plist>`, serviceName)

	documentWflow := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>AMApplicationBuild</key>
	<string>523</string>
	<key>AMApplicationVersion</key>
	<string>2.10</string>
	<key>AMDocumentVersion</key>
	<string>2</string>
	<key>actions</key>
	<array>
		<dict>
			<key>action</key>
			<dict>
				<key>AMAccepts</key>
				<dict>
					<key>Container</key>
					<string>List</string>
					<key>Optional</key>
					<true/>
					<key>Type</key>
					<string>0</string>
				</dict>
				<key>AMActionVersion</key>
				<string>1.0.2</string>
				<key>AMApplication</key>
				<array>
					<string>Automator</string>
				</array>
				<key>AMLargeIcon</key>
				<false/>
				<key>AMParameterProperties</key>
				<dict>
					<key>COMMAND_STRING</key>
					<dict/>
					<key>CheckedForUserDefaultShell</key>
					<dict/>
					<key>inputMethod</key>
					<dict/>
					<key>shell</key>
					<dict/>
					<key>source</key>
					<dict/>
				</dict>
				<key>AMProvides</key>
				<dict>
					<key>Container</key>
					<string>List</string>
					<key>Type</key>
					<string>0</string>
				</dict>
				<key>ActionBundlePath</key>
				<string>/System/Library/Automator/Run Shell Script.action</string>
				<key>ActionName</key>
				<string>Run Shell Script</string>
				<key>ActionParameters</key>
				<dict>
					<key>COMMAND_STRING</key>
					<string>%s</string>
					<key>CheckedForUserDefaultShell</key>
					<true/>
					<key>inputMethod</key>
					<integer>1</integer>
					<key>shell</key>
					<string>/bin/zsh</string>
					<key>source</key>
					<string></string>
				</dict>
				<key>BundleIdentifier</key>
				<string>com.apple.RunShellScript</string>
				<key>CFBundleVersion</key>
				<string>1.0.2</string>
				<key>CanShowSelectedItemsWhenRun</key>
				<false/>
				<key>CanShowWhenRun</key>
				<true/>
				<key>Category</key>
				<array>
					<string>AMCategoryUtilities</string>
				</array>
				<key>Class Name</key>
				<string>RunShellScriptAction</string>
				<key>InputUUID</key>
				<string>A1A1A1A1-B2B2-C3C3-D4D4-E5E5E5E5E5E5</string>
				<key>Keywords</key>
				<array>
					<string>Shell</string>
					<string>Script</string>
					<string>Command</string>
					<string>Run</string>
					<string>Unix</string>
				</array>
				<key>OutputUUID</key>
				<string>F6F6F6F6-A7A7-B8B8-C9C9-D0D0D0D0D0D0</string>
				<key>UUID</key>
				<string>12345678-AAAA-BBBB-CCCC-DDDDDDDDDDDD</string>
				<key>UnlocalizedApplications</key>
				<array>
					<string>Automator</string>
				</array>
			</dict>
		</dict>
	</array>
	<key>connectors</key>
	<dict/>
	<key>workflowMetaData</key>
	<dict>
		<key>workflowTypeIdentifier</key>
		<string>com.apple.Automator.servicesMenu</string>
	</dict>
</dict>
</plist>`, openCommand)

	if err := os.WriteFile(filepath.Join(wfContents, "Info.plist"), []byte(infoPlist), 0o644); err != nil {
		return fmt.Errorf("write Info.plist: %w", err)
	}
	if err := os.WriteFile(filepath.Join(wfContents, "document.wflow"), []byte(documentWflow), 0o644); err != nil {
		return fmt.Errorf("write document.wflow: %w", err)
	}

	// --- 2. Create a standalone .app launcher (opens Terminal + multica open) ---
	appDir, err := applicationsDir()
	if err != nil {
		return err
	}
	appPath := filepath.Join(appDir, launchAppName)
	appScript := fmt.Sprintf(`#!/bin/bash
osascript -e '
tell application "Terminal"
    activate
    do script "%s"
end tell'
`, strings.ReplaceAll(openCommand, `"`, `\"`))

	scriptDir := filepath.Join(appPath, "Contents", "MacOS")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		return fmt.Errorf("create app bundle: %w", err)
	}

	scriptPath := filepath.Join(scriptDir, "launcher")
	if err := os.WriteFile(scriptPath, []byte(appScript), 0o755); err != nil {
		return fmt.Errorf("write launcher script: %w", err)
	}

	appInfoPlist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>launcher</string>
	<key>CFBundleIdentifier</key>
	<string>com.multica.open</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundleVersion</key>
	<string>1.0</string>
	<key>LSUIElement</key>
	<true/>
</dict>
</plist>`, serviceName)

	contentsDir := filepath.Join(appPath, "Contents")
	if err := os.WriteFile(filepath.Join(contentsDir, "Info.plist"), []byte(appInfoPlist), 0o644); err != nil {
		return fmt.Errorf("write app Info.plist: %w", err)
	}

	fmt.Fprintln(os.Stderr, "✓ Installed Quick Action: "+wfPath)
	fmt.Fprintln(os.Stderr, "✓ Installed launcher app: "+appPath)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "To assign a global keyboard shortcut (e.g. "+shortcut+"):")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  1. Open System Settings → Keyboard → Keyboard Shortcuts → Services")
	fmt.Fprintln(os.Stderr, "  2. Find \""+serviceName+"\" under General")
	fmt.Fprintln(os.Stderr, "  3. Click \"Add Shortcut\" and press your preferred key combination")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Or use Spotlight / Raycast / Alfred to launch \"Multica Open\".")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "You can also run 'multica open' directly from any terminal.")

	return nil
}

func runLauncherUninstall(_ *cobra.Command, _ []string) error {
	removed := 0

	svcDir, err := servicesDir()
	if err == nil {
		wfPath := filepath.Join(svcDir, workflowDir)
		if err := os.RemoveAll(wfPath); err == nil {
			fmt.Fprintln(os.Stderr, "✓ Removed Quick Action: "+wfPath)
			removed++
		}
	}

	appDir, err := applicationsDir()
	if err == nil {
		appPath := filepath.Join(appDir, launchAppName)
		if err := os.RemoveAll(appPath); err == nil {
			fmt.Fprintln(os.Stderr, "✓ Removed launcher app: "+appPath)
			removed++
		}
	}

	if removed == 0 {
		fmt.Fprintln(os.Stderr, "Nothing to remove — launcher is not installed.")
	} else {
		fmt.Fprintln(os.Stderr, "\nIf you assigned a keyboard shortcut, remove it in:")
		fmt.Fprintln(os.Stderr, "  System Settings → Keyboard → Keyboard Shortcuts → Services")
	}

	return nil
}
