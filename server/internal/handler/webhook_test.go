package handler

import (
	"encoding/json"
	"testing"

	wh "github.com/nullne/multica/server/internal/webhook"
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

func TestMergeActionConfig_CommentIssue(t *testing.T) {
	existing := db.WebhookAction{
		ActionType: "comment_issue",
		Config: mustJSON(CommentIssueActionConfig{
			ContentTemplate: "old",
			MentionAgentID:  "agent-1",
		}),
	}

	merged, err := mergeActionConfig(existing, map[string]any{"content_template": "new"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CommentIssueActionConfig
	json.Unmarshal(merged, &result)
	if result.ContentTemplate != "new" {
		t.Errorf("content_template = %q, want new", result.ContentTemplate)
	}
	if result.MentionAgentID != "agent-1" {
		t.Errorf("mention_agent_id clobbered: %q", result.MentionAgentID)
	}
}

func TestActionMatchesFilters_NoFilters(t *testing.T) {
	evt := wh.Event{Type: "github.pull_request.opened", Data: map[string]string{"repo": "acme/widgets"}}
	if !actionMatchesFilters(nil, nil, evt) {
		t.Error("empty filters should match everything")
	}
}

func TestActionMatchesFilters_EventTypePrefix(t *testing.T) {
	evt := wh.Event{Type: "github.pull_request.opened", Data: map[string]string{"repo": "acme/widgets"}}
	if !actionMatchesFilters([]string{"github.pull_request"}, nil, evt) {
		t.Error("prefix match should accept github.pull_request.opened")
	}
	if actionMatchesFilters([]string{"github.issues"}, nil, evt) {
		t.Error("non-matching prefix should reject")
	}
	if !actionMatchesFilters([]string{"github.issues", "github.pull_request"}, nil, evt) {
		t.Error("OR match should accept when one entry matches")
	}
}

func TestActionMatchesFilters_RepoMatch(t *testing.T) {
	evt := wh.Event{Type: "github.push", Data: map[string]string{"repo": "acme/widgets"}}
	if !actionMatchesFilters(nil, []string{"acme/widgets"}, evt) {
		t.Error("exact repo match should accept")
	}
	if actionMatchesFilters(nil, []string{"other/repo"}, evt) {
		t.Error("non-matching repo should reject")
	}
}

func TestActionMatchesFilters_ExactEventType(t *testing.T) {
	evt := wh.Event{Type: "github.push", Data: map[string]string{"repo": "acme/widgets"}}
	if !actionMatchesFilters([]string{"github.push"}, nil, evt) {
		t.Error("exact event type should match")
	}
}

func TestValidateActionConfig_CommentIssueRequiresBot(t *testing.T) {
	webhookNoBot := db.Webhook{}
	cfg := CommentIssueActionConfig{ContentTemplate: "{{.body}}"}
	if err := validateActionConfig(webhookNoBot, "comment_issue", cfg); err == nil {
		t.Error("comment_issue without bot_user_id should fail validation")
	}
}

func TestValidateActionConfig_CreateIssueRequiresAgent(t *testing.T) {
	if err := validateActionConfig(db.Webhook{}, "create_issue", map[string]any{}); err == nil {
		t.Error("create_issue without agent_id should fail validation")
	}
}

func TestRenderTemplate(t *testing.T) {
	out := renderTemplate("Hello {{.name}}, repo={{.repo}}", map[string]string{
		"name": "world",
		"repo": "acme/x",
	})
	if out != "Hello world, repo=acme/x" {
		t.Errorf("got %q", out)
	}
}

func TestMergeActionConfig_SubscriberIDsPreserved(t *testing.T) {
	existing := db.WebhookAction{
		ActionType: "create_issue",
		Config: mustJSON(CreateIssueActionConfig{
			AgentID:       "agent-123",
			SubscriberIDs: []string{"user-aaa", "user-bbb"},
		}),
	}

	incoming := map[string]any{
		"title_template": "updated title",
	}

	merged, err := mergeActionConfig(existing, incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CreateIssueActionConfig
	if err := json.Unmarshal(merged, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.SubscriberIDs) != 2 || result.SubscriberIDs[0] != "user-aaa" || result.SubscriberIDs[1] != "user-bbb" {
		t.Errorf("subscriber_ids not preserved: got %v", result.SubscriberIDs)
	}
	if result.TitleTemplate != "updated title" {
		t.Errorf("title_template not updated: got %q", result.TitleTemplate)
	}
}

func TestMergeActionConfig_SubscriberIDsUpdated(t *testing.T) {
	existing := db.WebhookAction{
		ActionType: "create_issue",
		Config: mustJSON(CreateIssueActionConfig{
			AgentID:       "agent-123",
			SubscriberIDs: []string{"user-old"},
		}),
	}

	incoming := map[string]any{
		"agent_id":       "agent-123",
		"subscriber_ids": []string{"user-new-1", "user-new-2"},
	}

	merged, err := mergeActionConfig(existing, incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CreateIssueActionConfig
	if err := json.Unmarshal(merged, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.SubscriberIDs) != 2 || result.SubscriberIDs[0] != "user-new-1" {
		t.Errorf("subscriber_ids not updated: got %v", result.SubscriberIDs)
	}
}

