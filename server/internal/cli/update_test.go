package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestManagedClientUpdateReportHasFailures(t *testing.T) {
	report := ManagedClientUpdateReport{
		Results: []ManagedClientUpdateResult{
			{Name: "claude", Status: "updated"},
			{Name: "codex", Status: "failed", Err: errors.New("boom")},
		},
	}
	if !report.HasFailures() {
		t.Fatal("HasFailures() = false, want true")
	}
	if got := report.String(); !strings.Contains(got, "codex: failed") {
		t.Fatalf("String() missing failure status: %q", got)
	}
}

func TestSanitizeCommandOutput(t *testing.T) {
	short := "ok"
	if got := sanitizeCommandOutput(short); got != short {
		t.Fatalf("sanitizeCommandOutput(short) = %q, want %q", got, short)
	}

	long := strings.Repeat("a", 900)
	got := sanitizeCommandOutput(long)
	if !strings.Contains(got, "...(truncated)") {
		t.Fatalf("sanitizeCommandOutput(long) should include truncation marker: %q", got)
	}
	if len(got) >= len(long) {
		t.Fatalf("sanitizeCommandOutput(long) length = %d, want < %d", len(got), len(long))
	}
}
