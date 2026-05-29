package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/auth"
	gh "github.com/nullne/multica/server/internal/github"
	wh "github.com/nullne/multica/server/internal/webhook"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// signGitHubBody computes the X-Hub-Signature-256 header that GitHub would
// send for the given body and shared secret.
func signGitHubBody(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// setupGitHubWebhookFixture installs a source_type='github' webhook bound to
// the test workspace and returns its ID together with the installation_id.
func setupGitHubWebhookFixture(t *testing.T, installationID int64) pgtype.UUID {
	t.Helper()
	ctx := context.Background()

	rawToken, err := wh.GenerateToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	prefix := rawToken
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}

	webhook, err := testHandler.Queries.CreateWebhook(ctx, db.CreateWebhookParams{
		WorkspaceID:        parseUUID(testWorkspaceID),
		Name:               "GitHub App Test",
		SourceType:         "github",
		TokenHash:          auth.HashToken(rawToken),
		TokenPrefix:        prefix,
		DedupWindowSeconds: 600,
		CreatedBy:          parseUUID(testUserID),
		InstallationID:     pgtype.Int8{Int64: installationID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create webhook: %v", err)
	}
	t.Cleanup(func() {
		testHandler.Queries.DeleteWebhook(ctx, webhook.ID)
	})
	return webhook.ID
}

func TestReceiveGitHubEvent_RejectsBadSignature(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	// Provision a real GitHubApp so VerifySignature fails on bad sig (rather than
	// failing because no webhook secret is configured).
	app, err := gh.NewApp(12345, testRSAKeyPEM)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.SetWebhookSecret([]byte("secret"))

	prev := testHandler.GitHubApp
	testHandler.GitHubApp = app
	t.Cleanup(func() { testHandler.GitHubApp = prev })

	body := []byte(`{"installation":{"id":1}}`)
	req := httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	w := httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad signature, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReceiveGitHubEvent_PingShortCircuits(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}
	secret := []byte("test-secret")
	app, err := gh.NewApp(12345, testRSAKeyPEM)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.SetWebhookSecret(secret)
	prev := testHandler.GitHubApp
	testHandler.GitHubApp = app
	t.Cleanup(func() { testHandler.GitHubApp = prev })

	body := []byte(`{"zen":"keep it logically awesome"}`)
	req := httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
	w := httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ping: expected 200, got %d", w.Code)
	}
}

func TestReceiveGitHubEvent_UnknownInstallation(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}
	secret := []byte("test-secret")
	app, err := gh.NewApp(12345, testRSAKeyPEM)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.SetWebhookSecret(secret)
	prev := testHandler.GitHubApp
	testHandler.GitHubApp = app
	t.Cleanup(func() { testHandler.GitHubApp = prev })

	// installation_id 99999 has no webhook → handler returns 200 (silent drop)
	body := []byte(`{"action":"opened","installation":{"id":99999},"pull_request":{"number":1,"html_url":"https://x"},"repository":{"full_name":"a/b"}}`)
	req := httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
	w := httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unknown installation: expected 200, got %d", w.Code)
	}
}

func TestReceiveGitHubEvent_PullRequestCreatesIssueAndLink(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}
	ctx := context.Background()
	secret := []byte("test-secret")

	app, err := gh.NewApp(12345, testRSAKeyPEM)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.SetWebhookSecret(secret)
	prev := testHandler.GitHubApp
	testHandler.GitHubApp = app
	t.Cleanup(func() { testHandler.GitHubApp = prev })

	installationID := int64(424242)
	webhookID := setupGitHubWebhookFixture(t, installationID)

	// Look up the test agent to wire it as create_issue assignee.
	var agentIDStr string
	if err := testPool.QueryRow(ctx,
		`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "Handler Test Agent",
	).Scan(&agentIDStr); err != nil {
		t.Fatalf("find agent: %v", err)
	}

	cfg := CreateIssueActionConfig{
		AgentID:       agentIDStr,
		TitleTemplate: "PR: {{.title}}",
	}
	cfgJSON, _ := json.Marshal(cfg)
	if _, err := testHandler.Queries.CreateWebhookAction(ctx, db.CreateWebhookActionParams{
		WebhookID:  webhookID,
		ActionType: "create_issue",
		Config:     cfgJSON,
		Enabled:    true,
		Position:   0,
	}); err != nil {
		t.Fatalf("create action: %v", err)
	}

	prURL := fmt.Sprintf("https://github.com/acme/widgets/pull/%d", uniqueIssueNumber())
	body := []byte(fmt.Sprintf(`{
		"action":"opened",
		"installation":{"id":%d},
		"pull_request":{"number":777,"title":"Fix bug","html_url":%q,"user":{"login":"alice"},"head":{"ref":"feat"},"base":{"ref":"main"},"body":"x"},
		"repository":{"full_name":"acme/widgets"}
	}`, installationID, prURL))

	req := httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
	w := httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("PR opened: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	// Verify exactly one issue_link with the PR URL was created in this workspace.
	link, err := testHandler.Queries.GetIssueLinkByURL(ctx, db.GetIssueLinkByURLParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Url:         prURL,
	})
	if err != nil {
		t.Fatalf("issue_link not found: %v", err)
	}
	if link.Direction != "source" {
		t.Errorf("link.direction = %q, want source", link.Direction)
	}
	if link.Kind != "pr" {
		t.Errorf("link.kind = %q, want pr", link.Kind)
	}
	if link.SourceType != "github" {
		t.Errorf("link.source_type = %q, want github", link.SourceType)
	}

	// Cleanup.
	t.Cleanup(func() {
		testHandler.Queries.DeleteIssue(ctx, link.IssueID)
		testHandler.Queries.DeleteIssueLink(ctx, link.ID)
	})
}

func TestReceiveGitHubEvent_AutoFixIssueSwitchControlsBotComment(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}
	ctx := context.Background()
	secret := []byte("test-secret")

	app, err := gh.NewApp(12345, testRSAKeyPEM)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.SetWebhookSecret(secret)
	prev := testHandler.GitHubApp
	testHandler.GitHubApp = app
	t.Cleanup(func() { testHandler.GitHubApp = prev })

	installationID := time.Now().UnixNano()
	webhookID := setupGitHubWebhookFixture(t, installationID)

	var botID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO "user" (name, email, kind)
		VALUES ('GitHub', $1, 'bot')
		RETURNING id
	`, fmt.Sprintf("github-bot-%d@multica.test", installationID)).Scan(&botID); err != nil {
		t.Fatalf("create github bot user: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, botID) })
	if _, err := testPool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'member')
	`, testWorkspaceID, botID); err != nil {
		t.Fatalf("add bot member: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE webhook SET bot_user_id = $1 WHERE id = $2
	`, botID, uuidToString(webhookID)); err != nil {
		t.Fatalf("bind github bot: %v", err)
	}

	var agentID string
	if err := testPool.QueryRow(ctx, `
		SELECT id::text FROM agent WHERE workspace_id = $1 AND name = 'Handler Test Agent'
	`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}

	enabledPRNumber := uniqueIssueNumber()
	enabledPR := fmt.Sprintf("https://github.com/acme/widgets/pull/%d", enabledPRNumber)
	disabledPR := fmt.Sprintf("https://github.com/acme/widgets/pull/%d", uniqueIssueNumber())
	enabledIssueID := insertLinkedAutoFixIssue(t, "Auto-fix enabled issue", agentID, enabledPR, true)
	enabledIssueID2 := insertLinkedAutoFixIssue(t, "Second auto-fix enabled issue", agentID, enabledPR, true)
	disabledIssueID := insertLinkedAutoFixIssue(t, "Auto-fix disabled issue", agentID, disabledPR, false)
	var enabledLinkCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM issue_link
		WHERE workspace_id = $1 AND url = $2
	`, testWorkspaceID, enabledPR).Scan(&enabledLinkCount); err != nil {
		t.Fatalf("count enabled PR links: %v", err)
	}
	if enabledLinkCount != 2 {
		t.Fatalf("expected same PR URL to be linked to two issues, got %d links", enabledLinkCount)
	}

	sendComment := func(prURL string, commentURL string, bodyText string) {
		t.Helper()
		body := []byte(fmt.Sprintf(`{
			"action":"created",
			"installation":{"id":%d},
			"comment":{"body":%q,"html_url":%q,"user":{"login":"reviewer"}},
			"issue":{"number":123,"title":"Review feedback","html_url":%q,"pull_request":{"html_url":%q}},
			"repository":{"full_name":"acme/widgets"}
		}`, installationID, bodyText, commentURL, prURL, prURL))
		req := httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
		req.Header.Set("X-GitHub-Event", "issue_comment")
		req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
		w := httptest.NewRecorder()
		testHandler.ReceiveGitHubEvent(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("issue_comment: expected 202, got %d: %s", w.Code, w.Body.String())
		}
	}

	sendComment(enabledPR, enabledPR+"#issuecomment-1", "Please fix the failing assertion")
	sendComment(enabledPR, enabledPR+"#issuecomment-2", "Please update the tooltip copy")
	sendComment(disabledPR, disabledPR+"#issuecomment-1", "This should not notify the issue")
	sendReviewComment := func(prURL string, bodyText string) {
		t.Helper()
		body := []byte(fmt.Sprintf(`{
			"action":"created",
			"installation":{"id":%d},
			"comment":{"body":%q,"html_url":"%s#discussion_r1","path":"server/main.go","user":{"login":"reviewer"}},
			"pull_request":{"number":123,"title":"Review feedback","html_url":%q},
			"repository":{"full_name":"acme/widgets"}
		}`, installationID, bodyText, prURL, prURL))
		req := httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
		req.Header.Set("X-GitHub-Event", "pull_request_review_comment")
		req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
		w := httptest.NewRecorder()
		testHandler.ReceiveGitHubEvent(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("pull_request_review_comment: expected 202, got %d: %s", w.Code, w.Body.String())
		}
	}

	var enabledCommentID string
	var enabledContent string
	if err := testPool.QueryRow(ctx, `
		SELECT id::text, content FROM comment
		WHERE issue_id = $1 AND author_id = $2 AND author_type = 'member'
		  AND content LIKE '%Please fix the failing assertion%'
		ORDER BY created_at ASC LIMIT 1
	`, enabledIssueID, botID).Scan(&enabledCommentID, &enabledContent); err != nil {
		t.Fatalf("expected bot comment on enabled issue: %v", err)
	}
	if enabledContent == "" || !bytes.Contains([]byte(enabledContent), []byte("Please fix the failing assertion")) {
		t.Fatalf("bot comment content = %q, want GitHub feedback body", enabledContent)
	}

	var enabledTasks int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM agent_task_queue
		WHERE issue_id = $1 AND agent_id = $2 AND trigger_comment_id = $3
	`, enabledIssueID, agentID, enabledCommentID).Scan(&enabledTasks); err != nil {
		t.Fatalf("count enabled tasks: %v", err)
	}
	if enabledTasks != 1 {
		t.Fatalf("expected bot comment to enqueue one assignee task, got %d", enabledTasks)
	}
	var enabledCommentMentions int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM comment
		WHERE issue_id = $1 AND author_id = $2
		  AND (
		    content LIKE '%Please fix the failing assertion%'
		    OR content LIKE '%Please update the tooltip copy%'
		  )
	`, enabledIssueID, botID).Scan(&enabledCommentMentions); err != nil {
		t.Fatalf("count enabled issue comment syncs: %v", err)
	}
	if enabledCommentMentions != 2 {
		t.Fatalf("expected two distinct PR comments to sync to enabled issue, got %d", enabledCommentMentions)
	}
	var secondEnabledComments int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM comment
		WHERE issue_id = $1 AND author_id = $2
		  AND (
		    content LIKE '%Please fix the failing assertion%'
		    OR content LIKE '%Please update the tooltip copy%'
		  )
	`, enabledIssueID2, botID).Scan(&secondEnabledComments); err != nil {
		t.Fatalf("count second enabled comments: %v", err)
	}
	if secondEnabledComments != 2 {
		t.Fatalf("expected both PR comments to notify second linked issue, got %d comments", secondEnabledComments)
	}
	sendReviewComment(enabledPR, "Inline review feedback")

	body := []byte(fmt.Sprintf(`{
		"action":"completed",
		"installation":{"id":%d},
		"check_run":{
			"name":"unit tests",
			"status":"completed",
			"conclusion":"success",
			"html_url":"%s/checks/1",
			"head_sha":"abc123",
			"pull_requests":[{"html_url":%q}]
		},
		"repository":{"full_name":"acme/widgets"}
	}`, installationID, enabledPR, enabledPR))
	req := httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "check_run")
	req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
	w := httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("check_run success: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var successComments int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM comment
		WHERE issue_id = $1 AND author_id = $2 AND content LIKE '%Conclusion:** success%'
	`, enabledIssueID, botID).Scan(&successComments); err != nil {
		t.Fatalf("count success comments: %v", err)
	}
	if successComments != 0 {
		t.Fatalf("expected successful check_run to be ignored, got %d comments", successComments)
	}

	body = []byte(fmt.Sprintf(`{
		"action":"completed",
		"installation":{"id":%d},
		"check_run":{
			"name":"unit tests",
			"status":"completed",
			"conclusion":"failure",
			"html_url":"%s/checks/2",
			"head_sha":"def456",
			"pull_requests":[{"html_url":%q}]
		},
		"repository":{"full_name":"acme/widgets"}
	}`, installationID, enabledPR, enabledPR))
	req = httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "check_run")
	req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
	w = httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("check_run failure: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	body = []byte(fmt.Sprintf(`{
		"action":"completed",
		"installation":{"id":%d},
		"workflow_run":{
			"name":"CI",
			"status":"completed",
			"conclusion":"success",
			"html_url":"%s/actions/runs/1",
			"head_sha":"ghi789",
			"head_branch":"main",
			"pull_requests":[{"number":%d}]
		},
		"repository":{"full_name":"acme/widgets"}
	}`, installationID, enabledPR, enabledPRNumber))
	req = httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "workflow_run")
	req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
	w = httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("workflow_run success: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var eventComments int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM comment
		WHERE issue_id = $1 AND author_id = $2
		  AND (
		    content LIKE '%Inline review feedback%'
		    OR content LIKE '%Conclusion:** failure%'
		    OR content LIKE '%Workflow:** CI%'
		  )
	`, enabledIssueID, botID).Scan(&eventComments); err != nil {
		t.Fatalf("count event comments: %v", err)
	}
	if eventComments != 3 {
		t.Fatalf("expected review comment, failed check_run, and completed workflow comments, got %d", eventComments)
	}

	var disabledComments int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM comment
		WHERE issue_id = $1 AND author_id = $2
	`, disabledIssueID, botID).Scan(&disabledComments); err != nil {
		t.Fatalf("count disabled comments: %v", err)
	}
	if disabledComments != 0 {
		t.Fatalf("expected no bot comment on disabled issue, got %d", disabledComments)
	}
}

func insertLinkedAutoFixIssue(t *testing.T, title, agentID, prURL string, enabled bool) string {
	t.Helper()
	ctx := context.Background()
	var issueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (
			workspace_id, title, status, priority, assignee_type, assignee_id,
			creator_type, creator_id, number, github_auto_fix_enabled
		)
		VALUES ($1, $2, 'todo', 'medium', 'agent', $3, 'member', $4, $5, $6)
		RETURNING id
	`, testWorkspaceID, title, agentID, testUserID, uniqueIssueNumber(), enabled).Scan(&issueID); err != nil {
		t.Fatalf("insert linked auto-fix issue: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID) })
	if _, err := testHandler.Queries.CreateIssueLink(ctx, db.CreateIssueLinkParams{
		IssueID:     parseUUID(issueID),
		WorkspaceID: parseUUID(testWorkspaceID),
		SourceType:  "github",
		Kind:        "pr",
		Direction:   "source",
		Url:         prURL,
		ExternalID:  "",
	}); err != nil {
		t.Fatalf("create issue link: %v", err)
	}
	return issueID
}

func TestReceiveGitHubEvent_PullRequestReusesLinkedGitHubIssue(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}
	ctx := context.Background()
	secret := []byte("test-secret")

	app, err := gh.NewApp(12345, testRSAKeyPEM)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.SetWebhookSecret(secret)
	prev := testHandler.GitHubApp
	testHandler.GitHubApp = app
	t.Cleanup(func() { testHandler.GitHubApp = prev })

	installationID := int64(424243)
	webhookID := setupGitHubWebhookFixture(t, installationID)

	var agentIDStr string
	if err := testPool.QueryRow(ctx,
		`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "Handler Test Agent",
	).Scan(&agentIDStr); err != nil {
		t.Fatalf("find agent: %v", err)
	}

	cfgJSON, _ := json.Marshal(CreateIssueActionConfig{
		AgentID:       agentIDStr,
		TitleTemplate: "PR: {{.title}}",
	})
	if _, err := testHandler.Queries.CreateWebhookAction(ctx, db.CreateWebhookActionParams{
		WebhookID:  webhookID,
		ActionType: "create_issue",
		Config:     cfgJSON,
		Enabled:    true,
		Position:   0,
	}); err != nil {
		t.Fatalf("create action: %v", err)
	}

	linkedIssueID := createTestIssue(t, "Existing GitHub issue")
	linkedIssueURL := fmt.Sprintf("https://github.com/acme/widgets/issues/%d", time.Now().UnixNano())
	if _, err := testHandler.Queries.CreateIssueLink(ctx, db.CreateIssueLinkParams{
		IssueID:     parseUUID(linkedIssueID),
		WorkspaceID: parseUUID(testWorkspaceID),
		SourceType:  "github",
		Kind:        "issue",
		Direction:   "source",
		Url:         linkedIssueURL,
		ExternalID:  "acme/widgets#123",
	}); err != nil {
		t.Fatalf("create linked issue link: %v", err)
	}

	var beforeCount int
	if err := testPool.QueryRow(ctx,
		`SELECT count(*) FROM issue WHERE workspace_id = $1`,
		testWorkspaceID,
	).Scan(&beforeCount); err != nil {
		t.Fatalf("count issues before webhook: %v", err)
	}

	prURL := fmt.Sprintf("https://github.com/acme/widgets/pull/%d", time.Now().UnixNano())
	body := []byte(fmt.Sprintf(`{
		"action":"opened",
		"installation":{"id":%d},
		"pull_request":{"number":778,"title":"Fix existing issue","html_url":%q,"user":{"login":"alice"},"head":{"ref":"feat"},"base":{"ref":"main"},"body":"Fixes %s"},
		"repository":{"full_name":"acme/widgets"}
	}`, installationID, prURL, linkedIssueURL))

	req := httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
	w := httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("PR opened: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	prLink, err := testHandler.Queries.GetIssueLinkByURL(ctx, db.GetIssueLinkByURLParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Url:         prURL,
	})
	if err != nil {
		t.Fatalf("PR issue_link not found: %v", err)
	}
	if got := uuidToString(prLink.IssueID); got != linkedIssueID {
		t.Fatalf("PR link issue_id = %s, want existing issue %s", got, linkedIssueID)
	}

	issueLink, err := testHandler.Queries.GetIssueLinkByURL(ctx, db.GetIssueLinkByURLParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Url:         linkedIssueURL,
	})
	if err != nil {
		t.Fatalf("linked GitHub issue link not found: %v", err)
	}
	if issueLink.ExternalID != "acme/widgets#123" {
		t.Fatalf("linked GitHub issue external_id = %q, want acme/widgets#123", issueLink.ExternalID)
	}

	var afterCount int
	if err := testPool.QueryRow(ctx,
		`SELECT count(*) FROM issue WHERE workspace_id = $1`,
		testWorkspaceID,
	).Scan(&afterCount); err != nil {
		t.Fatalf("count issues after webhook: %v", err)
	}
	if afterCount != beforeCount {
		t.Fatalf("issue count after webhook = %d, want %d", afterCount, beforeCount)
	}
}

func TestReceiveGitHubEvent_MultipleCreateActionsStillCreateSeparateIssues(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}
	ctx := context.Background()
	secret := []byte("test-secret")

	app, err := gh.NewApp(12345, testRSAKeyPEM)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.SetWebhookSecret(secret)
	prev := testHandler.GitHubApp
	testHandler.GitHubApp = app
	t.Cleanup(func() { testHandler.GitHubApp = prev })

	installationID := time.Now().UnixNano()
	webhookID := setupGitHubWebhookFixture(t, installationID)

	var agentIDStr string
	if err := testPool.QueryRow(ctx,
		`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "Handler Test Agent",
	).Scan(&agentIDStr); err != nil {
		t.Fatalf("find agent: %v", err)
	}

	for i, titleTemplate := range []string{"A: {{.title}}", "B: {{.title}}"} {
		cfgJSON, _ := json.Marshal(CreateIssueActionConfig{
			AgentID:       agentIDStr,
			TitleTemplate: titleTemplate,
		})
		if _, err := testHandler.Queries.CreateWebhookAction(ctx, db.CreateWebhookActionParams{
			WebhookID:  webhookID,
			ActionType: "create_issue",
			Config:     cfgJSON,
			Enabled:    true,
			Position:   int32(i),
		}); err != nil {
			t.Fatalf("create action %d: %v", i, err)
		}
	}

	var beforeCount int
	if err := testPool.QueryRow(ctx,
		`SELECT count(*) FROM issue WHERE workspace_id = $1`,
		testWorkspaceID,
	).Scan(&beforeCount); err != nil {
		t.Fatalf("count issues before webhook: %v", err)
	}

	prURL := fmt.Sprintf("https://github.com/acme/widgets/pull/%d", time.Now().UnixNano())
	body := []byte(fmt.Sprintf(`{
		"action":"opened",
		"installation":{"id":%d},
		"pull_request":{"number":779,"title":"Create two issues","html_url":%q,"user":{"login":"alice"},"head":{"ref":"feat"},"base":{"ref":"main"},"body":"no linked issue"},
		"repository":{"full_name":"acme/widgets"}
	}`, installationID, prURL))

	req := httptest.NewRequest("POST", "/api/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
	w := httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("PR opened: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var afterCount int
	if err := testPool.QueryRow(ctx,
		`SELECT count(*) FROM issue WHERE workspace_id = $1`,
		testWorkspaceID,
	).Scan(&afterCount); err != nil {
		t.Fatalf("count issues after webhook: %v", err)
	}
	if afterCount != beforeCount+2 {
		t.Fatalf("issue count after webhook = %d, want %d", afterCount, beforeCount+2)
	}
}

// uniqueIssueNumber yields a value unlikely to collide across parallel test
// runs on the same DB. We don't actually care about the number's meaning —
// it just makes the PR URL unique so the issue_link unique constraint never
// trips when tests are re-run without cleanup.
var issueNumberCounter int64

func uniqueIssueNumber() int64 {
	issueNumberCounter++
	return issueNumberCounter + 100000
}

// testRSAKeyPEM is a throw-away RSA key (PKCS1) used only to satisfy
// NewApp's parser in handler-level tests. It is never used to talk to GitHub.
// Generated with: openssl genrsa -traditional 2048
var testRSAKeyPEM = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAlNLs7cx+NRpPfIAbLv4eMnQqbNcJDrKTxvTGdp71ANhjEBw7
njVtpLI9zgJOuyRcAlfLAmETjx/sLj6ITFlR5fX0gBuY/R3KwjkeDZz/CaHQqInK
qG/D57Th7GeVdW/+2y7o1YZXB8yjn/rbVeMhamNYdnC2/Ym4MG8165tcLyVmcjsr
MWRFrruECrJctsvyzD2Lq4brUgxO1Puqp31nup32NWNpgKrcjmbJLQHWVcghnKtE
ydMCNoZTE9g52m/gfwt+ede3qs2weLgAwVpskgpEGtJqa5KqQ1aWdJITCvKlO15n
Sqlt4vl4YLC2uh7FMHyt1zLrRQqVAjiVVNA7xQIDAQABAoIBAAI/z9nmOdAWpjXk
/8QtjgpILC358AabV1Vt9KPtUhmhq5meO55wA0i2cu2upj741TUp24UdL3z9yAWI
52lz/iNiHMBk6mKE9UALOfONuGMOYYdykbALBGR3nOSESkN8nlb0tgsoHR+ejaiI
05DQPnyLpNYhbPqW/sQooiF99x41RhWH77WPd4LiSY/f+pU7RT9yqJ+f2YdliER3
EZBMWanvdpffvG/lRJP5JqxEfX5hsL5yviZemkyYYlsxG72W6VXAlPMulKG0mKYC
IjfgYSdV+ujc3JFhM9L6UPu7Aw/SwcA4srq+zT94m1mWdRGFWxxYYlzd5vDVp634
MY8B2XECgYEA0TmnJGT1ylzKwePPVKcZyYf9OK6EIri0ZA56rtH8gPBWAPBNZH0U
/sEe6Q5TDRemsWm6t+8RzkytX7v7lD/WB5CqOtb6JqT4JYdgvCOVob0kupBfHbOO
uiZ/mnCH8or+fSfV/9gPD8+RylGDjwFa5PVYwYkcF8MNBzPghioWu+kCgYEAthhl
Lx5Q/9EnDElhTD040l6CNjoDY93yj2mAlq7i/sCvdkf5HxV+vdIAwzGjFo3RBrag
RkRly5NYxO5KxC4Cq6i/az2sHtdA1ubM75T0ul0yflawcTpWEvUhbfuDAiyFAGi1
ABSEZojIADKidBnVrOAqVKf2katyoWqc7il0w30CgYAHTev36U5rcjHh8wIaAntz
/btpby5NyAUEOT0vPUWDeuCFx93r1DIXcsaRfF6J5nl7WCWcpkwI18R1wypVUqU2
PmazBy5Uiw3ewYsvBk8DBodxu/iWIN6qwQ1TZvpYDWI1HF7sP67G7og4eAAPzgxO
UgJ3P0Ir0jNyPO1pwa5pgQKBgQCzbYxehm/n8u6YE8JU/kp8N/X0euuWPz/ggmPb
lo5D2hfK5Bacw3B0mHZ53/JEqg8an1+EfacUlqc0vV1cu72T6h5cDJQKe63/U8MC
HHOdI3I6vS71Ezd3TKXZGqi3vqh7g7E+V/kyk3sHft1Gq6I5y1TKwAqc9SRp24Sw
xJayfQKBgBT/h78vGtnVFicXvesfYeE6ROjAIBe4GPLexcp4IGJS8uPKkNVRjjNc
AKSfqpp6NJbdJoVp065TrmAABlRexrJYOwRQXNo/u8PFbNK0/3FEAd8WT+LOUwz+
Gv39lult+hj2NKk6PjOCLRPZBzuhRoDvV1+uT5KJJ+0dvFEQJ1++
-----END RSA PRIVATE KEY-----
`)
