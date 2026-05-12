package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/nullne/multica/server/internal/cli"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces you belong to",
	RunE:  runWorkspaceList,
}

var workspaceGetCmd = &cobra.Command{
	Use:   "get [workspace-id]",
	Short: "Get workspace details",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorkspaceGet,
}

var workspaceMembersCmd = &cobra.Command{
	Use:   "members [workspace-id]",
	Short: "List workspace members",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorkspaceMembers,
}

var workspaceWatchCmd = &cobra.Command{
	Use:   "watch <workspace-id>",
	Short: "Enable this user's local daemon for a workspace",
	Long:  "Enables the local daemon owned by the authenticated user for the given workspace. Assignments are stored on the server (daemon_workspace).",
	Args:  cobra.ExactArgs(1),
	RunE:  runWatch,
}

var workspaceUnwatchCmd = &cobra.Command{
	Use:   "unwatch <workspace-id>",
	Short: "Disable this user's local daemon for a workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runUnwatch,
}

func init() {
	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceGetCmd)
	workspaceCmd.AddCommand(workspaceMembersCmd)
	workspaceCmd.AddCommand(workspaceWatchCmd)
	workspaceCmd.AddCommand(workspaceUnwatchCmd)

	workspaceGetCmd.Flags().String("output", "json", "Output format: table or json")
	workspaceMembersCmd.Flags().String("output", "table", "Output format: table or json")
}

func runWorkspaceList(cmd *cobra.Command, _ []string) error {
	serverURL := resolveServerURL(cmd)
	token := resolveToken(cmd)
	if token == "" {
		return fmt.Errorf("not authenticated: run 'multica login' first")
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
		fmt.Fprintln(os.Stderr, "No workspaces found.")
		return nil
	}

	// Mark workspaces this user's local daemon is enabled for, by
	// looking up the daemon owned by the user (if any).
	enabled := loadEnabledWorkspaceIDs(ctx, client, cmd)

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDAEMON ENABLED")
	for _, ws := range workspaces {
		mark := ""
		if enabled[ws.ID] {
			mark = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", ws.ID, ws.Name, mark)
	}
	return w.Flush()
}

// loadEnabledWorkspaceIDs returns the set of workspace IDs the local daemon
// is currently enabled for. Resolves the daemon by matching its daemon_id
// string (defaults to the hostname) against `/api/me/daemons`. If no daemon
// matches, the returned set is empty.
func loadEnabledWorkspaceIDs(ctx context.Context, client *cli.APIClient, cmd *cobra.Command) map[string]bool {
	enabled := make(map[string]bool)
	daemonUUID := findLocalDaemonUUID(ctx, client, cmd)
	if daemonUUID == "" {
		return enabled
	}
	var assignments []struct {
		WorkspaceID string `json:"workspace_id"`
		Enabled     bool   `json:"enabled"`
	}
	if err := client.GetJSON(ctx, "/api/me/daemons/"+daemonUUID+"/workspaces", &assignments); err != nil {
		return enabled
	}
	for _, a := range assignments {
		if a.Enabled {
			enabled[a.WorkspaceID] = true
		}
	}
	return enabled
}

func workspaceIDFromArgs(cmd *cobra.Command, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return resolveWorkspaceID(cmd)
}

func runWorkspaceGet(cmd *cobra.Command, args []string) error {
	wsID := workspaceIDFromArgs(cmd, args)
	if wsID == "" {
		return fmt.Errorf("workspace ID is required: pass as argument or set MULTICA_WORKSPACE_ID")
	}

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var ws map[string]any
	if err := client.GetJSON(ctx, "/api/workspaces/"+wsID, &ws); err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		desc := strVal(ws, "description")
		if utf8.RuneCountInString(desc) > 60 {
			runes := []rune(desc)
			desc = string(runes[:57]) + "..."
		}
		wsContext := strVal(ws, "context")
		if utf8.RuneCountInString(wsContext) > 60 {
			runes := []rune(wsContext)
			wsContext = string(runes[:57]) + "..."
		}
		headers := []string{"ID", "NAME", "SLUG", "DESCRIPTION", "CONTEXT"}
		rows := [][]string{{
			strVal(ws, "id"),
			strVal(ws, "name"),
			strVal(ws, "slug"),
			desc,
			wsContext,
		}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}

	return cli.PrintJSON(os.Stdout, ws)
}

func runWorkspaceMembers(cmd *cobra.Command, args []string) error {
	wsID := workspaceIDFromArgs(cmd, args)
	if wsID == "" {
		return fmt.Errorf("workspace ID is required: pass as argument or set MULTICA_WORKSPACE_ID")
	}

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var members []map[string]any
	if err := client.GetJSON(ctx, "/api/workspaces/"+wsID+"/members", &members); err != nil {
		return fmt.Errorf("list members: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, members)
	}

	headers := []string{"USER ID", "NAME", "EMAIL", "ROLE"}
	rows := make([][]string, 0, len(members))
	for _, m := range members {
		rows = append(rows, []string{
			strVal(m, "user_id"),
			strVal(m, "name"),
			strVal(m, "email"),
			strVal(m, "role"),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runWatch(cmd *cobra.Command, args []string) error {
	return setDaemonWorkspaceEnabled(cmd, args[0], true)
}

func runUnwatch(cmd *cobra.Command, args []string) error {
	return setDaemonWorkspaceEnabled(cmd, args[0], false)
}

// setDaemonWorkspaceEnabled enables or disables the user's local daemon for a
// workspace by calling the server-side assignment API. The daemon must be
// registered before this works — assignments live on the server now.
func setDaemonWorkspaceEnabled(cmd *cobra.Command, workspaceID string, enabled bool) error {
	serverURL := resolveServerURL(cmd)
	token := resolveToken(cmd)
	if token == "" {
		return fmt.Errorf("not authenticated: run 'multica login' first")
	}

	client := cli.NewAPIClient(serverURL, "", token)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var ws struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := client.GetJSON(ctx, "/api/workspaces/"+workspaceID, &ws); err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	daemonUUID := findLocalDaemonUUID(ctx, client, cmd)
	if daemonUUID == "" {
		return fmt.Errorf("no daemon registered for this user yet — run 'multica daemon start' first")
	}

	if err := client.PutJSON(ctx, "/api/me/daemons/"+daemonUUID+"/workspaces/"+ws.ID, map[string]any{
		"enabled": enabled,
	}, nil); err != nil {
		return fmt.Errorf("update assignment: %w", err)
	}

	// Make this the default workspace on first enable for convenience.
	if enabled {
		profile := resolveProfile(cmd)
		if cfg, err := cli.LoadCLIConfigForProfile(profile); err == nil && cfg.WorkspaceID == "" {
			cfg.WorkspaceID = ws.ID
			cli.SaveCLIConfigForProfile(cfg, profile)
			fmt.Fprintf(os.Stderr, "Set default workspace to %s (%s)\n", ws.ID, ws.Name)
		}
		fmt.Fprintf(os.Stderr, "Watching workspace %s (%s)\n", ws.ID, ws.Name)
	} else {
		fmt.Fprintf(os.Stderr, "Stopped watching workspace %s (%s)\n", ws.ID, ws.Name)
	}
	return nil
}

// findLocalDaemonUUID looks up the UUID of the daemon owned by the current
// user whose daemon_id string matches the local hostname (the default
// MULTICA_DAEMON_ID). Returns empty if no match.
func findLocalDaemonUUID(ctx context.Context, client *cli.APIClient, cmd *cobra.Command) string {
	hostname, _ := os.Hostname()
	if v := os.Getenv("MULTICA_DAEMON_ID"); v != "" {
		hostname = v
	}
	if v, err := cmd.Flags().GetString("daemon-id"); err == nil && v != "" {
		hostname = v
	}
	if hostname == "" {
		return ""
	}
	var daemons []struct {
		ID       string `json:"id"`
		DaemonID string `json:"daemon_id"`
	}
	if err := client.GetJSON(ctx, "/api/me/daemons", &daemons); err != nil {
		return ""
	}
	for _, d := range daemons {
		if d.DaemonID == hostname {
			return d.ID
		}
	}
	// Fall back to the first daemon owned by this user.
	if len(daemons) > 0 {
		return daemons[0].ID
	}
	return ""
}
