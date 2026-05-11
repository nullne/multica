package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nullne/multica/server/internal/cli"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate and set up workspaces",
	Long:  "Log in to Multica. Workspace assignments for your local daemon are managed server-side and applied automatically on 'multica daemon start'.",
	RunE:  runLogin,
}

func init() {
	loginCmd.Flags().Bool("token", false, "Authenticate by pasting a personal access token")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Run the standard auth login flow.
	if err := runAuthLogin(cmd, args); err != nil {
		return err
	}

	if err := setDefaultWorkspace(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "\nCould not set default workspace: %v\n", err)
		return nil
	}

	fmt.Fprintf(os.Stderr, "\n→ Run 'multica daemon start' to start your local agent runtime.\n")
	fmt.Fprintf(os.Stderr, "   On first start it will be auto-enabled for all your workspaces.\n")
	return nil
}

// setDefaultWorkspace picks the first workspace as the CLI's default if none
// is configured. Daemon-to-workspace assignments live on the server now and
// happen at daemon registration time.
func setDefaultWorkspace(cmd *cobra.Command) error {
	serverURL := resolveServerURL(cmd)
	token := resolveToken(cmd)
	if token == "" {
		return fmt.Errorf("not authenticated")
	}

	client := cli.NewAPIClient(serverURL, "", token)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var workspaces []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := client.GetJSON(ctx, "/api/workspaces", &workspaces); err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	if len(workspaces) == 0 {
		fmt.Fprintln(os.Stderr, "\nNo workspaces found.")
		return nil
	}

	profile := resolveProfile(cmd)
	cfg, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return err
	}

	if cfg.WorkspaceID == "" {
		cfg.WorkspaceID = workspaces[0].ID
	}

	if err := cli.SaveCLIConfigForProfile(cfg, profile); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\n%d workspace(s) available:\n", len(workspaces))
	for _, ws := range workspaces {
		fmt.Fprintf(os.Stderr, "  • %s (%s)\n", ws.Name, ws.ID)
	}
	return nil
}
