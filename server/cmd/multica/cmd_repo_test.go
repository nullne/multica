package main

import (
	"strings"
	"testing"
)

func TestValidateRepoCheckoutContext(t *testing.T) {
	t.Parallel()

	if err := validateRepoCheckoutContext("ws-1", "Debuger", "task-1"); err != nil {
		t.Fatalf("expected valid checkout context, got %v", err)
	}

	err := validateRepoCheckoutContext("", "", "")
	if err == nil {
		t.Fatal("expected error for missing daemon task context")
	}

	for _, want := range []string{"MULTICA_WORKSPACE_ID", "MULTICA_AGENT_NAME", "MULTICA_TASK_ID"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error %q to contain %q", err, want)
		}
	}
}
