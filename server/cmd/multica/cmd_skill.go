package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/nullne/multica/server/internal/cli"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage skills",
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "List skills in the workspace",
	RunE:  runSkillList,
}

var skillGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get skill details (including files)",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillGet,
}

var skillCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new skill",
	RunE:  runSkillCreate,
}

var skillUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillUpdate,
}

var skillDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillDelete,
}

var skillImportCmd = &cobra.Command{
	Use:   "import <url>",
	Short: "Import a skill from clawhub.ai or skills.sh",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillImport,
}

var skillAssignCmd = &cobra.Command{
	Use:   "assign <agent>",
	Short: "Replace the set of skills attached to an agent",
	Long: `Replace the skill set on an agent.

Pass each desired skill via --skill (repeatable). Skills can be specified by
ID or by name (case-insensitive substring match). Pass --skill with no
values to remove all skills from the agent.`,
	Args: cobra.ExactArgs(1),
	RunE: runSkillAssign,
}

func init() {
	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillGetCmd)
	skillCmd.AddCommand(skillCreateCmd)
	skillCmd.AddCommand(skillUpdateCmd)
	skillCmd.AddCommand(skillDeleteCmd)
	skillCmd.AddCommand(skillImportCmd)
	skillCmd.AddCommand(skillAssignCmd)

	skillListCmd.Flags().String("output", "table", "Output format: table or json")

	skillGetCmd.Flags().String("output", "json", "Output format: table or json")

	skillCreateCmd.Flags().String("name", "", "Skill name (required)")
	skillCreateCmd.Flags().String("description", "", "Skill description")
	skillCreateCmd.Flags().String("content", "", "SKILL.md body (use - to read from stdin)")
	skillCreateCmd.Flags().String("content-file", "", "Path to a file containing the SKILL.md body")
	skillCreateCmd.Flags().StringSlice("file", nil, "Attach a supporting file: 'localpath' or 'remotepath=localpath' (repeatable)")
	skillCreateCmd.Flags().String("config", "", "Skill config as inline JSON (default: {})")
	skillCreateCmd.Flags().String("output", "json", "Output format: table or json")

	skillUpdateCmd.Flags().String("name", "", "New name")
	skillUpdateCmd.Flags().String("description", "", "New description")
	skillUpdateCmd.Flags().String("content", "", "New SKILL.md body (use - to read from stdin)")
	skillUpdateCmd.Flags().String("content-file", "", "Path to a file containing the new SKILL.md body")
	skillUpdateCmd.Flags().StringSlice("file", nil, "Replace supporting files: 'localpath' or 'remotepath=localpath' (repeatable). Passing --file at all replaces ALL existing files.")
	skillUpdateCmd.Flags().String("config", "", "New config as inline JSON")
	skillUpdateCmd.Flags().String("output", "json", "Output format: table or json")

	skillImportCmd.Flags().String("output", "json", "Output format: table or json")

	skillAssignCmd.Flags().StringSlice("skill", nil, "Skill name or ID to attach (repeatable). Omit to clear.")
	skillAssignCmd.Flags().String("output", "table", "Output format: table or json")
}

// ---------------------------------------------------------------------------
// list / get
// ---------------------------------------------------------------------------

func runSkillList(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var skills []map[string]any
	if err := client.GetJSON(ctx, "/api/skills", &skills); err != nil {
		return fmt.Errorf("list skills: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, skills)
	}

	headers := []string{"ID", "NAME", "DESCRIPTION", "UPDATED"}
	rows := make([][]string, 0, len(skills))
	for _, s := range skills {
		desc := strVal(s, "description")
		if utf8.RuneCountInString(desc) > 60 {
			runes := []rune(desc)
			desc = string(runes[:57]) + "..."
		}
		updated := strVal(s, "updated_at")
		if len(updated) >= 16 {
			updated = updated[:16]
		}
		rows = append(rows, []string{
			truncateID(strVal(s, "id")),
			strVal(s, "name"),
			desc,
			updated,
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runSkillGet(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var skill map[string]any
	if err := client.GetJSON(ctx, "/api/skills/"+args[0], &skill); err != nil {
		return fmt.Errorf("get skill: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		desc := strVal(skill, "description")
		if utf8.RuneCountInString(desc) > 80 {
			runes := []rune(desc)
			desc = string(runes[:77]) + "..."
		}
		files, _ := skill["files"].([]any)
		filePaths := make([]string, 0, len(files))
		for _, f := range files {
			if fm, ok := f.(map[string]any); ok {
				filePaths = append(filePaths, strVal(fm, "path"))
			}
		}
		filesStr := strings.Join(filePaths, ", ")
		if utf8.RuneCountInString(filesStr) > 60 {
			runes := []rune(filesStr)
			filesStr = string(runes[:57]) + "..."
		}
		headers := []string{"ID", "NAME", "DESCRIPTION", "FILES"}
		rows := [][]string{{
			truncateID(strVal(skill, "id")),
			strVal(skill, "name"),
			desc,
			filesStr,
		}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}

	return cli.PrintJSON(os.Stdout, skill)
}

// ---------------------------------------------------------------------------
// create / update
// ---------------------------------------------------------------------------

func runSkillCreate(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		return fmt.Errorf("--name is required")
	}

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	body := map[string]any{"name": name}
	if v, _ := cmd.Flags().GetString("description"); v != "" {
		body["description"] = v
	}

	content, err := readSkillContent(cmd)
	if err != nil {
		return err
	}
	if content != "" {
		body["content"] = content
	}

	if cmd.Flags().Changed("config") {
		raw, _ := cmd.Flags().GetString("config")
		var cfg any
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return fmt.Errorf("parse --config JSON: %w", err)
		}
		body["config"] = cfg
	}

	files, err := readSkillFiles(cmd)
	if err != nil {
		return err
	}
	if files != nil {
		body["files"] = files
	}

	timeout := 15 * time.Second
	if len(files) > 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var result map[string]any
	if err := client.PostJSON(ctx, "/api/skills", body, &result); err != nil {
		return fmt.Errorf("create skill: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		fmt.Fprintf(os.Stderr, "Skill %s created.\n", strVal(result, "id"))
		return nil
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runSkillUpdate(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	body := map[string]any{}
	if cmd.Flags().Changed("name") {
		v, _ := cmd.Flags().GetString("name")
		body["name"] = v
	}
	if cmd.Flags().Changed("description") {
		v, _ := cmd.Flags().GetString("description")
		body["description"] = v
	}

	if cmd.Flags().Changed("content") || cmd.Flags().Changed("content-file") {
		content, err := readSkillContent(cmd)
		if err != nil {
			return err
		}
		body["content"] = content
	}

	if cmd.Flags().Changed("config") {
		raw, _ := cmd.Flags().GetString("config")
		var cfg any
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return fmt.Errorf("parse --config JSON: %w", err)
		}
		body["config"] = cfg
	}

	if cmd.Flags().Changed("file") {
		files, err := readSkillFiles(cmd)
		if err != nil {
			return err
		}
		// Always send the array (possibly empty) so the server replaces the
		// existing file set.
		if files == nil {
			files = []map[string]string{}
		}
		body["files"] = files
	}

	if len(body) == 0 {
		return fmt.Errorf("no fields to update; use --name, --description, --content, --content-file, --config, or --file")
	}

	timeout := 15 * time.Second
	if v, ok := body["files"]; ok {
		if arr, ok := v.([]map[string]string); ok && len(arr) > 0 {
			timeout = 60 * time.Second
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var result map[string]any
	if err := client.PutJSON(ctx, "/api/skills/"+args[0], body, &result); err != nil {
		return fmt.Errorf("update skill: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		fmt.Fprintf(os.Stderr, "Skill %s updated.\n", strVal(result, "id"))
		return nil
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runSkillDelete(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := client.DeleteJSON(ctx, "/api/skills/"+args[0]); err != nil {
		return fmt.Errorf("delete skill: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Skill %s deleted.\n", truncateID(args[0]))
	return nil
}

// ---------------------------------------------------------------------------
// import
// ---------------------------------------------------------------------------

func runSkillImport(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	// Imports may pull many files from the network — give the server some headroom.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	body := map[string]any{"url": args[0]}
	var result map[string]any
	if err := client.PostJSON(ctx, "/api/skills/import", body, &result); err != nil {
		return fmt.Errorf("import skill: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		fmt.Fprintf(os.Stderr, "Skill %s imported as %q.\n", strVal(result, "id"), strVal(result, "name"))
		return nil
	}
	return cli.PrintJSON(os.Stdout, result)
}

// ---------------------------------------------------------------------------
// assign skills to an agent
// ---------------------------------------------------------------------------

func runSkillAssign(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required: use --workspace-id or set MULTICA_WORKSPACE_ID")
	}

	if !cmd.Flags().Changed("skill") {
		return fmt.Errorf("--skill is required (pass it with no value to clear all skills)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agentID, err := resolveAgent(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve agent: %w", err)
	}

	skillRefs, _ := cmd.Flags().GetStringSlice("skill")
	skillIDs := make([]string, 0, len(skillRefs))
	if len(skillRefs) > 0 {
		all, fetchErr := fetchAllSkills(ctx, client)
		if fetchErr != nil {
			return fetchErr
		}
		for _, ref := range skillRefs {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				continue
			}
			id, resolveErr := resolveSkillRef(all, ref)
			if resolveErr != nil {
				return resolveErr
			}
			skillIDs = append(skillIDs, id)
		}
	}

	body := map[string]any{"skill_ids": skillIDs}
	var result []map[string]any
	if err := client.PutJSON(ctx, "/api/agents/"+agentID+"/skills", body, &result); err != nil {
		return fmt.Errorf("assign skills: %w", err)
	}

	if len(skillIDs) == 0 {
		fmt.Fprintf(os.Stderr, "Cleared all skills from agent %s.\n", truncateID(agentID))
	} else {
		fmt.Fprintf(os.Stderr, "Agent %s now has %d skill(s).\n", truncateID(agentID), len(skillIDs))
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}

	headers := []string{"ID", "NAME", "DESCRIPTION"}
	rows := make([][]string, 0, len(result))
	for _, s := range result {
		desc := strVal(s, "description")
		if utf8.RuneCountInString(desc) > 60 {
			runes := []rune(desc)
			desc = string(runes[:57]) + "..."
		}
		rows = append(rows, []string{
			truncateID(strVal(s, "id")),
			strVal(s, "name"),
			desc,
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// readSkillContent returns the SKILL.md body from --content (with stdin
// support via "-") or --content-file. Both may be empty.
func readSkillContent(cmd *cobra.Command) (string, error) {
	contentFile, _ := cmd.Flags().GetString("content-file")
	contentFlag, _ := cmd.Flags().GetString("content")

	if contentFile != "" && contentFlag != "" {
		return "", fmt.Errorf("--content and --content-file are mutually exclusive")
	}
	if contentFile != "" {
		data, err := os.ReadFile(contentFile)
		if err != nil {
			return "", fmt.Errorf("read --content-file: %w", err)
		}
		return string(data), nil
	}
	return readFlagOrStdin(cmd, "content")
}

// readSkillFiles parses the --file flag entries into the request payload.
// Each entry is either "localpath" (remote path = basename) or
// "remotepath=localpath".
func readSkillFiles(cmd *cobra.Command) ([]map[string]string, error) {
	entries, _ := cmd.Flags().GetStringSlice("file")
	if len(entries) == 0 {
		return nil, nil
	}

	out := make([]map[string]string, 0, len(entries))
	for _, raw := range entries {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		var remote, local string
		if idx := strings.Index(raw, "="); idx > 0 {
			remote = strings.TrimSpace(raw[:idx])
			local = strings.TrimSpace(raw[idx+1:])
		} else {
			local = raw
			remote = filepath.Base(raw)
		}
		if local == "" || remote == "" {
			return nil, fmt.Errorf("invalid --file %q (expected 'localpath' or 'remotepath=localpath')", raw)
		}

		data, err := os.ReadFile(local)
		if err != nil {
			return nil, fmt.Errorf("read file %s: %w", local, err)
		}
		out = append(out, map[string]string{
			"path":    remote,
			"content": string(data),
		})
	}
	return out, nil
}

// fetchAllSkills returns every skill in the current workspace.
func fetchAllSkills(ctx context.Context, client *cli.APIClient) ([]map[string]any, error) {
	path := "/api/skills"
	if client.WorkspaceID != "" {
		path += "?" + url.Values{"workspace_id": {client.WorkspaceID}}.Encode()
	}
	var skills []map[string]any
	if err := client.GetJSON(ctx, path, &skills); err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	return skills, nil
}

// resolveSkillRef returns the skill ID matching ref, which may be an exact
// ID or a case-insensitive substring of a skill name.
func resolveSkillRef(all []map[string]any, ref string) (string, error) {
	refLower := strings.ToLower(ref)

	var idMatch map[string]any
	var nameMatches []map[string]any
	for _, s := range all {
		if strVal(s, "id") == ref {
			idMatch = s
			break
		}
		if strings.Contains(strings.ToLower(strVal(s, "name")), refLower) {
			nameMatches = append(nameMatches, s)
		}
	}
	if idMatch != nil {
		return strVal(idMatch, "id"), nil
	}
	switch len(nameMatches) {
	case 0:
		return "", fmt.Errorf("no skill found matching %q", ref)
	case 1:
		return strVal(nameMatches[0], "id"), nil
	default:
		parts := make([]string, 0, len(nameMatches))
		for _, m := range nameMatches {
			parts = append(parts, fmt.Sprintf("  %q (%s)", strVal(m, "name"), truncateID(strVal(m, "id"))))
		}
		return "", fmt.Errorf("ambiguous skill %q; matches:\n%s", ref, strings.Join(parts, "\n"))
	}
}
