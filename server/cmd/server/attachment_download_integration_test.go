package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// TestAttachmentDownloadURLIsReachableThroughRouter is a regression test for
// PR #163 review feedback: the `download_url` we hand back to clients must be
// reachable using just JWT auth, without a workspace_id query param or
// X-Workspace-ID header. Earlier the route lived under RequireWorkspaceMember,
// so callers following download_url got a 400 ("workspace_id is required")
// before the handler ever ran. The fix moved the route to the user-scoped
// group so the handler resolves the workspace from the attachment row itself.
func TestAttachmentDownloadURLIsReachableThroughRouter(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)

	// Seed an attachment row in the test workspace. The bucket URL is fake —
	// we only care about reaching the handler, not what it eventually serves.
	att, err := queries.CreateAttachment(ctx, db.CreateAttachmentParams{
		WorkspaceID:  pgUUID(testWorkspaceID),
		UploaderType: "member",
		UploaderID:   pgUUID(testUserID),
		Filename:     "router-test.txt",
		Url:          "https://storage.googleapis.com/private-bucket/router-test.txt",
		ContentType:  "text/plain",
		SizeBytes:    4,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM attachment WHERE id = $1`, att.ID)
	})

	attID := uuidString(att.ID)
	downloadPath := "/api/attachments/" + attID + "/download"

	// Issue the request with auth ONLY — no X-Workspace-ID header, no
	// workspace_id query param. This mirrors what an agent that just read
	// `download_url` off an AttachmentResponse will do.
	req, err := http.NewRequest(http.MethodGet, testServer.URL+downloadPath, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	// Don't follow the redirect; in the no-CloudFront test env we either get
	// a streamed response or a storage error — but never the 400 we used to.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// The middleware regression manifested as 400 with body
	// `{"error":"workspace_id is required"}`. Anything else means we got past
	// it. The test storage backend isn't configured to actually serve the
	// fake bucket URL, so we expect a downstream error (502 from the proxy or
	// 503 when storage is nil) rather than 200 — both are fine.
	if resp.StatusCode == http.StatusBadRequest {
		var body map[string]string
		json.NewDecoder(resp.Body).Decode(&body)
		t.Fatalf("download endpoint returned 400 (workspace middleware regression): %v", body)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Fatalf("download endpoint rejected authenticated workspace member with %d", resp.StatusCode)
	}
}

// TestAttachmentDownloadRejectsUnauthenticated verifies the new route is
// still behind the auth middleware — anonymous requests must not reach the
// handler.
func TestAttachmentDownloadRejectsUnauthenticated(t *testing.T) {
	resp, err := http.Get(testServer.URL + "/api/attachments/00000000-0000-0000-0000-000000000000/download")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous download: expected 401, got %d", resp.StatusCode)
	}
}

// TestAttachmentResponseDownloadURLDoesNotLeakRawBucket sanity-checks that
// AttachmentResponse.download_url returned by /api/issues/{id} for a
// freshly-attached file points at our proxy path, not the raw bucket URL —
// the user-facing piece of the bug from #162.
func TestAttachmentResponseDownloadURLDoesNotLeakRawBucket(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)

	// Create an issue + attachment bound to it.
	resp := authRequest(t, http.MethodPost, "/api/issues", map[string]any{
		"title":  "download_url regression test issue",
		"status": "todo",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create issue: %d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	readJSON(t, resp, &created)
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, created.ID) })

	att, err := queries.CreateAttachment(ctx, db.CreateAttachmentParams{
		WorkspaceID:  pgUUID(testWorkspaceID),
		UploaderType: "member",
		UploaderID:   pgUUID(testUserID),
		IssueID:      pgUUID(created.ID),
		Filename:     "input.bin",
		Url:          "https://storage.googleapis.com/private-bucket/input.bin",
		ContentType:  "application/octet-stream",
		SizeBytes:    16,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM attachment WHERE id = $1`, att.ID) })

	// Fetch the issue and inspect the embedded attachment payload.
	resp = authRequest(t, http.MethodGet, "/api/issues/"+created.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get issue: %d", resp.StatusCode)
	}
	var got struct {
		Attachments []struct {
			URL         string `json:"url"`
			DownloadURL string `json:"download_url"`
		} `json:"attachments"`
	}
	readJSON(t, resp, &got)
	if len(got.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(got.Attachments))
	}

	a := got.Attachments[0]
	if a.URL == "" {
		t.Fatalf("attachment.url empty")
	}
	if !strings.Contains(a.DownloadURL, "/api/attachments/") || !strings.HasSuffix(a.DownloadURL, "/download") {
		t.Fatalf("download_url should route through the app proxy, got %q", a.DownloadURL)
	}
}

// pgUUID parses a string UUID into pgtype.UUID for the integration tests
// (the main test file lives in a different package and we don't want to
// import its private helpers).
func pgUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}
	}
	return u
}

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	v, err := u.Value()
	if err != nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
