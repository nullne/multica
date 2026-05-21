package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// seedAgentTaskForResultCommentTest creates an agent and an issue assigned
// to it. The CreateIssue handler auto-enqueues a task for any agent-assigned
// issue, so we read that task back instead of creating a second one (which
// would trip idx_one_pending_task_per_issue). Returns agent ID, issue ID,
// and task ID. The caller's t.Cleanup tears everything down.
func seedAgentTaskForResultCommentTest(t *testing.T, ctx context.Context, agentName string) (string, string, string) {
	t.Helper()

	var agentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, description, visibility, owner_id, tools, triggers, providers)
		VALUES ($1, $2, '', 'workspace', $3, '[]'::jsonb, '[]'::jsonb, ARRAY['codex'])
		RETURNING id
	`, testWorkspaceID, agentName, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "result-comment-link test issue " + agentName,
		"status":        "in_progress",
		"assignee_type": "agent",
		"assignee_id":   agentID,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != 201 {
		t.Fatalf("create issue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issueResp IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issueResp); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}
	issueID := issueResp.ID

	// Pick up the task that CreateIssue auto-enqueued for the agent.
	var taskID string
	if err := testPool.QueryRow(ctx,
		`SELECT id::text FROM agent_task_queue WHERE issue_id = $1 ORDER BY created_at DESC LIMIT 1`,
		issueID,
	).Scan(&taskID); err != nil {
		t.Fatalf("read auto-enqueued task: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
		testPool.Exec(ctx, `DELETE FROM agent WHERE id = $1`, agentID)
	})

	return agentID, issueID, taskID
}

// fetchTaskResultCommentID reads result_comment_id straight from the DB
// (bypasses the API to assert what was persisted).
func fetchTaskResultCommentID(t *testing.T, ctx context.Context, taskID string) *string {
	t.Helper()
	var resultID *string
	if err := testPool.QueryRow(ctx,
		`SELECT result_comment_id::text FROM agent_task_queue WHERE id = $1`, taskID,
	).Scan(&resultID); err != nil {
		t.Fatalf("read result_comment_id: %v", err)
	}
	return resultID
}

func TestCreateComment_AgentAuthorWithTaskID_LinksResultComment(t *testing.T) {
	ctx := context.Background()
	agentID, issueID, taskID := seedAgentTaskForResultCommentTest(t, ctx, "result-comment-link-happy")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "Done.",
		"task_id": taskID,
	})
	req.Header.Set("X-Agent-ID", agentID)
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp CommentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode comment response: %v", err)
	}

	got := fetchTaskResultCommentID(t, ctx, taskID)
	if got == nil {
		t.Fatalf("expected result_comment_id to be set after agent comment with task_id, got nil")
	}
	if *got != resp.ID {
		t.Fatalf("result_comment_id = %s, want %s", *got, resp.ID)
	}
}

func TestCreateComment_MemberAuthorWithTaskID_IgnoresTaskID(t *testing.T) {
	ctx := context.Background()
	_, issueID, taskID := seedAgentTaskForResultCommentTest(t, ctx, "result-comment-link-member")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "Hi.",
		"task_id": taskID,
	})
	// No X-Agent-ID — handler resolves the authenticated member as author.
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if got := fetchTaskResultCommentID(t, ctx, taskID); got != nil {
		t.Fatalf("expected result_comment_id to remain NULL for member-authored comment, got %q", *got)
	}
}

func TestCreateComment_AgentAuthorWithMismatchedTaskID_IgnoresTaskID(t *testing.T) {
	ctx := context.Background()
	agentA, issueA, taskA := seedAgentTaskForResultCommentTest(t, ctx, "result-comment-link-mismatch-a")
	_ = agentA
	_ = issueA
	// A different agent (agent B) tries to claim taskA as its result. Should be ignored.
	agentB, issueB, _ := seedAgentTaskForResultCommentTest(t, ctx, "result-comment-link-mismatch-b")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueB+"/comments", map[string]any{
		"content": "Wrong task link.",
		"task_id": taskA,
	})
	req.Header.Set("X-Agent-ID", agentB)
	req = withURLParam(req, "id", issueB)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if got := fetchTaskResultCommentID(t, ctx, taskA); got != nil {
		t.Fatalf("expected result_comment_id to remain NULL when agent_id mismatches, got %q", *got)
	}
}

func TestCreateComment_SystemTypeWithTaskID_IgnoresTaskID(t *testing.T) {
	ctx := context.Background()
	agentID, issueID, taskID := seedAgentTaskForResultCommentTest(t, ctx, "result-comment-link-system")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "System annotation.",
		"type":    "system",
		"task_id": taskID,
	})
	req.Header.Set("X-Agent-ID", agentID)
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if got := fetchTaskResultCommentID(t, ctx, taskID); got != nil {
		t.Fatalf("expected result_comment_id to remain NULL for system-type comment, got %q", *got)
	}
}
