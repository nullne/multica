package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These tests cover the issue-task lookup endpoints
// (/api/issues/{id}/task-runs and /api/issues/{id}/active-task) and assert
// they behave the same way as the other /api/issues/{id}/* endpoints:
//
//   - the {id} URL param is resolved through loadIssueForUser, so callers
//     can use either the human identifier (e.g. "HAN-12") or the raw UUID
//   - the user must be a member of the issue's workspace, otherwise 404
//
// The handlers used to call parseUUID on the URL param directly, which made
// identifier URLs silently return an empty list and bypassed the workspace
// membership check.

func TestListTasksByIssue_AcceptsIdentifier(t *testing.T) {
	ctx := context.Background()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues", map[string]any{
		"title": "List tasks by identifier",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issue); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issue.ID)
	})

	if issue.Identifier == "" {
		t.Fatalf("expected non-empty identifier on issue %s", issue.ID)
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issue.Identifier+"/task-runs", nil)
	req = withURLParam(req, "id", issue.Identifier)
	testHandler.ListTasksByIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListTasksByIssue(identifier): expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var tasks []AgentTaskResponse
	if err := json.NewDecoder(w.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected empty task list for fresh issue, got %d", len(tasks))
	}
}

func TestListTasksByIssue_RejectsCrossWorkspaceUUID(t *testing.T) {
	ctx := context.Background()

	// Stand up a second workspace + issue. The handler test user is not
	// a member of this workspace, so any lookup for its issue must 404.
	var otherWorkspaceID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, issue_prefix)
		VALUES ($1, $2, $3)
		RETURNING id
	`, "Other Workspace", "other-ws-cross-test", "OTH").Scan(&otherWorkspaceID); err != nil {
		t.Fatalf("insert other workspace: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, otherWorkspaceID)
	})

	var otherUserID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, "Cross Test User", "cross-test@multica.ai").Scan(&otherUserID); err != nil {
		t.Fatalf("insert other user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, otherUserID)
	})

	if _, err := testPool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, otherWorkspaceID, otherUserID); err != nil {
		t.Fatalf("insert other membership: %v", err)
	}

	var otherIssueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, creator_type, creator_id, number)
		VALUES ($1, $2, 'member', $3, 1)
		RETURNING id
	`, otherWorkspaceID, "Other workspace issue", otherUserID).Scan(&otherIssueID); err != nil {
		t.Fatalf("insert other issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, otherIssueID)
	})

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues/"+otherIssueID+"/task-runs", nil)
	req = withURLParam(req, "id", otherIssueID)
	testHandler.ListTasksByIssue(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("ListTasksByIssue(foreign uuid): expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetActiveTaskForIssue_AcceptsIdentifier(t *testing.T) {
	ctx := context.Background()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues", map[string]any{
		"title": "Active task by identifier",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issue); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issue.ID)
	})

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issue.Identifier+"/active-task", nil)
	req = withURLParam(req, "id", issue.Identifier)
	testHandler.GetActiveTaskForIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetActiveTaskForIssue(identifier): expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["task"] != nil {
		t.Fatalf("expected task=nil for fresh issue, got %#v", body["task"])
	}
}
