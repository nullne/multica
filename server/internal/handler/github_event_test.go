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
	req := httptest.NewRequest("POST", "/api/github/events", bytes.NewReader(body))
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
	req := httptest.NewRequest("POST", "/api/github/events", bytes.NewReader(body))
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
	req := httptest.NewRequest("POST", "/api/github/events", bytes.NewReader(body))
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

	req := httptest.NewRequest("POST", "/api/github/events", bytes.NewReader(body))
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

	req := httptest.NewRequest("POST", "/api/github/events", bytes.NewReader(body))
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

	req := httptest.NewRequest("POST", "/api/github/events", bytes.NewReader(body))
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

// TestReceiveGitHubEvent_StaleDispatchDaemonIDIsDropped exercises the
// fallback path in executeCreateIssueAction: when the webhook's create_issue
// config still references a daemon that no longer exists (e.g. the owner
// removed the local daemon, or a migration wiped the table), the webhook
// must not fail with the issue_dispatch_daemon_id_fkey FK violation. The
// issue should be created with dispatch_daemon_id = NULL instead.
func TestReceiveGitHubEvent_StaleDispatchDaemonIDIsDropped(t *testing.T) {
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

	// Random UUID that is guaranteed not to exist in the daemon table.
	var bogusDaemonID string
	if err := testPool.QueryRow(ctx, `SELECT gen_random_uuid()`).Scan(&bogusDaemonID); err != nil {
		t.Fatalf("generate bogus daemon id: %v", err)
	}

	cfgJSON, _ := json.Marshal(CreateIssueActionConfig{
		AgentID:          agentIDStr,
		TitleTemplate:    "PR: {{.title}}",
		DispatchDaemonID: bogusDaemonID,
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

	prURL := fmt.Sprintf("https://github.com/acme/widgets/pull/%d", uniqueIssueNumber())
	body := []byte(fmt.Sprintf(`{
		"action":"opened",
		"installation":{"id":%d},
		"pull_request":{"number":888,"title":"Stale daemon","html_url":%q,"user":{"login":"alice"},"head":{"ref":"feat"},"base":{"ref":"main"},"body":"x"},
		"repository":{"full_name":"acme/widgets"}
	}`, installationID, prURL))

	req := httptest.NewRequest("POST", "/api/github/events", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", signGitHubBody(secret, body))
	w := httptest.NewRecorder()
	testHandler.ReceiveGitHubEvent(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("PR opened with stale daemon id: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	link, err := testHandler.Queries.GetIssueLinkByURL(ctx, db.GetIssueLinkByURLParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Url:         prURL,
	})
	if err != nil {
		t.Fatalf("issue_link not found: %v", err)
	}
	t.Cleanup(func() {
		testHandler.Queries.DeleteIssue(ctx, link.IssueID)
		testHandler.Queries.DeleteIssueLink(ctx, link.ID)
	})

	var storedDaemonID *string
	if err := testPool.QueryRow(ctx,
		`SELECT dispatch_daemon_id::text FROM issue WHERE id = $1`,
		uuidToString(link.IssueID),
	).Scan(&storedDaemonID); err != nil {
		t.Fatalf("load issue dispatch_daemon_id: %v", err)
	}
	if storedDaemonID != nil {
		t.Fatalf("dispatch_daemon_id = %q, want NULL (stale id should have been dropped)", *storedDaemonID)
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
