//go:build !darwin

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var launcherCmd = &cobra.Command{
	Use:   "launcher",
	Short: "Manage global shortcut for the project picker (macOS only)",
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("the launcher command is only available on macOS")
	},
}
