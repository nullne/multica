package daemon

import "testing"

func TestTaskTokenTracking(t *testing.T) {
	t.Parallel()

	d := &Daemon{taskTokens: make(map[string]string)}

	d.rememberTaskToken("task-1", "ghs_secret123")
	if got := d.lookupTaskToken("task-1"); got != "ghs_secret123" {
		t.Fatalf("unexpected token: %q", got)
	}

	d.clearTaskToken("task-1")
	if got := d.lookupTaskToken("task-1"); got != "" {
		t.Fatalf("expected token to be cleared, got %q", got)
	}

	// Empty taskID and empty token should both be no-ops.
	d.rememberTaskToken("", "ghs_other")
	if got := d.lookupTaskToken(""); got != "" {
		t.Fatalf("expected empty taskID lookup to return empty, got %q", got)
	}

	d.rememberTaskToken("task-2", "")
	if got := d.lookupTaskToken("task-2"); got != "" {
		t.Fatalf("expected empty token to be skipped, got %q", got)
	}

	d.clearTaskToken("") // should not panic
}
