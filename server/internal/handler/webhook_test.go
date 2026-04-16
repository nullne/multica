package handler

import (
	"encoding/json"
	"testing"

	db "github.com/nullne/multica/server/pkg/db/generated"
)

func TestMergeActionConfig_PartialUpdate(t *testing.T) {
	existing := db.WebhookAction{
		ActionType: "create_issue",
		Config: mustJSON(CreateIssueActionConfig{
			AgentID:          "agent-123",
			TitleTemplate:    "[Alert] {{.title}}",
			DispatchDaemonID: "daemon-456",
		}),
	}

	incoming := map[string]any{
		"dispatch_provider": "codex",
	}

	merged, err := mergeActionConfig(existing, incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CreateIssueActionConfig
	if err := json.Unmarshal(merged, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.AgentID != "agent-123" {
		t.Errorf("agent_id clobbered: got %q, want %q", result.AgentID, "agent-123")
	}
	if result.TitleTemplate != "[Alert] {{.title}}" {
		t.Errorf("title_template clobbered: got %q", result.TitleTemplate)
	}
	if result.DispatchDaemonID != "daemon-456" {
		t.Errorf("dispatch_daemon_id clobbered: got %q", result.DispatchDaemonID)
	}
	if result.DispatchProvider != "codex" {
		t.Errorf("dispatch_provider not applied: got %q, want %q", result.DispatchProvider, "codex")
	}
}

func TestMergeActionConfig_FullReplace(t *testing.T) {
	existing := db.WebhookAction{
		ActionType: "create_issue",
		Config: mustJSON(CreateIssueActionConfig{
			AgentID:       "agent-old",
			TitleTemplate: "old title",
		}),
	}

	incoming := CreateIssueActionConfig{
		AgentID:          "agent-new",
		TitleTemplate:    "new title",
		DispatchProvider: "claude",
	}

	merged, err := mergeActionConfig(existing, incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CreateIssueActionConfig
	json.Unmarshal(merged, &result)

	if result.AgentID != "agent-new" {
		t.Errorf("agent_id not updated: got %q", result.AgentID)
	}
	if result.TitleTemplate != "new title" {
		t.Errorf("title_template not updated: got %q", result.TitleTemplate)
	}
	if result.DispatchProvider != "claude" {
		t.Errorf("dispatch_provider not set: got %q", result.DispatchProvider)
	}
}

func TestMergeActionConfig_MissingAgentID(t *testing.T) {
	existing := db.WebhookAction{
		ActionType: "create_issue",
		Config:     mustJSON(CreateIssueActionConfig{}),
	}

	incoming := map[string]any{
		"dispatch_provider": "codex",
	}

	_, err := mergeActionConfig(existing, incoming)
	if err == nil {
		t.Fatal("expected error for missing agent_id, got nil")
	}
}

func TestMergeActionConfig_NonCreateIssue(t *testing.T) {
	existing := db.WebhookAction{
		ActionType: "custom",
		Config:     []byte(`{"foo":"bar"}`),
	}

	incoming := map[string]any{"baz": "qux"}

	merged, err := mergeActionConfig(existing, incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	json.Unmarshal(merged, &result)
	if result["baz"] != "qux" {
		t.Errorf("expected baz=qux, got %v", result["baz"])
	}
	if _, ok := result["foo"]; ok {
		t.Errorf("non-create_issue should not merge, but found 'foo'")
	}
}

