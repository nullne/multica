package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/nullne/multica/server/internal/cli"
)

var routineCmd = &cobra.Command{
	Use:     "routine",
	Aliases: []string{"routines"},
	Short:   "Manage routines",
}

var routineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List routines in the workspace",
	RunE:  runRoutineList,
}

var routineGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get routine details",
	Args:  cobra.ExactArgs(1),
	RunE:  runRoutineGet,
}

var routineCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a routine",
	RunE:  runRoutineCreate,
}

var routineUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a routine",
	Args:  cobra.ExactArgs(1),
	RunE:  runRoutineUpdate,
}

var routineDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a routine",
	Args:  cobra.ExactArgs(1),
	RunE:  runRoutineDelete,
}

var routineRunCmd = &cobra.Command{
	Use:   "run <id>",
	Short: "Run a routine now",
	Args:  cobra.ExactArgs(1),
	RunE:  runRoutineRun,
}

var routineRunsCmd = &cobra.Command{
	Use:   "runs <id>",
	Short: "List routine run history",
	Args:  cobra.ExactArgs(1),
	RunE:  runRoutineRuns,
}

var routineTokenDraftCmd = &cobra.Command{
	Use:   "token-draft",
	Short: "Generate an API trigger token draft",
	RunE:  runRoutineTokenDraft,
}

func init() {
	routineCmd.AddCommand(routineListCmd)
	routineCmd.AddCommand(routineGetCmd)
	routineCmd.AddCommand(routineCreateCmd)
	routineCmd.AddCommand(routineUpdateCmd)
	routineCmd.AddCommand(routineDeleteCmd)
	routineCmd.AddCommand(routineRunCmd)
	routineCmd.AddCommand(routineRunsCmd)
	routineCmd.AddCommand(routineTokenDraftCmd)

	routineListCmd.Flags().String("output", "table", "Output format: table or json")

	routineGetCmd.Flags().String("output", "json", "Output format: table or json")

	registerRoutineBodyFlags(routineCreateCmd)
	routineCreateCmd.Flags().String("output", "json", "Output format: table or json")

	registerRoutineBodyFlags(routineUpdateCmd)
	routineUpdateCmd.Flags().String("output", "json", "Output format: table or json")

	routineRunsCmd.Flags().Int("limit", 100, "Maximum number of runs to return")
	routineRunsCmd.Flags().Int("offset", 0, "Number of runs to skip")
	routineRunsCmd.Flags().String("output", "table", "Output format: table or json")

	routineRunCmd.Flags().String("output", "json", "Output format: table or json")
	routineTokenDraftCmd.Flags().String("output", "json", "Output format: table or json")
}

func registerRoutineBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("body", "", "Full routine request JSON body (use - to read from stdin)")
	cmd.Flags().String("body-file", "", "Path to a file containing the full routine request JSON body")
	cmd.Flags().String("name", "", "Routine name")
	cmd.Flags().String("instructions", "", "Issue instructions/template (use - to read from stdin)")
	cmd.Flags().String("priority", "", "Issue priority")
	cmd.Flags().String("assignee", "", "Assignee name (member or agent)")
	cmd.Flags().Int32("due-date-offset-hours", 0, "Due date offset in hours from trigger time")
	cmd.Flags().Bool("enabled", true, "Whether the routine is enabled")
	cmd.Flags().String("dispatch-provider", "", "Dispatch provider")
	cmd.Flags().String("dispatch-daemon", "", "Dispatch daemon name or ID")
	cmd.Flags().String("dispatch-daemon-label", "", "Dispatch daemon label")
	cmd.Flags().StringSlice("subscriber-id", nil, "Subscriber user ID (repeatable)")
	cmd.Flags().StringSlice("label-id", nil, "Label ID (repeatable)")
	cmd.Flags().StringArray("trigger-json", nil, "Trigger JSON object (repeatable)")
	cmd.Flags().StringArray("action-json", nil, "Action JSON object (repeatable)")
	cmd.Flags().String("schedule", "", "Cron schedule for a schedule trigger")
	cmd.Flags().String("timezone", "UTC", "Timezone for --schedule or --run-at")
	cmd.Flags().String("run-at", "", "RFC3339 timestamp for a one-time schedule trigger")
}

func runRoutineList(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var routines []map[string]any
	if err := client.GetJSON(ctx, "/api/routines", &routines); err != nil {
		return fmt.Errorf("list routines: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, routines)
	}

	headers := []string{"ID", "NAME", "ENABLED", "MANAGED", "PRIORITY", "ASSIGNEE", "TRIGGERS", "UPDATED"}
	rows := make([][]string, 0, len(routines))
	for _, r := range routines {
		updated := strVal(r, "updated_at")
		if len(updated) >= 16 {
			updated = updated[:16]
		}
		rows = append(rows, []string{
			truncateID(strVal(r, "id")),
			truncateRunes(strVal(r, "name"), 40),
			boolVal(r, "enabled"),
			boolVal(r, "managed"),
			strVal(r, "priority"),
			formatRoutineAssignee(r),
			formatRoutineTriggers(r["triggers"]),
			updated,
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runRoutineGet(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var routine map[string]any
	if err := client.GetJSON(ctx, "/api/routines/"+args[0], &routine); err != nil {
		return fmt.Errorf("get routine: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		headers := []string{"ID", "NAME", "ENABLED", "MANAGED", "PRIORITY", "ASSIGNEE", "TRIGGERS", "ACTIONS"}
		rows := [][]string{{
			truncateID(strVal(routine, "id")),
			truncateRunes(strVal(routine, "name"), 40),
			boolVal(routine, "enabled"),
			boolVal(routine, "managed"),
			strVal(routine, "priority"),
			formatRoutineAssignee(routine),
			formatRoutineTriggers(routine["triggers"]),
			formatRoutineActions(routine["actions"]),
		}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}
	return cli.PrintJSON(os.Stdout, routine)
}

func runRoutineCreate(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body, err := buildRoutineBody(ctx, cmd, client, nil)
	if err != nil {
		return err
	}
	if _, ok := body["name"]; !ok {
		return fmt.Errorf("--name is required unless --body or --body-file is provided")
	}

	var result map[string]any
	if err := client.PostJSON(ctx, "/api/routines", body, &result); err != nil {
		return fmt.Errorf("create routine: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		headers := []string{"ID", "NAME", "ENABLED", "PRIORITY"}
		rows := [][]string{{
			truncateID(strVal(result, "id")),
			strVal(result, "name"),
			boolVal(result, "enabled"),
			strVal(result, "priority"),
		}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runRoutineUpdate(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var existing map[string]any
	if !routineBodyFlagChanged(cmd) {
		if err := client.GetJSON(ctx, "/api/routines/"+args[0], &existing); err != nil {
			return fmt.Errorf("get routine before update: %w", err)
		}
	}

	body, err := buildRoutineBody(ctx, cmd, client, existing)
	if err != nil {
		return err
	}
	if existing != nil && !hasRoutineUpdateFlag(cmd) {
		return fmt.Errorf("no fields to update; use routine flags, --body, or --body-file")
	}

	var result map[string]any
	if err := client.PatchJSON(ctx, "/api/routines/"+args[0], body, &result); err != nil {
		return fmt.Errorf("update routine: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		headers := []string{"ID", "NAME", "ENABLED", "PRIORITY"}
		rows := [][]string{{
			truncateID(strVal(result, "id")),
			strVal(result, "name"),
			boolVal(result, "enabled"),
			strVal(result, "priority"),
		}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runRoutineDelete(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := client.DeleteJSON(ctx, "/api/routines/"+args[0]); err != nil {
		return fmt.Errorf("delete routine: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Routine %s deleted.\n", truncateID(args[0]))
	return nil
}

func runRoutineRun(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var result map[string]any
	if err := client.PostJSON(ctx, "/api/routines/"+args[0]+"/trigger", map[string]any{}, &result); err != nil {
		return fmt.Errorf("run routine: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		fmt.Fprintf(os.Stderr, "Routine %s ran %s action(s).\n", truncateID(args[0]), strVal(result, "ran"))
		return nil
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runRoutineRuns(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	params := url.Values{}
	if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset, _ := cmd.Flags().GetInt("offset"); offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", offset))
	}
	path := "/api/routines/" + args[0] + "/runs"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var runs []map[string]any
	if err := client.GetJSON(ctx, path, &runs); err != nil {
		return fmt.Errorf("list routine runs: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, runs)
	}

	headers := []string{"ID", "STATUS", "EVENT", "ISSUE", "COMMENT", "CREATED", "ERROR"}
	rows := make([][]string, 0, len(runs))
	for _, run := range runs {
		created := strVal(run, "created_at")
		if len(created) >= 16 {
			created = created[:16]
		}
		rows = append(rows, []string{
			truncateID(strVal(run, "id")),
			strVal(run, "status"),
			strVal(run, "event_type"),
			truncateID(strVal(run, "issue_id")),
			truncateID(strVal(run, "comment_id")),
			created,
			truncateRunes(strVal(run, "error_message"), 50),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runRoutineTokenDraft(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var result map[string]any
	if err := client.PostJSON(ctx, "/api/routine-trigger-token-drafts", map[string]any{}, &result); err != nil {
		return fmt.Errorf("generate routine token draft: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		headers := []string{"DRAFT ID", "TOKEN PREFIX", "TOKEN"}
		rows := [][]string{{strVal(result, "draft_id"), strVal(result, "token_prefix"), strVal(result, "token")}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}
	return cli.PrintJSON(os.Stdout, result)
}

func buildRoutineBody(ctx context.Context, cmd *cobra.Command, client *cli.APIClient, base map[string]any) (map[string]any, error) {
	if routineBodyFlagChanged(cmd) {
		return readRoutineBody(cmd)
	}

	body := map[string]any{}
	for k, v := range base {
		body[k] = v
	}

	if cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		body["name"] = strings.TrimSpace(name)
	}
	if cmd.Flags().Changed("instructions") {
		instructions, err := readFlagOrStdin(cmd, "instructions")
		if err != nil {
			return nil, err
		}
		instructions = strings.TrimSpace(instructions)
		if instructions == "" {
			body["instructions"] = nil
		} else {
			body["instructions"] = instructions
		}
	}
	if cmd.Flags().Changed("priority") {
		priority, _ := cmd.Flags().GetString("priority")
		body["priority"] = priority
	}
	if cmd.Flags().Changed("assignee") {
		assignee, _ := cmd.Flags().GetString("assignee")
		assigneeType, assigneeID, err := resolveAssignee(ctx, client, assignee)
		if err != nil {
			return nil, fmt.Errorf("resolve assignee: %w", err)
		}
		body["assignee_type"] = assigneeType
		body["assignee_id"] = assigneeID
	}
	if cmd.Flags().Changed("due-date-offset-hours") {
		offset, _ := cmd.Flags().GetInt32("due-date-offset-hours")
		body["due_date_offset_hours"] = offset
	}
	if cmd.Flags().Changed("enabled") {
		enabled, _ := cmd.Flags().GetBool("enabled")
		body["enabled"] = enabled
	}
	if cmd.Flags().Changed("dispatch-provider") {
		provider, _ := cmd.Flags().GetString("dispatch-provider")
		body["dispatch_provider"] = nilIfEmpty(provider)
	}
	if cmd.Flags().Changed("dispatch-daemon") {
		daemonName, _ := cmd.Flags().GetString("dispatch-daemon")
		if daemonName == "" {
			body["dispatch_daemon_id"] = nil
		} else {
			daemonID, err := resolveDaemonID(ctx, client, daemonName)
			if err != nil {
				return nil, fmt.Errorf("resolve daemon: %w", err)
			}
			body["dispatch_daemon_id"] = daemonID
		}
	}
	if cmd.Flags().Changed("dispatch-daemon-label") {
		label, _ := cmd.Flags().GetString("dispatch-daemon-label")
		body["dispatch_daemon_label"] = nilIfEmpty(label)
	}
	if cmd.Flags().Changed("subscriber-id") {
		ids, _ := cmd.Flags().GetStringSlice("subscriber-id")
		body["subscriber_ids"] = ids
	}
	if cmd.Flags().Changed("label-id") {
		ids, _ := cmd.Flags().GetStringSlice("label-id")
		body["label_ids"] = ids
	}
	if cmd.Flags().Changed("trigger-json") {
		triggers, err := parseRoutineJSONListFlag(cmd, "trigger-json")
		if err != nil {
			return nil, err
		}
		body["triggers"] = triggers
	} else if cmd.Flags().Changed("schedule") || cmd.Flags().Changed("run-at") {
		trigger, err := buildScheduleRoutineTrigger(cmd)
		if err != nil {
			return nil, err
		}
		body["triggers"] = []map[string]any{trigger}
	}
	if cmd.Flags().Changed("action-json") {
		actions, err := parseRoutineJSONListFlag(cmd, "action-json")
		if err != nil {
			return nil, err
		}
		body["actions"] = actions
	} else if base == nil {
		body["actions"] = []map[string]any{{
			"action_type": "create_issue",
			"config":      map[string]any{},
			"enabled":     true,
			"position":    0,
		}}
	}
	return body, nil
}

func routineBodyFlagChanged(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("body") || cmd.Flags().Changed("body-file")
}

func hasRoutineUpdateFlag(cmd *cobra.Command) bool {
	for _, name := range []string{
		"name", "instructions", "priority", "assignee", "due-date-offset-hours",
		"enabled", "dispatch-provider", "dispatch-daemon", "dispatch-daemon-label",
		"subscriber-id", "label-id", "trigger-json", "action-json", "schedule", "run-at",
	} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return routineBodyFlagChanged(cmd)
}

func readRoutineBody(cmd *cobra.Command) (map[string]any, error) {
	if cmd.Flags().Changed("body") && cmd.Flags().Changed("body-file") {
		return nil, fmt.Errorf("--body and --body-file are mutually exclusive")
	}

	var data []byte
	var err error
	if cmd.Flags().Changed("body-file") {
		path, _ := cmd.Flags().GetString("body-file")
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read --body-file: %w", err)
		}
	} else {
		raw, _ := cmd.Flags().GetString("body")
		if raw == "-" {
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("read stdin for --body: %w", err)
			}
		} else {
			data = []byte(raw)
		}
	}

	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("parse routine JSON body: %w", err)
	}
	return body, nil
}

func parseRoutineJSONListFlag(cmd *cobra.Command, flag string) ([]map[string]any, error) {
	values, _ := cmd.Flags().GetStringArray(flag)
	items := make([]map[string]any, 0, len(values))
	for _, raw := range values {
		var item map[string]any
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("parse --%s JSON: %w", flag, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func buildScheduleRoutineTrigger(cmd *cobra.Command) (map[string]any, error) {
	schedule, _ := cmd.Flags().GetString("schedule")
	runAt, _ := cmd.Flags().GetString("run-at")
	if schedule != "" && runAt != "" {
		return nil, fmt.Errorf("--schedule and --run-at are mutually exclusive")
	}
	timezone, _ := cmd.Flags().GetString("timezone")
	if timezone == "" {
		timezone = "UTC"
	}
	trigger := map[string]any{
		"trigger_type": "schedule",
		"timezone":     timezone,
		"config":       map[string]any{},
		"enabled":      true,
	}
	if runAt != "" {
		trigger["run_at"] = runAt
		trigger["config"] = map[string]any{"mode": "once"}
	} else {
		trigger["schedule"] = schedule
		trigger["config"] = map[string]any{"mode": "custom"}
	}
	return trigger, nil
}

func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func boolVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	b, ok := v.(bool)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	if b {
		return "yes"
	}
	return "no"
}

func formatRoutineAssignee(routine map[string]any) string {
	assigneeType := strVal(routine, "assignee_type")
	assigneeID := strVal(routine, "assignee_id")
	if assigneeType == "" || assigneeID == "" {
		return ""
	}
	return assigneeType + ":" + truncateID(assigneeID)
}

func formatRoutineTriggers(raw any) string {
	triggers, ok := raw.([]any)
	if !ok || len(triggers) == 0 {
		return ""
	}
	parts := make([]string, 0, len(triggers))
	for _, item := range triggers {
		trigger, ok := item.(map[string]any)
		if !ok {
			continue
		}
		label := strVal(trigger, "trigger_type")
		if schedule := strVal(trigger, "schedule"); schedule != "" {
			label += ":" + schedule
		} else if runAt := strVal(trigger, "run_at"); runAt != "" {
			label += ":" + truncateRunes(runAt, 16)
		}
		parts = append(parts, label)
	}
	return truncateRunes(strings.Join(parts, ", "), 60)
}

func formatRoutineActions(raw any) string {
	actions, ok := raw.([]any)
	if !ok || len(actions) == 0 {
		return ""
	}
	parts := make([]string, 0, len(actions))
	for _, item := range actions {
		action, ok := item.(map[string]any)
		if !ok {
			continue
		}
		parts = append(parts, strVal(action, "action_type"))
	}
	return truncateRunes(strings.Join(parts, ", "), 60)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
