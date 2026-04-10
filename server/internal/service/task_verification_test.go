package service

import (
	"encoding/json"
	"testing"
)

func TestExtractCriteriaPayload(t *testing.T) {
	t.Parallel()

	output := `analysis text
<!--multica:criteria
{"criteria":[{"id":"AC-1","title":"Title","check":"Check","severity":"must"}]}
-->
`

	criteria, err := extractCriteriaPayload(output)
	if err != nil {
		t.Fatalf("extractCriteriaPayload returned error: %v", err)
	}
	if len(criteria) != 1 {
		t.Fatalf("expected 1 criterion, got %d", len(criteria))
	}
	if criteria[0]["id"] != "AC-1" {
		t.Fatalf("expected criterion id AC-1, got %v", criteria[0]["id"])
	}
}

func TestExtractVerificationPayload(t *testing.T) {
	t.Parallel()

	output := `summary
<!--multica:verification
{"decision":"fail","summary":"not passed","failed_checks":[{"id":"AC-1","reason":"missing output"}]}
-->
`

	got, err := extractVerificationPayload(output)
	if err != nil {
		t.Fatalf("extractVerificationPayload returned error: %v", err)
	}
	if got.Decision != "fail" {
		t.Fatalf("expected decision fail, got %s", got.Decision)
	}
	if len(got.FailedChecks) != 1 || got.FailedChecks[0].ID != "AC-1" {
		t.Fatalf("unexpected failed checks: %#v", got.FailedChecks)
	}
}

func TestStripMachineBlocks(t *testing.T) {
	t.Parallel()

	output := `human summary
<!--multica:criteria
{"criteria":[{"id":"AC-1"}]}
-->
more text
<!--multica:verification
{"decision":"pass","summary":"ok","failed_checks":[]}
-->
`
	got := stripMachineBlocks(output)
	if got != "human summary\n\nmore text" {
		t.Fatalf("unexpected stripped output: %q", got)
	}
}

func TestBuildAndParseVerificationTaskContext(t *testing.T) {
	t.Parallel()

	ctx := verificationTaskContext{
		Flow:            verificationFlowName,
		Role:            taskRoleValidator,
		Round:           2,
		ExecutorAgentID: "executor-id",
		VerifierAgentID: "verifier-id",
		AcceptanceCriteria: []map[string]any{
			{"id": "AC-1"},
		},
	}

	raw := buildVerificationTaskContext(ctx)
	if len(raw) == 0 {
		t.Fatal("expected non-empty context bytes")
	}
	parsed := parseVerificationTaskContext(raw)
	if parsed.Role != taskRoleValidator || parsed.Round != 2 {
		t.Fatalf("unexpected parsed context: %#v", parsed)
	}

	// Ensure marshaled JSON stays valid and stable for daemon payload decoding.
	var anyMap map[string]any
	if err := json.Unmarshal(raw, &anyMap); err != nil {
		t.Fatalf("context JSON is invalid: %v", err)
	}
}
