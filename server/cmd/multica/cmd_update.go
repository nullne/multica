package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nullne/multica/server/internal/cli"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update multica to the latest version",
	RunE:  runUpdate,
}

func runUpdate(_ *cobra.Command, _ []string) error {
	fmt.Fprintf(os.Stderr, "Current version: %s (commit: %s)\n", version, commit)

	// Check latest version from GitHub.
	cliUpToDate := false
	latest, err := cli.FetchLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not check latest version: %v\n", err)
	} else {
		latestVer := strings.TrimPrefix(latest.TagName, "v")
		currentVer := strings.TrimPrefix(version, "v")
		if currentVer == latestVer {
			fmt.Fprintln(os.Stderr, "Already up to date.")
			cliUpToDate = true
		}
		if !cliUpToDate {
			fmt.Fprintf(os.Stderr, "Latest version:  %s\n\n", latest.TagName)
		}
	}

	if !cliUpToDate {
		// Detect installation method and update accordingly.
		if cli.IsBrewInstall() {
			fmt.Fprintln(os.Stderr, "Updating via Homebrew...")
			output, err := cli.UpdateViaBrew()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", output)
				return fmt.Errorf("brew upgrade failed: %w\nYou can try manually: brew upgrade nullne/tap/multica", err)
			}
			fmt.Fprintln(os.Stderr, "CLI update complete.")
		} else {
			// Not installed via brew — download binary directly from GitHub Releases.
			if latest == nil {
				return fmt.Errorf("could not determine latest version; check https://github.com/nullne/multica/releases/latest")
			}
			targetVersion := latest.TagName
			fmt.Fprintf(os.Stderr, "Downloading %s from GitHub Releases...\n", targetVersion)
			output, err := cli.UpdateViaDownload(targetVersion)
			if err != nil {
				return fmt.Errorf("update failed: %w", err)
			}
			fmt.Fprintf(os.Stderr, "%s\nCLI update complete.\n", output)
		}
	}

	fmt.Fprintln(os.Stderr, "\nUpdating managed agent clients (claude, codex, cursor)...")
	clientReport := cli.UpdateManagedAgentClients()
	for _, result := range clientReport.Results {
		switch {
		case result.Err != nil:
			fmt.Fprintf(os.Stderr, "- %s: failed (%v)\n", result.Name, result.Err)
			if result.Output != "" {
				fmt.Fprintf(os.Stderr, "  output: %s\n", result.Output)
			}
		default:
			fmt.Fprintf(os.Stderr, "- %s: %s\n", result.Name, result.Status)
		}
	}
	if clientReport.HasFailures() {
		fmt.Fprintln(os.Stderr, "Some client updates failed. You can rerun `multica update` after fixing local prerequisites (npm/curl/PATH).")
	}
	fmt.Fprintln(os.Stderr, "Update workflow complete.")
	return nil
}
