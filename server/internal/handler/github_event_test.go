package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	gh "github.com/nullne/multica/server/internal/github"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// signGitHubBody computes the X-Hub-Signature-256 header that GitHub would
// send for the given body and shared secret.
func signGitHubBody(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// setupManagedAutoFixRoutine provisions the managed GitHub auto-fix routine
// for the test workspace bound to the given installation_id and returns the
// workspace's GitHub bot user id. The routine is removed on cleanup so each
// test is isolated (the workspace can only hold one managed routine).
func setupManagedAutoFixRoutine(t *testing.T, installationID int64) string {
	t.Helper()
	ctx := context.Background()
	ws := parseUUID(testWorkspaceID)

	// Clear any leftover managed routine so this test's triggers are bound to
	// the installation id under test.
	_ = testHandler.Queries.DeleteManagedRoutineByWorkspace(ctx, ws)

	if err := EnsureGitHubAutoFixRoutine(
		ctx,
		testHandler.Queries,
		testPool,
		ws,
		pgtype.Int8{Int64: installationID, Valid: true},
	); err != nil {
		t.Fatalf("EnsureGitHubAutoFixRoutine: %v", err)
	}
	t.Cleanup(func() {
		_ = testHandler.Queries.DeleteManagedRoutineByWorkspace(ctx, ws)
	})

	routine, err := testHandler.Queries.GetManagedRoutineByWorkspace(ctx, ws)
	if err != nil {
		t.Fatalf("GetManagedRoutineByWorkspace: %v", err)
	}
	return uuidToString(routine.CreatedByID)
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

	// installation_id 99999 has no routine trigger → handler returns 200 (silent drop)
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

// TestEnsureGitHubAutoFixRoutine_Idempotent verifies the managed routine
// bootstrap creates exactly one routine (with its two github triggers, one
// comment_issue action, and a github bot user) and no-ops on a second call —
// mirroring connect/restart/reconnect behavior — and that disconnect removes
// it.
func TestEnsureGitHubAutoFixRoutine_Idempotent(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}
	ctx := context.Background()
	ws := parseUUID(testWorkspaceID)
	installationID := time.Now().UnixNano()

	botID := setupManagedAutoFixRoutine(t, installationID)
	if botID == "" {
		t.Fatal("expected a github bot user id")
	}

	// The bot user must exist and be a workspace member.
	var botKind string
	if err := testPool.QueryRow(ctx, `SELECT kind FROM "user" WHERE id = $1`, botID).Scan(&botKind); err != nil {
		t.Fatalf("load bot user: %v", err)
	}
	if botKind != "bot" {
		t.Fatalf("bot user kind = %q, want bot", botKind)
	}

	routine, err := testHandler.Queries.GetManagedRoutineByWorkspace(ctx, ws)
	if err != nil {
		t.Fatalf("GetManagedRoutineByWorkspace: %v", err)
	}
	if !routine.Managed {
		t.Fatal("expected routine.managed = true")
	}

	triggers, err := testHandler.Queries.ListRoutineTriggers(ctx, routine.ID)
	if err != nil {
		t.Fatalf("ListRoutineTriggers: %v", err)
	}
	if len(triggers) != 2 {
		t.Fatalf("managed routine triggers = %d, want 2", len(triggers))
	}
	actions, err := testHandler.Queries.ListRoutineActions(ctx, routine.ID)
	if err != nil {
		t.Fatalf("ListRoutineActions: %v", err)
	}
	if len(actions) != 1 || actions[0].ActionType != "comment_issue" {
		t.Fatalf("managed routine actions = %+v, want one comment_issue", actions)
	}

	// Second call must not create a duplicate routine.
	if err := EnsureGitHubAutoFixRoutine(ctx, testHandler.Queries, testPool, ws, pgtype.Int8{Int64: installationID, Valid: true}); err != nil {
		t.Fatalf("EnsureGitHubAutoFixRoutine (second call): %v", err)
	}
	again, err := testHandler.Queries.GetManagedRoutineByWorkspace(ctx, ws)
	if err != nil {
		t.Fatalf("GetManagedRoutineByWorkspace (second call): %v", err)
	}
	if uuidToString(again.ID) != uuidToString(routine.ID) {
		t.Fatalf("idempotency broken: routine id changed from %s to %s", uuidToString(routine.ID), uuidToString(again.ID))
	}

	// Disconnect: deleting the managed routine leaves no managed routine behind.
	if err := testHandler.Queries.DeleteManagedRoutineByWorkspace(ctx, ws); err != nil {
		t.Fatalf("DeleteManagedRoutineByWorkspace: %v", err)
	}
	if _, err := testHandler.Queries.GetManagedRoutineByWorkspace(ctx, ws); err != pgx.ErrNoRows {
		t.Fatalf("after disconnect expected ErrNoRows, got %v", err)
	}
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
	botID := setupManagedAutoFixRoutine(t, installationID)

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

	// A successful check_run must be ignored — the managed routine's failed-CI
	// trigger only matches the failed conclusions set.
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
