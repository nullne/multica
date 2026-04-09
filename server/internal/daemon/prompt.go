package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BuildPrompt constructs the task prompt for an agent CLI.
// Keep this minimal — detailed instructions live in CLAUDE.md / AGENTS.md
// injected by execenv.InjectRuntimeConfig.
func BuildPrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a local coding agent for a Multica workspace.\n\n")
	fmt.Fprintf(&b, "Your assigned issue ID is: %s\n\n", task.IssueID)
	fmt.Fprintf(&b, "Start by running `multica issue get %s --output json` to understand your task, then complete it.\n", task.IssueID)

	role := taskContextRole(task.Context)
	switch role {
	case "criteria":
		b.WriteString("\nThis task is the acceptance-criteria definition step before execution.\n")
		b.WriteString("Define clear, testable acceptance criteria for this issue, then include the following machine-readable block exactly once:\n\n")
		b.WriteString("<!--multica:criteria\n")
		b.WriteString("{\"criteria\":[{\"id\":\"AC-1\",\"title\":\"...\",\"check\":\"...\",\"severity\":\"must\"}]}\n")
		b.WriteString("-->\n")
	case "validator":
		b.WriteString("\nThis task is the verification/acceptance step.\n")
		if raw := marshalContextField(task.Context, "acceptance_criteria"); raw != "" {
			b.WriteString("Verify the work against these acceptance criteria JSON data:\n")
			b.WriteString(raw + "\n")
		}
		b.WriteString("Then include this machine-readable block exactly once:\n\n")
		b.WriteString("<!--multica:verification\n")
		b.WriteString("{\"decision\":\"pass|fail\",\"summary\":\"...\",\"failed_checks\":[{\"id\":\"AC-1\",\"reason\":\"...\"}]}\n")
		b.WriteString("-->\n")
	case "rework":
		round := taskContextRound(task.Context)
		if round > 0 {
			fmt.Fprintf(&b, "\nThis task is a rework round (round %d) after verification failed.\n", round)
		} else {
			b.WriteString("\nThis task is a rework round after verification failed.\n")
		}
		if summary := taskContextString(task.Context, "verification_summary"); summary != "" {
			b.WriteString("Previous verification summary:\n")
			b.WriteString(summary + "\n")
		}
		if raw := marshalContextField(task.Context, "failed_checks"); raw != "" {
			b.WriteString("Failed checks JSON:\n")
			b.WriteString(raw + "\n")
		}
	}
	return b.String()
}

func taskContextRole(ctx map[string]any) string {
	return taskContextString(ctx, "role")
}

func taskContextRound(ctx map[string]any) int {
	if ctx == nil {
		return 0
	}
	v, ok := ctx["round"]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	default:
		return 0
	}
}

func taskContextString(ctx map[string]any, key string) string {
	if ctx == nil {
		return ""
	}
	v, ok := ctx[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func marshalContextField(ctx map[string]any, key string) string {
	if ctx == nil {
		return ""
	}
	v, ok := ctx[key]
	if !ok || v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
