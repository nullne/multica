package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"
)

// createTestIssue creates a test issue and returns its ID.
// Cleanup is registered via t.Cleanup.
func createTestIssue(t *testing.T, title string) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":    title,
		"status":   "todo",
		"priority": "low",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != 201 {
		t.Fatalf("createTestIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp IssueResponse
	json.NewDecoder(w.Body).Decode(&resp)
	t.Cleanup(func() {
		wr := httptest.NewRecorder()
		r := newRequest("DELETE", "/api/issues/"+resp.ID, nil)
		r = withURLParam(r, "id", resp.ID)
		testHandler.DeleteIssue(wr, r)
	})
	return resp.ID
}

// createTestAgent inserts an agent directly into the DB with the given attributes
// and returns its UUID string. Cleanup is registered via t.Cleanup.
func createTestAgent(t *testing.T, name string, providers []string, archived bool, triggerJSON string) string {
	t.Helper()
	ctx := context.Background()

	var archivedAt *time.Time
	if archived {
		now := time.Now()
		archivedAt = &now
	}

	var id string
	var err error
	var defaultProvider any
	var defaultDaemonID any
	if len(providers) > 0 {
		defaultProvider = providers[0]
		var daemonID string
		if qerr := testPool.QueryRow(ctx, `
			SELECT daemon_ref
			FROM agent_runtime
			WHERE workspace_id = $1 AND provider = $2 AND daemon_ref IS NOT NULL
			LIMIT 1
		`, testWorkspaceID, providers[0]).Scan(&daemonID); qerr == nil {
			defaultDaemonID = daemonID
		}
	}
	if archivedAt != nil {
		err = testPool.QueryRow(ctx, `
			INSERT INTO agent (workspace_id, name, description, visibility, owner_id, tools, triggers, providers, default_provider, default_daemon_id, archived_at)
			VALUES ($1, $2, '', 'workspace', $3, '[]'::jsonb, $4::jsonb, $5, $6, $7, $8)
			RETURNING id
		`, testWorkspaceID, name, testUserID, triggerJSON, providers, defaultProvider, defaultDaemonID, archivedAt).Scan(&id)
	} else {
		err = testPool.QueryRow(ctx, `
			INSERT INTO agent (workspace_id, name, description, visibility, owner_id, tools, triggers, providers, default_provider, default_daemon_id)
			VALUES ($1, $2, '', 'workspace', $3, '[]'::jsonb, $4::jsonb, $5, $6, $7)
			RETURNING id
		`, testWorkspaceID, name, testUserID, triggerJSON, providers, defaultProvider, defaultDaemonID).Scan(&id)
	}
	if err != nil {
		t.Fatalf("createTestAgent(%s): %v", name, err)
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, id)
	})
	return id
}

func clearAgentDispatchDefaults(t *testing.T, agentID string) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent
		SET default_provider = NULL, default_daemon_id = NULL
		WHERE id = $1
	`, agentID); err != nil {
		t.Fatalf("clearAgentDispatchDefaults(%s): %v", agentID, err)
	}
}

// listSystemNoticesForIssue returns all system-authored comments for the given issue.
func listSystemNoticesForIssue(t *testing.T, issueID string) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := testPool.Query(ctx, `
		SELECT content FROM comment
		WHERE issue_id = $1 AND author_type = 'system'
		ORDER BY created_at
	`, issueID)
	if err != nil {
		t.Fatalf("listSystemNoticesForIssue: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var content string
		rows.Scan(&content)
		out = append(out, content)
	}
	return out
}

func TestMentionDispatch_SuccessfulDispatch_NoSystemNotice(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: success test")

	// Create an agent with on_mention enabled, with an online runtime (set up in TestMain).
	// The global test runtime uses provider "codex", so create an agent with codex.
	triggerJSON := `[{"type":"on_mention","enabled":true}]`
	agentID := createTestAgent(t, "mention-success-agent", []string{"codex"}, false, triggerJSON)

	// Post a comment mentioning this agent.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@TestAgent](mention://agent/%s) please help", agentID),
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) != 0 {
		t.Errorf("expected no system notices for successful dispatch, got %d: %v", len(notices), notices)
	}
}

func TestMentionDispatch_OnMentionDisabled_PostsNotice(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: on_mention disabled test")

	triggerJSON := `[{"type":"on_mention","enabled":false}]`
	agentID := createTestAgent(t, "mention-disabled-agent", []string{"codex"}, false, triggerJSON)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@DisabledAgent](mention://agent/%s) please help", agentID),
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) == 0 {
		t.Fatal("expected a system notice for disabled on_mention, got none")
	}
	found := false
	for _, n := range notices {
		if containsAll(n, "mention trigger disabled") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected notice about disabled trigger, got: %v", notices)
	}
}

func TestMentionDispatch_NoProviders_PostsNotice(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: no providers test")

	triggerJSON := `[{"type":"on_mention","enabled":true}]`
	agentID := createTestAgent(t, "mention-noprovider-agent", []string{} /* no providers */, false, triggerJSON)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@NoProviderAgent](mention://agent/%s) please help", agentID),
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) == 0 {
		t.Fatal("expected a system notice for agent with no providers, got none")
	}
	found := false
	for _, n := range notices {
		if containsAll(n, "no AI providers") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected notice about missing providers, got: %v", notices)
	}
}

func TestMentionDispatch_ArchivedAgent_PostsNotice(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: archived agent test")

	triggerJSON := `[{"type":"on_mention","enabled":true}]`
	agentID := createTestAgent(t, "mention-archived-agent", []string{"codex"}, true /* archived */, triggerJSON)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@ArchivedAgent](mention://agent/%s) please help", agentID),
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) == 0 {
		t.Fatal("expected a system notice for archived agent, got none")
	}
	found := false
	for _, n := range notices {
		if containsAll(n, "archived") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected notice about archived agent, got: %v", notices)
	}
}

func TestMentionDispatch_IncompleteDefaults_PostsNotice(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: incomplete defaults test")

	triggerJSON := `[{"type":"on_mention","enabled":true}]`
	agentID := createTestAgent(t, "mention-incomplete-defaults-agent", []string{"codex"}, false, triggerJSON)
	clearAgentDispatchDefaults(t, agentID)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@IncompleteAgent](mention://agent/%s) please help", agentID),
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) == 0 {
		t.Fatal("expected a system notice for incomplete defaults, got none")
	}
	found := false
	for _, n := range notices {
		if containsAll(n, "missing complete dispatch defaults") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected notice about incomplete defaults, got: %v", notices)
	}
}

func TestMentionDispatch_DuplicatePendingTask_PostsNotice(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: duplicate pending task test")

	triggerJSON := `[{"type":"on_mention","enabled":true}]`
	agentID := createTestAgent(t, "mention-dupetask-agent", []string{"codex"}, false, triggerJSON)

	// Post the first mention — this should succeed (task enqueued).
	commentContent := fmt.Sprintf("[@Agent](mention://agent/%s) first request", agentID)
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": commentContent,
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment (first): expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Post the second mention — the agent now has a pending task.
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@Agent](mention://agent/%s) second request", agentID),
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment (second): expected 201, got %d: %s", w.Code, w.Body.String())
	}

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) == 0 {
		t.Fatal("expected a system notice for duplicate pending task, got none")
	}
	found := false
	for _, n := range notices {
		if containsAll(n, "already has a pending task") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected notice about pending task, got: %v", notices)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
