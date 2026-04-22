package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/nullne/multica/server/internal/cli"
	"github.com/nullne/multica/server/internal/daemon"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents in the workspace",
	RunE:  runAgentList,
}

func init() {
	agentCmd.AddCommand(agentListCmd)

	agentListCmd.Flags().String("output", "table", "Output format: table or json")
}

// resolveProfile returns the --profile flag value (empty string means default profile).
func resolveProfile(cmd *cobra.Command) string {
	val, _ := cmd.Flags().GetString("profile")
	return val
}

func newAPIClient(cmd *cobra.Command) (*cli.APIClient, error) {
	serverURL := resolveServerURL(cmd)
	workspaceID := resolveWorkspaceID(cmd)
	token := resolveToken(cmd)

	if serverURL == "" {
		return nil, fmt.Errorf("server URL not set: use --server-url flag, MULTICA_SERVER_URL env, or 'multica config set server_url <url>'")
	}

	client := cli.NewAPIClient(serverURL, workspaceID, token)
	// When running inside a daemon task, attribute actions to the agent.
	if agentID := os.Getenv("MULTICA_AGENT_ID"); agentID != "" {
		client.AgentID = agentID
	}
	if taskID := os.Getenv("MULTICA_TASK_ID"); taskID != "" {
		client.TaskID = taskID
	}
	return client, nil
}

func resolveServerURL(cmd *cobra.Command) string {
	val := cli.FlagOrEnv(cmd, "server-url", "MULTICA_SERVER_URL", "")
	if val != "" {
		return normalizeAPIBaseURL(val)
	}
	profile := resolveProfile(cmd)
	cfg, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return "https://multica.claw4us.com"
	}
	if cfg.ServerURL != "" {
		return normalizeAPIBaseURL(cfg.ServerURL)
	}
	return "https://multica.claw4us.com"
}

func normalizeAPIBaseURL(raw string) string {
	normalized, err := daemon.NormalizeServerBaseURL(raw)
	if err == nil {
		return normalized
	}
	return raw
}

func resolveWorkspaceID(cmd *cobra.Command) string {
	val := cli.FlagOrEnv(cmd, "workspace-id", "MULTICA_WORKSPACE_ID", "")
	if val != "" {
		return val
	}
	profile := resolveProfile(cmd)
	cfg, _ := cli.LoadCLIConfigForProfile(profile)
	return cfg.WorkspaceID
}

func runAgentList(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var agents []map[string]any
	path := "/api/agents"
	if client.WorkspaceID != "" {
		path += "?" + url.Values{"workspace_id": {client.WorkspaceID}}.Encode()
	}
	if err := client.GetJSON(ctx, path, &agents); err != nil {
		return fmt.Errorf("list agents: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, agents)
	}

	headers := []string{"ID", "NAME", "STATUS", "RUNTIME", "SKILLS"}
	rows := make([][]string, 0, len(agents))
	for _, a := range agents {
		rows = append(rows, []string{
			strVal(a, "id"),
			strVal(a, "name"),
			strVal(a, "status"),
			strVal(a, "runtime_mode"),
			formatSkillsSummary(a["skills"]),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

// formatSkillsSummary renders the agent's skills column for the list table as
// a comma-separated "name(idShort)" list, truncated to keep the row readable.
func formatSkillsSummary(raw any) string {
	skills, ok := raw.([]any)
	if !ok || len(skills) == 0 {
		return ""
	}

	parts := make([]string, 0, len(skills))
	for _, item := range skills {
		s, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strVal(s, "name")
		id := strVal(s, "id")
		switch {
		case name != "" && id != "":
			parts = append(parts, fmt.Sprintf("%s(%s)", name, truncateID(id)))
		case name != "":
			parts = append(parts, name)
		case id != "":
			parts = append(parts, truncateID(id))
		}
	}

	out := strings.Join(parts, ", ")
	if utf8.RuneCountInString(out) > 60 {
		runes := []rune(out)
		out = string(runes[:57]) + "..."
	}
	return out
}

func strVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
