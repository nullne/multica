package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nullne/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

func TestTruncateID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"short", "abc", "abc"},
		{"exact 8", "abcdefgh", "abcdefgh"},
		{"longer than 8", "abcdefgh-1234-5678", "abcdefgh"},
		{"empty", "", ""},
		{"unicode", "日本語テスト文字列追加", "日本語テスト文字"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateID(tt.id)
			if got != tt.want {
				t.Errorf("truncateID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestFormatAssignee(t *testing.T) {
	tests := []struct {
		name  string
		issue map[string]any
		want  string
	}{
		{"empty", map[string]any{}, ""},
		{"no type", map[string]any{"assignee_id": "abc"}, ""},
		{"no id", map[string]any{"assignee_type": "member"}, ""},
		{"member", map[string]any{"assignee_type": "member", "assignee_id": "abcdefgh-1234"}, "member:abcdefgh"},
		{"agent", map[string]any{"assignee_type": "agent", "assignee_id": "xyz"}, "agent:xyz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAssignee(tt.issue)
			if got != tt.want {
				t.Errorf("formatAssignee() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveAssignee(t *testing.T) {
	membersResp := []map[string]any{
		{"user_id": "user-1111", "name": "Alice Smith"},
		{"user_id": "user-2222", "name": "Bob Jones"},
	}
	agentsResp := []map[string]any{
		{"id": "agent-3333", "name": "CodeBot"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/workspaces/ws-1/members":
			json.NewEncoder(w).Encode(membersResp)
		case "/api/agents":
			json.NewEncoder(w).Encode(agentsResp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := cli.NewAPIClient(srv.URL, "ws-1", "test-token")
	ctx := context.Background()

	t.Run("exact match member", func(t *testing.T) {
		aType, aID, err := resolveAssignee(ctx, client, "Alice Smith")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if aType != "member" || aID != "user-1111" {
			t.Errorf("got (%q, %q), want (member, user-1111)", aType, aID)
		}
	})

	t.Run("case-insensitive substring", func(t *testing.T) {
		aType, aID, err := resolveAssignee(ctx, client, "bob")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if aType != "member" || aID != "user-2222" {
			t.Errorf("got (%q, %q), want (member, user-2222)", aType, aID)
		}
	})

	t.Run("match agent", func(t *testing.T) {
		aType, aID, err := resolveAssignee(ctx, client, "codebot")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if aType != "agent" || aID != "agent-3333" {
			t.Errorf("got (%q, %q), want (agent, agent-3333)", aType, aID)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, _, err := resolveAssignee(ctx, client, "nobody")
		if err == nil {
			t.Fatal("expected error for no match")
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		// Both "Alice Smith" and "Bob Jones" contain a space — but let's use a broader query
		// "e" matches "Alice Smith" and "Bob Jones" and "CodeBot"
		_, _, err := resolveAssignee(ctx, client, "o")
		if err == nil {
			t.Fatal("expected error for ambiguous match")
		}
		if got := err.Error(); !strings.Contains(got, "ambiguous") {
			t.Errorf("expected ambiguous error, got: %s", got)
		}
	})

	t.Run("missing workspace ID", func(t *testing.T) {
		noWSClient := cli.NewAPIClient(srv.URL, "", "test-token")
		_, _, err := resolveAssignee(ctx, noWSClient, "alice")
		if err == nil {
			t.Fatal("expected error for missing workspace ID")
		}
	})
}

func TestUnescapeContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello world", "hello world"},
		{"literal newline", `line1\nline2`, "line1\nline2"},
		{"literal tab", `col1\tcol2`, "col1\tcol2"},
		{"multiple newlines", `a\n\nb`, "a\n\nb"},
		{"mixed", `first\nsecond\tthird`, "first\nsecond\tthird"},
		{"no escapes", "already fine\nwith real newlines", "already fine\nwith real newlines"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unescapeContent(tt.input)
			if got != tt.want {
				t.Errorf("unescapeContent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidIssueStatuses(t *testing.T) {
	expected := map[string]bool{
		"backlog":     true,
		"todo":        true,
		"in_progress": true,
		"in_review":   true,
		"done":        true,
		"blocked":     true,
		"cancelled":   true,
	}
	for _, s := range validIssueStatuses {
		if !expected[s] {
			t.Errorf("unexpected status in validIssueStatuses: %q", s)
		}
	}
	if len(validIssueStatuses) != len(expected) {
		t.Errorf("validIssueStatuses has %d entries, expected %d", len(validIssueStatuses), len(expected))
	}
}

func TestIssueCreateSendsDispatchAfter(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/issues" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		got["id"] = "issue-1"
		got["status"] = "todo"
		got["priority"] = "medium"
		json.NewEncoder(w).Encode(got)
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)

	cmd := newIssueCommandForTest("create",
		"--title", "Deferred work",
		"--dispatch-after", "2026-06-11T09:00:00Z",
		"--output", "table",
	)
	if err := runIssueCreate(cmd, nil); err != nil {
		t.Fatalf("runIssueCreate: %v", err)
	}
	if got["dispatch_after"] != "2026-06-11T09:00:00Z" {
		t.Fatalf("dispatch_after = %#v, want timestamp", got["dispatch_after"])
	}
}

func TestIssueUpdateCanClearDispatchAfter(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/issues/issue-1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":       "issue-1",
			"title":    "Deferred work",
			"status":   "todo",
			"priority": "medium",
		})
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)

	cmd := newIssueCommandForTest("update", "--clear-dispatch-after", "--output", "table")
	if err := runIssueUpdate(cmd, []string{"issue-1"}); err != nil {
		t.Fatalf("runIssueUpdate: %v", err)
	}
	if _, ok := got["dispatch_after"]; !ok {
		t.Fatal("dispatch_after not present in request body")
	}
	if got["dispatch_after"] != nil {
		t.Fatalf("dispatch_after = %#v, want nil", got["dispatch_after"])
	}
}

func TestIssueAssignSendsDispatchAfter(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/workspaces/ws-1/members":
			json.NewEncoder(w).Encode([]map[string]any{{"user_id": "member-1", "name": "Alice"}})
		case "/api/agents":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/issues/issue-1":
			if r.Method != http.MethodPut {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"id":       "issue-1",
				"title":    "Deferred work",
				"status":   "todo",
				"priority": "medium",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")

	cmd := newIssueCommandForTest("assign",
		"--to", "Alice",
		"--dispatch-after", "2026-06-11T09:00:00Z",
		"--output", "table",
	)
	if err := runIssueAssign(cmd, []string{"issue-1"}); err != nil {
		t.Fatalf("runIssueAssign: %v", err)
	}
	if got["assignee_type"] != "member" || got["assignee_id"] != "member-1" {
		t.Fatalf("assignee = (%#v, %#v), want member/member-1", got["assignee_type"], got["assignee_id"])
	}
	if got["dispatch_after"] != "2026-06-11T09:00:00Z" {
		t.Fatalf("dispatch_after = %#v, want timestamp", got["dispatch_after"])
	}
}

func newIssueCommandForTest(_ string, args ...string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("title", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("status", "", "")
	cmd.Flags().String("priority", "", "")
	cmd.Flags().String("assignee", "", "")
	cmd.Flags().String("to", "", "")
	cmd.Flags().Bool("unassign", false, "")
	cmd.Flags().String("daemon", "", "")
	cmd.Flags().String("verifier", "", "")
	cmd.Flags().Bool("clear-verifier", false, "")
	cmd.Flags().String("parent", "", "")
	cmd.Flags().String("due-date", "", "")
	cmd.Flags().String("dispatch-after", "", "")
	cmd.Flags().Bool("clear-dispatch-after", false, "")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().StringSlice("attachment", nil, "")
	if err := cmd.ParseFlags(args); err != nil {
		panic(err)
	}
	return cmd
}
