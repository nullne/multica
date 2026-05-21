package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

func TestAttachmentObjectKeyIncludesWorkspaceUserAndMonthPrefix(t *testing.T) {
	key := attachmentObjectKey(
		"workspace-123",
		"user-456",
		"Design Notes.PDF",
		"abcdef0123456789",
		time.Date(2026, 5, 21, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60)),
	)

	want := "workspaces/workspace-123/users/user-456/attachments/2026/05/abcdef0123456789.PDF"
	if key != want {
		t.Fatalf("attachmentObjectKey() = %q, want %q", key, want)
	}
}

// TestAttachmentDownloadURLUsesProxyPath verifies that the attachment payload
// exposes a download_url pointing at the in-app proxy (/api/attachments/{id}/download)
// when CloudFront signing is not configured. This is the path agents fetch
// over Multica bearer auth, so it must not leak the raw bucket URL.
func TestAttachmentDownloadURLUsesProxyPath(t *testing.T) {
	ctx := context.Background()

	// Create an attachment row directly so we don't depend on Storage being set.
	att, err := testHandler.Queries.CreateAttachment(ctx, db.CreateAttachmentParams{
		WorkspaceID:  parseUUID(testWorkspaceID),
		UploaderType: "member",
		UploaderID:   parseUUID(testUserID),
		Filename:     "report.pdf",
		Url:          "https://storage.googleapis.com/test-bucket/abc.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    1024,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM attachment WHERE id = $1`, att.ID)
	})

	req := httptest.NewRequest("GET", "https://multica.example/api/issues", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp := testHandler.attachmentToResponse(req, att)

	wantPath := "/api/attachments/" + uuidToString(att.ID) + "/download"
	if !strings.HasSuffix(resp.DownloadURL, wantPath) {
		t.Fatalf("download_url = %q, want suffix %q", resp.DownloadURL, wantPath)
	}
	if !strings.HasPrefix(resp.DownloadURL, "https://") {
		t.Fatalf("download_url should be absolute https URL, got %q", resp.DownloadURL)
	}
	if resp.URL != "https://storage.googleapis.com/test-bucket/abc.pdf" {
		t.Fatalf("url should retain raw bucket URL, got %q", resp.URL)
	}
}

func TestAttachmentDownloadURLUsesHTTPSInProductionBehindProxy(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	ctx := context.Background()

	att, err := testHandler.Queries.CreateAttachment(ctx, db.CreateAttachmentParams{
		WorkspaceID:  parseUUID(testWorkspaceID),
		UploaderType: "member",
		UploaderID:   parseUUID(testUserID),
		Filename:     "report.pdf",
		Url:          "https://storage.googleapis.com/test-bucket/abc.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    1024,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM attachment WHERE id = $1`, att.ID)
	})

	req := httptest.NewRequest("GET", "http://multica.example/api/issues", nil)
	resp := testHandler.attachmentToResponse(req, att)

	if !strings.HasPrefix(resp.DownloadURL, "https://multica.example/") {
		t.Fatalf("download_url should use https in production, got %q", resp.DownloadURL)
	}
}

// TestDownloadAttachmentNotFound verifies the endpoint returns 404 for an
// unknown attachment ID rather than leaking storage errors.
func TestDownloadAttachmentNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/attachments/00000000-0000-0000-0000-000000000000/download", nil)
	req = withURLParam(req, "id", "00000000-0000-0000-0000-000000000000")
	testHandler.DownloadAttachment(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("DownloadAttachment for unknown id: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDownloadAttachmentForeignWorkspace verifies that an authenticated user
// cannot download attachments from a workspace they are not a member of —
// the handler must return 404 rather than streaming bytes or leaking the
// existence of the attachment.
func TestDownloadAttachmentForeignWorkspace(t *testing.T) {
	ctx := context.Background()

	// Build a workspace the test user is NOT a member of.
	var foreignWorkspaceID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ('foreign', 'foreign-download-test', '', 'FOR')
		RETURNING id
	`).Scan(&foreignWorkspaceID); err != nil {
		t.Fatalf("create foreign workspace: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, foreignWorkspaceID)
	})

	// Seed an attachment in that foreign workspace (uploader is fine — we
	// just need a row that exists).
	att, err := testHandler.Queries.CreateAttachment(ctx, db.CreateAttachmentParams{
		WorkspaceID:  parseUUID(foreignWorkspaceID),
		UploaderType: "member",
		UploaderID:   parseUUID(testUserID),
		Filename:     "secret.pdf",
		Url:          "https://cdn.example/secret.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    16,
	})
	if err != nil {
		t.Fatalf("create foreign attachment: %v", err)
	}

	id := uuidToString(att.ID)
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/attachments/"+id+"/download", nil)
	req = withURLParam(req, "id", id)
	testHandler.DownloadAttachment(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("DownloadAttachment for foreign workspace: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreateIssueLinksAttachmentsBeforeDispatch is the regression test for the
// race in `multica issue create --attachment ... --assignee <agent>`: the
// attachment ID(s) supplied in the request must be linked to the new issue
// before the agent task is enqueued, so the daemon never claims a task whose
// attachments are still floating loose.
func TestCreateIssueLinksAttachmentsBeforeDispatch(t *testing.T) {
	ctx := context.Background()

	// Spin up an agent with an online runtime so the task can be enqueued
	// without rejection.
	var agentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, description, visibility, owner_id, tools, triggers, providers)
		VALUES ($1, $2, '', 'workspace', $3, '[]'::jsonb, '[]'::jsonb, ARRAY['codex'])
		RETURNING id
	`, testWorkspaceID, "attachment-dispatch-agent", testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)
		testPool.Exec(ctx, `DELETE FROM agent WHERE id = $1`, agentID)
	})

	// Pre-upload an attachment (issue_id and comment_id NULL) — this mirrors
	// what the CLI does now: upload first, then create.
	att, err := testHandler.Queries.CreateAttachment(ctx, db.CreateAttachmentParams{
		WorkspaceID:  parseUUID(testWorkspaceID),
		UploaderType: "member",
		UploaderID:   parseUUID(testUserID),
		Filename:     "input.txt",
		Url:          "https://cdn.example/test-input.txt",
		ContentType:  "text/plain",
		SizeBytes:    9,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	attachmentID := uuidToString(att.ID)
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM attachment WHERE id = $1`, att.ID)
	})

	// Create an issue with attachment_ids set and the agent as assignee.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":          "Attachment dispatch ordering test",
		"status":         "todo",
		"assignee_type":  "agent",
		"assignee_id":    agentID,
		"attachment_ids": []string{attachmentID},
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp IssueResponse
	json.NewDecoder(w.Body).Decode(&resp)
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, resp.ID) })

	// The response should already include the linked attachment.
	if len(resp.Attachments) != 1 || resp.Attachments[0].ID != attachmentID {
		t.Fatalf("issue response attachments = %+v, want one attachment %s", resp.Attachments, attachmentID)
	}

	// And the attachment row must now point at the new issue.
	got, err := testHandler.Queries.GetAttachment(ctx, db.GetAttachmentParams{
		ID:          att.ID,
		WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("reload attachment: %v", err)
	}
	if !got.IssueID.Valid || uuidToString(got.IssueID) != resp.ID {
		t.Fatalf("attachment.issue_id = %v, want %s", got.IssueID, resp.ID)
	}

	// And a queued task must exist for the assignee agent on this issue —
	// proving the link happened *before* dispatch, since the handler returns
	// after enqueueing.
	var taskCount int
	if err := testPool.QueryRow(ctx,
		`SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2`,
		resp.ID, agentID,
	).Scan(&taskCount); err != nil {
		t.Fatalf("query task queue: %v", err)
	}
	if taskCount != 1 {
		t.Fatalf("expected 1 queued task for new issue, got %d", taskCount)
	}
}

// TestLinkAttachmentsToIssueDoesNotStealCommentAttachments verifies the
// "unlinked only" guard: attachment_ids passed at issue-create time cannot
// steal attachments that are already bound to a comment (or another issue).
func TestLinkAttachmentsToIssueDoesNotStealCommentAttachments(t *testing.T) {
	ctx := context.Background()
	wsUUID := parseUUID(testWorkspaceID)

	// Create a host issue + comment, attach to the comment.
	hostIssue, err := testHandler.Queries.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID: wsUUID,
		Title:       "host issue for link guard",
		Status:      "todo",
		Priority:    "none",
		CreatorType: "member",
		CreatorID:   parseUUID(testUserID),
		Number:      9999001,
	})
	if err != nil {
		t.Fatalf("create host issue: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, hostIssue.ID) })

	hostComment, err := testHandler.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     hostIssue.ID,
		WorkspaceID: wsUUID,
		AuthorType:  "member",
		AuthorID:    parseUUID(testUserID),
		Content:     "host",
		Type:        "comment",
	})
	if err != nil {
		t.Fatalf("create host comment: %v", err)
	}

	att, err := testHandler.Queries.CreateAttachment(ctx, db.CreateAttachmentParams{
		WorkspaceID:  wsUUID,
		UploaderType: "member",
		UploaderID:   parseUUID(testUserID),
		Filename:     "owned-by-comment.txt",
		Url:          "https://cdn.example/owned.txt",
		ContentType:  "text/plain",
		SizeBytes:    1,
		IssueID:      hostIssue.ID,
		CommentID:    hostComment.ID,
	})
	if err != nil {
		t.Fatalf("create bound attachment: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM attachment WHERE id = $1`, att.ID) })

	// Try to attach this comment-owned attachment to a brand new issue.
	w := httptest.NewRecorder()
	body := map[string]any{
		"title":          "tries to steal",
		"status":         "todo",
		"attachment_ids": []string{uuidToString(att.ID)},
	}
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, body)
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp IssueResponse
	json.NewDecoder(w.Body).Decode(&resp)
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, resp.ID) })

	// The attachment must still belong to the original comment.
	reloaded, err := testHandler.Queries.GetAttachment(ctx, db.GetAttachmentParams{
		ID:          att.ID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		t.Fatalf("reload attachment: %v", err)
	}
	if !reloaded.CommentID.Valid || !bytes.Equal(uuidBytes(reloaded.CommentID), uuidBytes(hostComment.ID)) {
		t.Fatalf("attachment.comment_id changed: got %v, want %v", reloaded.CommentID, hostComment.ID)
	}
	if !reloaded.IssueID.Valid || !bytes.Equal(uuidBytes(reloaded.IssueID), uuidBytes(hostIssue.ID)) {
		t.Fatalf("attachment.issue_id changed: got %v, want %v", reloaded.IssueID, hostIssue.ID)
	}
}

// uuidBytes copies the raw 16 bytes from a pgtype.UUID for use in equality
// checks; we avoid pulling the pgtype dependency into tests just for cmp.
func uuidBytes(u pgtype.UUID) []byte {
	out := make([]byte, 16)
	copy(out, u.Bytes[:])
	return out
}
