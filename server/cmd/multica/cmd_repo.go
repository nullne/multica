package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
}

var repoCheckoutCmd = &cobra.Command{
	Use:   "checkout <url>",
	Short: "Check out a repository into the working directory",
	Long:  "Creates a git worktree from the daemon's bare clone cache. Used by agents to check out repos on demand.",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoCheckout,
}

func init() {
	repoCmd.AddCommand(repoCheckoutCmd)
}

func runRepoCheckout(cmd *cobra.Command, args []string) error {
	repoURL := args[0]

	daemonPort := os.Getenv("MULTICA_DAEMON_PORT")
	if daemonPort == "" {
		return fmt.Errorf("MULTICA_DAEMON_PORT not set (this command is intended to be run by an agent inside a daemon task)")
	}

	workspaceID := os.Getenv("MULTICA_WORKSPACE_ID")
	agentName := os.Getenv("MULTICA_AGENT_NAME")
	taskID := os.Getenv("MULTICA_TASK_ID")
	if err := validateRepoCheckoutContext(workspaceID, agentName, taskID); err != nil {
		return err
	}

	// Use current working directory as the checkout target.
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	reqBody := map[string]string{
		"url":          repoURL,
		"workspace_id": workspaceID,
		"workdir":      workDir,
		"agent_name":   agentName,
		"task_id":      taskID,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(
		fmt.Sprintf("http://127.0.0.1:%s/repo/checkout", daemonPort),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checkout failed: %s", string(body))
	}

	var result struct {
		Path       string `json:"path"`
		BranchName string `json:"branch_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Fprintf(os.Stdout, "%s\n", result.Path)
	fmt.Fprintf(os.Stderr, "Checked out %s → %s (branch: %s)\n", repoURL, result.Path, result.BranchName)

	return nil
}

func validateRepoCheckoutContext(workspaceID, agentName, taskID string) error {
	missing := make([]string, 0, 3)
	if workspaceID == "" {
		missing = append(missing, "MULTICA_WORKSPACE_ID")
	}
	if agentName == "" {
		missing = append(missing, "MULTICA_AGENT_NAME")
	}
	if taskID == "" {
		missing = append(missing, "MULTICA_TASK_ID")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"%s not set (this command must run inside a daemon task; setting only MULTICA_DAEMON_PORT is not enough)",
		strings.Join(missing, ", "),
	)
}
