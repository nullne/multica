package daemon

import "testing"

func TestValidateRepoCheckoutRequest(t *testing.T) {
	t.Parallel()

	req := repoCheckoutRequest{
		URL:         "https://github.com/nullne/multica",
		WorkspaceID: "ws-1",
		WorkDir:     "/tmp/workdir",
		AgentName:   "Debuger",
		TaskID:      "task-1",
	}
	if err := validateRepoCheckoutRequest(req); err != nil {
		t.Fatalf("expected valid checkout request, got %v", err)
	}

	req.AgentName = ""
	req.TaskID = ""
	err := validateRepoCheckoutRequest(req)
	if err == nil {
		t.Fatal("expected error for missing request fields")
	}
	if got, want := err.Error(), "agent_name, task_id are required"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
