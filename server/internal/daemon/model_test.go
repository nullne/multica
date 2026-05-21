package daemon

import "testing"

func TestExecutionModelPrefersTaskRequestedModel(t *testing.T) {
	t.Parallel()

	got := executionModel(Task{RequestedModel: "gpt-5.2-codex"}, AgentEntry{Model: "gpt-5"})
	if got != "gpt-5.2-codex" {
		t.Fatalf("executionModel = %q, want task requested model", got)
	}
}

func TestExecutionModelFallsBackToAgentEntryModel(t *testing.T) {
	t.Parallel()

	got := executionModel(Task{}, AgentEntry{Model: "gpt-5"})
	if got != "gpt-5" {
		t.Fatalf("executionModel = %q, want agent entry model", got)
	}
}
