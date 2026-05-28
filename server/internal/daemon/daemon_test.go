package daemon

import (
	"strings"
	"testing"
)

func TestNormalizeServerBaseURL(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("ws://localhost:8080/ws")
	if err != nil {
		t.Fatalf("NormalizeServerBaseURL returned error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Fatalf("expected http://localhost:8080, got %s", got)
	}
}

func TestBuildPromptContainsIssueID(t *testing.T) {
	t.Parallel()

	issueID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	prompt := BuildPrompt(Task{
		IssueID: issueID,
		Agent: &AgentData{
			Name: "Local Codex",
			Skills: []SkillData{
				{Name: "Concise", Content: "Be concise."},
			},
		},
	})

	// Prompt should contain the issue ID and CLI hint.
	for _, want := range []string{
		issueID,
		"multica issue get",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}

	// Skills should NOT be inlined in the prompt (they're in runtime config).
	for _, absent := range []string{"## Agent Skills", "Be concise."} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q (skills are in runtime config)", absent)
		}
	}
}

func TestBuildPromptNoIssueDetails(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID: "test-id",
		Agent:   &AgentData{Name: "Test"},
	})

	// Prompt should not contain issue title/description (agent fetches via CLI).
	for _, absent := range []string{"**Issue:**", "**Summary:**"} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q — agent fetches details via CLI", absent)
		}
	}
}

func TestBuildPromptIncludesRoutineEventContext(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID: "issue-1",
		Context: map[string]any{
			"routine_event": map[string]any{
				"type":        "custom",
				"raw_payload": `{"deployment":{"service":"api"}}`,
			},
		},
	})
	for _, want := range []string{
		"routine trigger",
		"raw_payload",
		"deployment",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func TestBuildPromptCriteriaRoleIncludesStructuredBlock(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID: "issue-1",
		Context: map[string]any{"role": "criteria"},
	})
	for _, want := range []string{
		"acceptance-criteria definition step",
		"<!--multica:criteria",
		"\"criteria\"",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func TestBuildPromptValidatorRoleIncludesCriteriaJSON(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID: "issue-1",
		Context: map[string]any{
			"role": "validator",
			"acceptance_criteria": []map[string]any{
				{"id": "AC-1", "check": "response contains x"},
			},
		},
	})
	for _, want := range []string{
		"verification/acceptance step",
		"acceptance criteria JSON data",
		"AC-1",
		"<!--multica:verification",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func TestExtractPRURL(t *testing.T) {
	t.Parallel()

	text := "Implemented the fix.\nPR: https://github.com/acme/demo/pull/42\nPlease review."
	got := extractPRURL(text)
	if got != "https://github.com/acme/demo/pull/42" {
		t.Fatalf("unexpected PR URL: %q", got)
	}

	if url := extractPRURL("No pull request yet."); url != "" {
		t.Fatalf("expected no PR URL, got %q", url)
	}
}

func TestTaskBranchTracking(t *testing.T) {
	t.Parallel()

	d := &Daemon{taskBranches: make(map[string]string)}
	d.rememberTaskBranch("task-1", "agent/codex/abc12345")
	if got := d.consumeTaskBranch("task-1"); got != "agent/codex/abc12345" {
		t.Fatalf("unexpected consumed branch: %q", got)
	}
	if got := d.consumeTaskBranch("task-1"); got != "" {
		t.Fatalf("expected branch to be cleared, got %q", got)
	}

	d.rememberTaskBranch("task-2", "agent/codex/def67890")
	d.clearTaskBranch("task-2")
	if got := d.consumeTaskBranch("task-2"); got != "" {
		t.Fatalf("expected cleared branch, got %q", got)
	}
}

func TestApplyRegisterResponseKeepsHiddenRuntimesOutOfActiveAssignments(t *testing.T) {
	t.Parallel()

	d := &Daemon{}
	d.applyRegisterResponse(&RegisterResponse{
		Workspaces: []WorkspaceRegistration{
			{
				WorkspaceID: "ws-enabled",
				Enabled:     true,
				Runtimes: []Runtime{
					{ID: "rt-enabled", Provider: "claude"},
				},
			},
			{
				WorkspaceID: "ws-hidden",
				Enabled:     false,
				Runtimes: []Runtime{
					{ID: "rt-hidden", Provider: "codex"},
				},
			},
		},
	})

	if got := d.allRuntimeIDs(); len(got) != 2 {
		t.Fatalf("expected both projected runtimes to be tracked, got %v", got)
	}
	if rt := d.findRuntime("rt-hidden"); rt == nil || rt.Provider != "codex" {
		t.Fatalf("expected hidden runtime to stay claimable, got %+v", rt)
	}

	d.mu.Lock()
	enabled := d.workspaces["ws-enabled"]
	hidden := d.workspaces["ws-hidden"]
	d.mu.Unlock()

	if enabled == nil || !enabled.enabled {
		t.Fatalf("expected enabled workspace to remain active, got %+v", enabled)
	}
	if hidden == nil {
		t.Fatal("expected hidden workspace projection to be retained")
	}
	if hidden.enabled {
		t.Fatalf("expected hidden workspace to stay out of active assignment set, got %+v", hidden)
	}
}
