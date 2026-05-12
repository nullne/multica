package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nullne/multica/server/internal/auth"
	"github.com/nullne/multica/server/internal/events"
	"github.com/nullne/multica/server/internal/realtime"
	whpkg "github.com/nullne/multica/server/internal/webhook"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

type stubFirebaseVerifier struct {
	identity *auth.FirebaseIdentity
	err      error
}

func (s stubFirebaseVerifier) VerifyIDToken(context.Context, string) (*auth.FirebaseIdentity, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.identity, nil
}

var testHandler *Handler
var testPool *pgxpool.Pool
var testUserID string
var testWorkspaceID string

const (
	handlerTestEmail         = "handler-test@multica.ai"
	handlerTestName          = "Handler Test User"
	handlerTestWorkspaceSlug = "handler-tests"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("Skipping tests: could not connect to database: %v\n", err)
		os.Exit(0)
	}
	if err := pool.Ping(ctx); err != nil {
		fmt.Printf("Skipping tests: database not reachable: %v\n", err)
		pool.Close()
		os.Exit(0)
	}

	queries := db.New(pool)
	hub := realtime.NewHub()
	go hub.Run()
	bus := events.New()
	testHandler = New(queries, pool, hub, bus, nil, nil, nil, nil)
	testPool = pool

	testUserID, testWorkspaceID, err = setupHandlerTestFixture(ctx, pool)
	if err != nil {
		fmt.Printf("Failed to set up handler test fixture: %v\n", err)
		pool.Close()
		os.Exit(1)
	}

	code := m.Run()
	if err := cleanupHandlerTestFixture(context.Background(), pool); err != nil {
		fmt.Printf("Failed to clean up handler test fixture: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	pool.Close()
	os.Exit(code)
}

func setupHandlerTestFixture(ctx context.Context, pool *pgxpool.Pool) (string, string, error) {
	if err := cleanupHandlerTestFixture(ctx, pool); err != nil {
		return "", "", err
	}

	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, handlerTestName, handlerTestEmail).Scan(&userID); err != nil {
		return "", "", err
	}

	var workspaceID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "Handler Tests", handlerTestWorkspaceSlug, "Temporary workspace for handler tests", "HAN").Scan(&workspaceID); err != nil {
		return "", "", err
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, workspaceID, userID); err != nil {
		return "", "", err
	}

	var runtimeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at
		)
		VALUES ($1, NULL, $2, 'cloud', $3, 'online', $4, '{}'::jsonb, now())
		RETURNING id
	`, workspaceID, "Handler Test Runtime", "codex", "Handler test runtime").Scan(&runtimeID); err != nil {
		return "", "", err
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO agent (
			workspace_id, name, description,
			visibility, owner_id, tools, triggers, providers
		)
		VALUES ($1, $2, '', 'workspace', $3, '[]'::jsonb, '[]'::jsonb, ARRAY['codex'])
	`, workspaceID, "Handler Test Agent", userID); err != nil {
		return "", "", err
	}

	return userID, workspaceID, nil
}

func cleanupHandlerTestFixture(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `DELETE FROM workspace WHERE slug = $1`, handlerTestWorkspaceSlug); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, handlerTestEmail); err != nil {
		return err
	}
	return nil
}

func newRequest(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	return req
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx, ok := req.Context().Value(chi.RouteCtxKey).(*chi.Context)
	if !ok || rctx == nil {
		rctx = chi.NewRouteContext()
	}
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestIssueCRUD(t *testing.T) {
	// Create
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":    "Test issue from Go test",
		"status":   "todo",
		"priority": "medium",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created IssueResponse
	json.NewDecoder(w.Body).Decode(&created)
	if created.Title != "Test issue from Go test" {
		t.Fatalf("CreateIssue: expected title 'Test issue from Go test', got '%s'", created.Title)
	}
	if created.Status != "todo" {
		t.Fatalf("CreateIssue: expected status 'todo', got '%s'", created.Status)
	}
	issueID := created.ID

	// Get
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.GetIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetIssue: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fetched IssueResponse
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != issueID {
		t.Fatalf("GetIssue: expected id '%s', got '%s'", issueID, fetched.ID)
	}

	// Update - partial (only status)
	w = httptest.NewRecorder()
	status := "in_progress"
	req = newRequest("PUT", "/api/issues/"+issueID, map[string]any{
		"status": status,
	})
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssue: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated IssueResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Status != "in_progress" {
		t.Fatalf("UpdateIssue: expected status 'in_progress', got '%s'", updated.Status)
	}
	if updated.Title != "Test issue from Go test" {
		t.Fatalf("UpdateIssue: title should be preserved, got '%s'", updated.Title)
	}
	if updated.Priority != "medium" {
		t.Fatalf("UpdateIssue: priority should be preserved, got '%s'", updated.Priority)
	}

	// List
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID, nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListIssues: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var listResp map[string]any
	json.NewDecoder(w.Body).Decode(&listResp)
	issues := listResp["issues"].([]any)
	if len(issues) == 0 {
		t.Fatal("ListIssues: expected at least 1 issue")
	}

	// Delete
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.DeleteIssue(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteIssue: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify deleted
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.GetIssue(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetIssue after delete: expected 404, got %d", w.Code)
	}
}

func TestUpdateIssuePreservesLinks(t *testing.T) {
	ctx := context.Background()

	// Create an issue.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":    "Link preservation test issue",
		"status":   "todo",
		"priority": "medium",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created IssueResponse
	json.NewDecoder(w.Body).Decode(&created)
	issueID := created.ID
	defer func() {
		w := httptest.NewRecorder()
		req := newRequest("DELETE", "/api/issues/"+issueID, nil)
		req = withURLParam(req, "id", issueID)
		testHandler.DeleteIssue(w, req)
	}()

	// Seed a source link directly via the DB.
	issueUUID := parseUUID(issueID)
	wsUUID := parseUUID(testWorkspaceID)
	_, err := testHandler.Queries.CreateIssueLink(ctx, db.CreateIssueLinkParams{
		IssueID:     issueUUID,
		WorkspaceID: wsUUID,
		SourceType:  "github",
		Kind:        "issue",
		Direction:   "source",
		Url:         "https://github.com/test/repo/issues/1",
		ExternalID:  "test/repo#1",
	})
	if err != nil {
		t.Fatalf("CreateIssueLink: %v", err)
	}

	// Update the description — this must not drop links from the response.
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/issues/"+issueID, map[string]any{
		"description": "Updated description",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssue: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated IssueResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if len(updated.Links) == 0 {
		t.Fatal("UpdateIssue: links must not be empty after description update")
	}
	if updated.Links[0].URL != "https://github.com/test/repo/issues/1" {
		t.Fatalf("UpdateIssue: unexpected link URL: %s", updated.Links[0].URL)
	}
}

func TestCommentCRUD(t *testing.T) {
	// Create an issue first
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Comment test issue",
	})
	testHandler.CreateIssue(w, req)
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)
	issueID := issue.ID

	// Create comment
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "Test comment from Go test",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List comments
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID+"/comments", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.ListComments(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListComments: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var comments []CommentResponse
	json.NewDecoder(w.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Fatalf("ListComments: expected 1 comment, got %d", len(comments))
	}
	if comments[0].Content != "Test comment from Go test" {
		t.Fatalf("ListComments: expected content 'Test comment from Go test', got '%s'", comments[0].Content)
	}

	// Cleanup
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.DeleteIssue(w, req)
}

func TestAgentCRUD(t *testing.T) {
	// List agents
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/agents?workspace_id="+testWorkspaceID, nil)
	testHandler.ListAgents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListAgents: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var agents []AgentResponse
	json.NewDecoder(w.Body).Decode(&agents)
	if len(agents) == 0 {
		t.Fatal("ListAgents: expected at least 1 agent")
	}

	// Update agent status
	agentID := agents[0].ID
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/agents/"+agentID, map[string]any{
		"status": "idle",
	})
	req = withURLParam(req, "id", agentID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated AgentResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Status != "idle" {
		t.Fatalf("UpdateAgent: expected status 'idle', got '%s'", updated.Status)
	}
	if updated.Name != agents[0].Name {
		t.Fatalf("UpdateAgent: name should be preserved, got '%s'", updated.Name)
	}
}

func TestAgentGitHubCodeAccess(t *testing.T) {
	// List agents to get an existing agent.
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/agents?workspace_id="+testWorkspaceID, nil)
	testHandler.ListAgents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListAgents: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var agents []AgentResponse
	json.NewDecoder(w.Body).Decode(&agents)
	if len(agents) == 0 {
		t.Fatal("need at least 1 agent")
	}
	agentID := agents[0].ID

	// Default should be "read".
	if agents[0].GitHubCodeAccess != "read" {
		t.Errorf("default github_code_access = %q, want 'read'", agents[0].GitHubCodeAccess)
	}

	// Update to "write".
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/agents/"+agentID, map[string]any{
		"github_code_access": "write",
	})
	req = withURLParam(req, "id", agentID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent(write): expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated AgentResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.GitHubCodeAccess != "write" {
		t.Errorf("after update: github_code_access = %q, want 'write'", updated.GitHubCodeAccess)
	}

	// Update to "admin".
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/agents/"+agentID, map[string]any{
		"github_code_access": "admin",
	})
	req = withURLParam(req, "id", agentID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent(admin): expected 200, got %d: %s", w.Code, w.Body.String())
	}
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.GitHubCodeAccess != "admin" {
		t.Errorf("after update: github_code_access = %q, want 'admin'", updated.GitHubCodeAccess)
	}

	// Invalid value should be rejected.
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/agents/"+agentID, map[string]any{
		"github_code_access": "invalid",
	})
	req = withURLParam(req, "id", agentID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("UpdateAgent(invalid): expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Restore to default.
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/agents/"+agentID, map[string]any{
		"github_code_access": "read",
	})
	req = withURLParam(req, "id", agentID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent(restore): expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWorkspaceGitHubConnected(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/workspaces", nil)
	testHandler.ListWorkspaces(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListWorkspaces: expected 200, got %d", w.Code)
	}
	var workspaces []WorkspaceResponse
	json.NewDecoder(w.Body).Decode(&workspaces)
	if len(workspaces) == 0 {
		t.Fatal("need at least 1 workspace")
	}
	if workspaces[0].GitHubConnected {
		t.Error("new workspace should not have github_connected=true")
	}
}

func TestWorkspaceCRUD(t *testing.T) {
	// List workspaces
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/workspaces", nil)
	testHandler.ListWorkspaces(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListWorkspaces: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var workspaces []WorkspaceResponse
	json.NewDecoder(w.Body).Decode(&workspaces)
	if len(workspaces) == 0 {
		t.Fatal("ListWorkspaces: expected at least 1 workspace")
	}

	// Get workspace
	wsID := workspaces[0].ID
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/workspaces/"+wsID, nil)
	req = withURLParam(req, "id", wsID)
	testHandler.GetWorkspace(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetWorkspace: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoginWithFirebase(t *testing.T) {
	const email = "firebase-login-test@multica.ai"
	ctx := context.Background()
	originalVerifier := testHandler.FirebaseVerifier
	testHandler.FirebaseVerifier = stubFirebaseVerifier{
		identity: &auth.FirebaseIdentity{
			Email: email,
			Name:  "Firebase Login Test",
		},
	}

	t.Cleanup(func() {
		testHandler.FirebaseVerifier = originalVerifier
		user, err := testHandler.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			workspaces, listErr := testHandler.Queries.ListWorkspaces(ctx, user.ID)
			if listErr == nil {
				for _, workspace := range workspaces {
					_ = testHandler.Queries.DeleteWorkspace(ctx, workspace.ID)
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"id_token": "firebase-token"})
	req := httptest.NewRequest("POST", "/auth/firebase", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.LoginWithFirebase(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("LoginWithFirebase: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LoginResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Token == "" {
		t.Fatal("LoginWithFirebase: expected non-empty token")
	}
	if resp.User.Email != email {
		t.Fatalf("LoginWithFirebase: expected email '%s', got '%s'", email, resp.User.Email)
	}
}

func TestLoginDevDisabledByDefault(t *testing.T) {
	t.Setenv("DEV_AUTH_BYPASS", "")

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": "dev-disabled@multica.ai"})
	req := httptest.NewRequest("POST", "/auth/dev", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.LoginDev(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("LoginDev (disabled): expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoginDevIssuesToken(t *testing.T) {
	const email = "dev-bypass-test@multica.ai"
	ctx := context.Background()
	t.Setenv("DEV_AUTH_BYPASS", "1")

	t.Cleanup(func() {
		user, err := testHandler.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			workspaces, listErr := testHandler.Queries.ListWorkspaces(ctx, user.ID)
			if listErr == nil {
				for _, workspace := range workspaces {
					_ = testHandler.Queries.DeleteWorkspace(ctx, workspace.ID)
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": email, "name": "Dev Bypass"})
	req := httptest.NewRequest("POST", "/auth/dev", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.LoginDev(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("LoginDev: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LoginResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Token == "" {
		t.Fatal("LoginDev: expected non-empty token")
	}
	if resp.User.Email != email {
		t.Fatalf("LoginDev: expected email '%s', got '%s'", email, resp.User.Email)
	}

	// A workspace should have been provisioned for the new user.
	user, err := testHandler.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	workspaces, err := testHandler.Queries.ListWorkspaces(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("ListWorkspaces: expected 1 workspace, got %d", len(workspaces))
	}
}

func TestLoginDevRejectsMissingEmail(t *testing.T) {
	t.Setenv("DEV_AUTH_BYPASS", "1")

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{})
	req := httptest.NewRequest("POST", "/auth/dev", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.LoginDev(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("LoginDev: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoginWithFirebaseRejectsInvalidToken(t *testing.T) {
	originalVerifier := testHandler.FirebaseVerifier
	testHandler.FirebaseVerifier = stubFirebaseVerifier{err: errors.New("invalid token")}
	t.Cleanup(func() {
		testHandler.FirebaseVerifier = originalVerifier
	})

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"id_token": "bad-token"})
	req := httptest.NewRequest("POST", "/auth/firebase", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.LoginWithFirebase(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("LoginWithFirebase: expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoginWithFirebaseCreatesWorkspace(t *testing.T) {
	const email = "firebase-workspace-test@multica.ai"
	ctx := context.Background()
	originalVerifier := testHandler.FirebaseVerifier
	testHandler.FirebaseVerifier = stubFirebaseVerifier{
		identity: &auth.FirebaseIdentity{
			Email: email,
			Name:  "Workspace Provision Test",
		},
	}

	t.Cleanup(func() {
		testHandler.FirebaseVerifier = originalVerifier
		user, err := testHandler.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			workspaces, listErr := testHandler.Queries.ListWorkspaces(ctx, user.ID)
			if listErr == nil {
				for _, workspace := range workspaces {
					_ = testHandler.Queries.DeleteWorkspace(ctx, workspace.ID)
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"id_token": "firebase-token"})
	req := httptest.NewRequest("POST", "/auth/firebase", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.LoginWithFirebase(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("LoginWithFirebase: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	user, err := testHandler.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}

	workspaces, err := testHandler.Queries.ListWorkspaces(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("ListWorkspaces: expected 1 workspace, got %d", len(workspaces))
	}
	if !strings.Contains(workspaces[0].Name, "Workspace") {
		t.Fatalf("expected auto-created workspace name, got %q", workspaces[0].Name)
	}
}

func TestLoginWithFirebaseAllowsInvitedMember(t *testing.T) {
	const email = "invited-member-login@example.invalid"
	ctx := context.Background()

	// Restrict the allowlist so this email would be rejected on its own.
	t.Setenv("ALLOWED_EMAILS", "someone-else@multica.ai")
	t.Setenv("ALLOWED_EMAIL_DOMAINS", "")

	originalVerifier := testHandler.FirebaseVerifier
	testHandler.FirebaseVerifier = stubFirebaseVerifier{
		identity: &auth.FirebaseIdentity{
			Email: email,
			Name:  "Invited Member",
		},
	}

	// Simulate the invite flow: create a user with this email and make them a
	// member of the existing test workspace.
	var invitedUserID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, "Invited Member", email).Scan(&invitedUserID); err != nil {
		t.Fatalf("seed invited user: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'member')
	`, testWorkspaceID, invitedUserID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	t.Cleanup(func() {
		testHandler.FirebaseVerifier = originalVerifier
		testPool.Exec(ctx, `DELETE FROM member WHERE user_id = $1`, invitedUserID)
		testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, invitedUserID)
	})

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"id_token": "firebase-token"})
	req := httptest.NewRequest("POST", "/auth/firebase", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.LoginWithFirebase(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("LoginWithFirebase: expected 200 for invited member, got %d: %s", w.Code, w.Body.String())
	}

	var resp LoginResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.User.Email != email {
		t.Fatalf("LoginWithFirebase: expected email %q, got %q", email, resp.User.Email)
	}
}

func TestLoginWithFirebaseRejectsNonMember(t *testing.T) {
	const email = "stranger@example.invalid"
	ctx := context.Background()

	t.Setenv("ALLOWED_EMAILS", "someone-else@multica.ai")
	t.Setenv("ALLOWED_EMAIL_DOMAINS", "")

	originalVerifier := testHandler.FirebaseVerifier
	testHandler.FirebaseVerifier = stubFirebaseVerifier{
		identity: &auth.FirebaseIdentity{
			Email: email,
			Name:  "Stranger",
		},
	}

	t.Cleanup(func() {
		testHandler.FirebaseVerifier = originalVerifier
		// In case the user got created somehow, clean up.
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"id_token": "firebase-token"})
	req := httptest.NewRequest("POST", "/auth/firebase", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.LoginWithFirebase(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("LoginWithFirebase: expected 403 for non-member, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := testHandler.Queries.GetUserByEmail(ctx, email); err == nil {
		t.Fatalf("LoginWithFirebase: rejected user should not have been created")
	}
}

func TestLoginWithFirebaseHonorsAllowlist(t *testing.T) {
	const email = "allowlisted@multica.ai"
	ctx := context.Background()

	t.Setenv("ALLOWED_EMAILS", email)
	t.Setenv("ALLOWED_EMAIL_DOMAINS", "")

	originalVerifier := testHandler.FirebaseVerifier
	testHandler.FirebaseVerifier = stubFirebaseVerifier{
		identity: &auth.FirebaseIdentity{
			Email: email,
			Name:  "Allowlisted User",
		},
	}

	t.Cleanup(func() {
		testHandler.FirebaseVerifier = originalVerifier
		user, err := testHandler.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			workspaces, listErr := testHandler.Queries.ListWorkspaces(ctx, user.ID)
			if listErr == nil {
				for _, workspace := range workspaces {
					_ = testHandler.Queries.DeleteWorkspace(ctx, workspace.ID)
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"id_token": "firebase-token"})
	req := httptest.NewRequest("POST", "/auth/firebase", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.LoginWithFirebase(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("LoginWithFirebase: expected 200 for allowlisted email, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveActor(t *testing.T) {
	ctx := context.Background()

	// Look up the agent created by the test fixture.
	var agentID string
	err := testPool.QueryRow(ctx,
		`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "Handler Test Agent",
	).Scan(&agentID)
	if err != nil {
		t.Fatalf("failed to find test agent: %v", err)
	}

	// Create a task for the agent so we can test X-Task-ID validation.
	var issueID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position)
		 VALUES ($1, 'resolveActor test', 'todo', 'none', 'member', $2, 9999, 0)
		 RETURNING id`, testWorkspaceID, testUserID,
	).Scan(&issueID)
	if err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}

	// Look up a runtime in the same workspace.
	var runtimeID string
	err = testPool.QueryRow(ctx, `SELECT id FROM agent_runtime WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&runtimeID)
	if err != nil {
		t.Fatalf("failed to get runtime_id: %v", err)
	}

	var taskID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority)
		 VALUES ($1, $2, $3, 'queued', 0)
		 RETURNING id`, agentID, runtimeID, issueID,
	).Scan(&taskID)
	if err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
	})

	tests := []struct {
		name          string
		agentIDHeader string
		taskIDHeader  string
		wantActorType string
		wantIsAgent   bool
	}{
		{
			name:          "no headers returns member",
			wantActorType: "member",
		},
		{
			name:          "valid agent ID returns agent",
			agentIDHeader: agentID,
			wantActorType: "agent",
			wantIsAgent:   true,
		},
		{
			name:          "non-existent agent ID returns member",
			agentIDHeader: "00000000-0000-0000-0000-000000000099",
			wantActorType: "member",
		},
		{
			name:          "valid agent + valid task returns agent",
			agentIDHeader: agentID,
			taskIDHeader:  taskID,
			wantActorType: "agent",
			wantIsAgent:   true,
		},
		{
			name:          "valid agent + wrong task returns member",
			agentIDHeader: agentID,
			taskIDHeader:  "00000000-0000-0000-0000-000000000099",
			wantActorType: "member",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRequest("GET", "/test", nil)
			if tt.agentIDHeader != "" {
				req.Header.Set("X-Agent-ID", tt.agentIDHeader)
			}
			if tt.taskIDHeader != "" {
				req.Header.Set("X-Task-ID", tt.taskIDHeader)
			}

			actorType, actorID := testHandler.resolveActor(req, testUserID, testWorkspaceID)

			if actorType != tt.wantActorType {
				t.Errorf("actorType = %q, want %q", actorType, tt.wantActorType)
			}
			if tt.wantIsAgent {
				if actorID != tt.agentIDHeader {
					t.Errorf("actorID = %q, want agent %q", actorID, tt.agentIDHeader)
				}
			} else {
				if actorID != testUserID {
					t.Errorf("actorID = %q, want user %q", actorID, testUserID)
				}
			}
		})
	}
}

func TestDaemonRegisterRequiresAuthenticatedUser(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(`{
		"daemon_id":"local-daemon",
		"device_name":"test-machine",
		"runtimes":[{"name":"Local Codex","type":"codex","version":"1.0.0","status":"online"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	// Intentionally omit X-User-ID — daemon registration is user-scoped now.

	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("DaemonRegister: expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDaemonRegisterRequiresDaemonID(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/register", map[string]any{
		"device_name": "test-machine",
		"runtimes":    []map[string]any{},
	})

	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("DaemonRegister: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Daemon / Runtime Status Lifecycle Tests
// ---------------------------------------------------------------------------

// daemonRegisterResponse is the shape returned by DaemonRegister.
// The daemon is user-scoped and projected into one workspace registration
// entry per enabled workspace.
type daemonRegisterResponse struct {
	Daemon     DaemonResponse          `json:"daemon"`
	Workspaces []WorkspaceRegistration `json:"workspaces"`
}

// runtimesInTestWorkspace flattens the per-workspace runtime list and returns
// just the runtimes projected into the default test workspace.
func (r daemonRegisterResponse) runtimesInTestWorkspace() []AgentRuntimeResponse {
	for _, ws := range r.Workspaces {
		if ws.WorkspaceID == testWorkspaceID {
			return ws.Runtimes
		}
	}
	return nil
}

// registerDaemon is a test helper that registers a daemon and returns the
// parsed response. It fails the test on any error.
func registerDaemon(t *testing.T, daemonID, deviceName string, runtimes []map[string]any) daemonRegisterResponse {
	t.Helper()
	body := map[string]any{
		"daemon_id":   daemonID,
		"device_name": deviceName,
		"cli_version": "0.1.0",
		"runtimes":    runtimes,
	}
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/register", body)
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonRegister: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp daemonRegisterResponse
	json.NewDecoder(w.Body).Decode(&resp)
	return resp
}

// cleanupDaemon removes daemon, its runtimes, and workspace assignments.
func cleanupDaemon(t *testing.T, daemonUUID string) {
	t.Helper()
	ctx := context.Background()
	testPool.Exec(ctx, `DELETE FROM agent_runtime WHERE daemon_ref = $1`, daemonUUID)
	testPool.Exec(ctx, `DELETE FROM daemon_workspace WHERE daemon_id = $1`, daemonUUID)
	testPool.Exec(ctx, `DELETE FROM daemon WHERE id = $1`, daemonUUID)
}

// TestDaemonRegisterSetsOnlineStatus verifies that registering a daemon
// sets both the daemon and its runtimes to "online", and that runtime rows
// are projected into the user's existing workspace.
func TestDaemonRegisterSetsOnlineStatus(t *testing.T) {
	resp := registerDaemon(t, "status-test-daemon", "test-machine", []map[string]any{
		{"name": "Test Claude", "type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		{"name": "Test Codex", "type": "codex", "version": "2.0.0", "status": "online", "auth_status": "unauthenticated"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	// Daemon should be online and owned by the authenticated user.
	if resp.Daemon.Status != "online" {
		t.Fatalf("expected daemon status 'online', got '%s'", resp.Daemon.Status)
	}
	if resp.Daemon.DaemonID != "status-test-daemon" {
		t.Fatalf("expected daemon_id 'status-test-daemon', got '%s'", resp.Daemon.DaemonID)
	}
	if resp.Daemon.UserID != testUserID {
		t.Fatalf("expected daemon owner %s, got %s", testUserID, resp.Daemon.UserID)
	}

	// The test workspace should have been auto-enabled on first register.
	runtimes := resp.runtimesInTestWorkspace()
	if len(runtimes) != 2 {
		t.Fatalf("expected 2 runtimes in test workspace, got %d (workspaces: %d)", len(runtimes), len(resp.Workspaces))
	}
	for _, rt := range runtimes {
		if rt.Status != "online" {
			t.Fatalf("expected runtime status 'online', got '%s' for provider %s", rt.Status, rt.Provider)
		}
	}
}

// TestDaemonRegisterAuthStatusValues verifies that auth_status is correctly
// stored for each runtime during registration.
func TestDaemonRegisterAuthStatusValues(t *testing.T) {
	resp := registerDaemon(t, "auth-status-daemon", "auth-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		{"type": "codex", "version": "2.0.0", "status": "online", "auth_status": "unauthenticated"},
		{"type": "opencode", "version": "3.0.0", "status": "online", "auth_status": "not_installed"},
		{"type": "cursor", "version": "4.0.0", "status": "online", "auth_status": "bogus_value"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	authByProvider := map[string]string{}
	for _, rt := range resp.runtimesInTestWorkspace() {
		authByProvider[rt.Provider] = rt.AuthStatus
	}

	// Valid values are passed through.
	if authByProvider["claude"] != "ready" {
		t.Fatalf("claude auth_status: expected 'ready', got '%s'", authByProvider["claude"])
	}
	if authByProvider["codex"] != "unauthenticated" {
		t.Fatalf("codex auth_status: expected 'unauthenticated', got '%s'", authByProvider["codex"])
	}
	if authByProvider["opencode"] != "not_installed" {
		t.Fatalf("opencode auth_status: expected 'not_installed', got '%s'", authByProvider["opencode"])
	}
	// Invalid values default to "unknown".
	if authByProvider["cursor"] != "unknown" {
		t.Fatalf("cursor auth_status: expected 'unknown' for invalid input, got '%s'", authByProvider["cursor"])
	}
}

// TestDaemonRegisterOfflineRuntime verifies that a runtime explicitly
// reported as offline during registration keeps offline status.
func TestDaemonRegisterOfflineRuntime(t *testing.T) {
	resp := registerDaemon(t, "offline-rt-daemon", "machine-1", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "offline", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	// Daemon itself is online (it registered successfully).
	if resp.Daemon.Status != "online" {
		t.Fatalf("expected daemon status 'online', got '%s'", resp.Daemon.Status)
	}
	// But the runtime it reported is offline.
	runtimes := resp.runtimesInTestWorkspace()
	if len(runtimes) != 1 {
		t.Fatalf("expected 1 runtime, got %d", len(runtimes))
	}
	if runtimes[0].Status != "offline" {
		t.Fatalf("expected runtime status 'offline', got '%s'", runtimes[0].Status)
	}
}

// TestDaemonHeartbeatRestoresOnlineStatus verifies that after a heartbeat,
// the daemon and all its runtimes are set back to "online".
func TestDaemonHeartbeatRestoresOnlineStatus(t *testing.T) {
	// Register a daemon first.
	resp := registerDaemon(t, "hb-test-daemon", "hb-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })
	daemonUUID := resp.Daemon.ID

	// Manually set daemon + runtimes to offline to simulate a stale sweep.
	ctx := context.Background()
	testPool.Exec(ctx, `UPDATE daemon SET status = 'offline' WHERE id = $1`, daemonUUID)
	testPool.Exec(ctx, `UPDATE agent_runtime SET status = 'offline' WHERE daemon_ref = $1`, daemonUUID)

	// Verify they're offline.
	var status string
	testPool.QueryRow(ctx, `SELECT status FROM daemon WHERE id = $1`, daemonUUID).Scan(&status)
	if status != "offline" {
		t.Fatalf("precondition: expected daemon offline, got '%s'", status)
	}

	// Send heartbeat.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/heartbeat", map[string]any{
		"daemon_id": daemonUUID,
	})
	testHandler.DaemonHeartbeat(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonHeartbeat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify daemon is back online.
	testPool.QueryRow(ctx, `SELECT status FROM daemon WHERE id = $1`, daemonUUID).Scan(&status)
	if status != "online" {
		t.Fatalf("expected daemon status 'online' after heartbeat, got '%s'", status)
	}

	// Verify runtime is back online.
	testPool.QueryRow(ctx, `SELECT status FROM agent_runtime WHERE daemon_ref = $1`, daemonUUID).Scan(&status)
	if status != "online" {
		t.Fatalf("expected runtime status 'online' after heartbeat, got '%s'", status)
	}
}

// TestDaemonHeartbeatUpdatesAuthStatus verifies that auth_statuses sent
// during heartbeat update the runtime's auth_status.
func TestDaemonHeartbeatUpdatesAuthStatus(t *testing.T) {
	resp := registerDaemon(t, "hb-auth-daemon", "auth-hb-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "unauthenticated"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	// Verify initial auth_status.
	ctx := context.Background()
	var authStatus string
	testPool.QueryRow(ctx, `SELECT auth_status FROM agent_runtime WHERE daemon_ref = $1 AND provider = 'claude'`, resp.Daemon.ID).Scan(&authStatus)
	if authStatus != "unauthenticated" {
		t.Fatalf("precondition: expected auth_status 'unauthenticated', got '%s'", authStatus)
	}

	// Heartbeat with updated auth status.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/heartbeat", map[string]any{
		"daemon_id": resp.Daemon.ID,
		"auth_statuses": map[string]string{
			"claude": "ready",
		},
	})
	testHandler.DaemonHeartbeat(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonHeartbeat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify auth_status updated.
	testPool.QueryRow(ctx, `SELECT auth_status FROM agent_runtime WHERE daemon_ref = $1 AND provider = 'claude'`, resp.Daemon.ID).Scan(&authStatus)
	if authStatus != "ready" {
		t.Fatalf("expected auth_status 'ready' after heartbeat, got '%s'", authStatus)
	}
}

// TestDaemonDeregisterSetsOffline verifies that deregistering a daemon
// sets both the daemon and all its runtimes to "offline".
func TestDaemonDeregisterSetsOffline(t *testing.T) {
	resp := registerDaemon(t, "dereg-daemon", "dereg-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		{"type": "codex", "version": "2.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	// Verify all are online.
	ctx := context.Background()
	var count int
	testPool.QueryRow(ctx, `SELECT count(*) FROM agent_runtime WHERE daemon_ref = $1 AND status = 'online'`, resp.Daemon.ID).Scan(&count)
	if count != 2 {
		t.Fatalf("precondition: expected 2 online runtimes, got %d", count)
	}

	// Deregister.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/deregister", map[string]any{
		"daemon_id": resp.Daemon.ID,
	})
	testHandler.DaemonDeregister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonDeregister: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Daemon should be offline.
	var status string
	testPool.QueryRow(ctx, `SELECT status FROM daemon WHERE id = $1`, resp.Daemon.ID).Scan(&status)
	if status != "offline" {
		t.Fatalf("expected daemon status 'offline' after deregister, got '%s'", status)
	}

	// All runtimes should be offline.
	testPool.QueryRow(ctx, `SELECT count(*) FROM agent_runtime WHERE daemon_ref = $1 AND status = 'offline'`, resp.Daemon.ID).Scan(&count)
	if count != 2 {
		t.Fatalf("expected 2 offline runtimes after deregister, got %d", count)
	}
}

// TestDaemonReRegisterAfterDeregister verifies that re-registering a
// previously deregistered daemon restores online status.
func TestDaemonReRegisterAfterDeregister(t *testing.T) {
	resp := registerDaemon(t, "rereg-daemon", "rereg-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	// Deregister.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/deregister", map[string]any{
		"daemon_id": resp.Daemon.ID,
	})
	testHandler.DaemonDeregister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonDeregister: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Re-register with the same daemon_id.
	resp2 := registerDaemon(t, "rereg-daemon", "rereg-machine", []map[string]any{
		{"type": "claude", "version": "1.1.0", "status": "online", "auth_status": "ready"},
	})

	// Should reuse the same daemon row (upsert by user+daemon_id).
	if resp2.Daemon.ID != resp.Daemon.ID {
		t.Fatalf("expected same daemon UUID after re-register, got different: %s vs %s", resp.Daemon.ID, resp2.Daemon.ID)
	}
	if resp2.Daemon.Status != "online" {
		t.Fatalf("expected daemon status 'online' after re-register, got '%s'", resp2.Daemon.Status)
	}
	runtimes2 := resp2.runtimesInTestWorkspace()
	if len(runtimes2) != 1 || runtimes2[0].Status != "online" {
		t.Fatalf("expected runtime online after re-register, got %d runtimes", len(runtimes2))
	}
}

// TestListDaemonsReturnsCorrectStatus verifies that ListDaemons returns
// the real status from the database, matching what the management page sees.
func TestListDaemonsReturnsCorrectStatus(t *testing.T) {
	resp := registerDaemon(t, "list-status-daemon", "list-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	// List daemons — should show online.
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/daemons?workspace_id="+testWorkspaceID, nil)
	testHandler.ListDaemons(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListDaemons: expected 200, got %d", w.Code)
	}
	var daemons []DaemonResponse
	json.NewDecoder(w.Body).Decode(&daemons)

	found := false
	for _, d := range daemons {
		if d.ID == resp.Daemon.ID {
			found = true
			if d.Status != "online" {
				t.Fatalf("ListDaemons: expected status 'online', got '%s'", d.Status)
			}
		}
	}
	if !found {
		t.Fatal("ListDaemons: registered daemon not found in list")
	}

	// Set daemon to offline, re-list.
	ctx := context.Background()
	testPool.Exec(ctx, `UPDATE daemon SET status = 'offline' WHERE id = $1`, resp.Daemon.ID)

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/daemons?workspace_id="+testWorkspaceID, nil)
	testHandler.ListDaemons(w, req)
	json.NewDecoder(w.Body).Decode(&daemons)

	for _, d := range daemons {
		if d.ID == resp.Daemon.ID {
			if d.Status != "offline" {
				t.Fatalf("ListDaemons: expected status 'offline' after manual update, got '%s'", d.Status)
			}
		}
	}

	// Set daemon to updating, re-list — this is the key consistency check.
	testPool.Exec(ctx, `UPDATE daemon SET status = 'updating' WHERE id = $1`, resp.Daemon.ID)

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/daemons?workspace_id="+testWorkspaceID, nil)
	testHandler.ListDaemons(w, req)
	json.NewDecoder(w.Body).Decode(&daemons)

	for _, d := range daemons {
		if d.ID == resp.Daemon.ID {
			if d.Status != "updating" {
				t.Fatalf("ListDaemons: expected status 'updating', got '%s'", d.Status)
			}
		}
	}
}

// TestListRuntimesReturnsCorrectStatus verifies that ListAgentRuntimes
// returns the real status for each runtime.
func TestListRuntimesReturnsCorrectStatus(t *testing.T) {
	resp := registerDaemon(t, "list-rt-daemon", "list-rt-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	wsRuntimes := resp.runtimesInTestWorkspace()
	if len(wsRuntimes) == 0 {
		t.Fatal("expected at least one runtime projected into test workspace")
	}
	rtID := wsRuntimes[0].ID

	// List runtimes — should show online.
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/runtimes?workspace_id="+testWorkspaceID, nil)
	testHandler.ListAgentRuntimes(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListAgentRuntimes: expected 200, got %d", w.Code)
	}
	var runtimes []AgentRuntimeResponse
	json.NewDecoder(w.Body).Decode(&runtimes)

	for _, rt := range runtimes {
		if rt.ID == rtID {
			if rt.Status != "online" {
				t.Fatalf("ListRuntimes: expected 'online', got '%s'", rt.Status)
			}
		}
	}

	// Set to updating, verify ListRuntimes reflects it.
	ctx := context.Background()
	testPool.Exec(ctx, `UPDATE agent_runtime SET status = 'updating' WHERE id = $1`, rtID)

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/runtimes?workspace_id="+testWorkspaceID, nil)
	testHandler.ListAgentRuntimes(w, req)
	json.NewDecoder(w.Body).Decode(&runtimes)

	for _, rt := range runtimes {
		if rt.ID == rtID {
			if rt.Status != "updating" {
				t.Fatalf("ListRuntimes: expected 'updating', got '%s'", rt.Status)
			}
		}
	}
}

// TestStaleDaemonInUpdatingStatusGetSwept verifies that daemons stuck in
// "updating" status are swept to "offline" by MarkStaleDaemonsOffline.
// This was a bug: the sweeper only checked status='online'.
func TestStaleDaemonInUpdatingStatusGetsSwept(t *testing.T) {
	resp := registerDaemon(t, "stale-updating-daemon", "stale-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	ctx := context.Background()

	// Simulate: daemon is updating and then crashes (no more heartbeats).
	// Set status to 'updating' and backdate last_seen_at.
	testPool.Exec(ctx, `UPDATE daemon SET status = 'updating', last_seen_at = now() - interval '2 minutes' WHERE id = $1`, resp.Daemon.ID)
	testPool.Exec(ctx, `UPDATE agent_runtime SET status = 'updating', last_seen_at = now() - interval '2 minutes' WHERE daemon_ref = $1`, resp.Daemon.ID)

	// Verify precondition.
	var status string
	testPool.QueryRow(ctx, `SELECT status FROM daemon WHERE id = $1`, resp.Daemon.ID).Scan(&status)
	if status != "updating" {
		t.Fatalf("precondition: expected daemon status 'updating', got '%s'", status)
	}

	// Run the sweeper query with a threshold of 45 seconds.
	queries := db.New(testPool)
	staleDaemons, err := queries.MarkStaleDaemonsOffline(ctx, 45.0)
	if err != nil {
		t.Fatalf("MarkStaleDaemonsOffline failed: %v", err)
	}

	// Verify our daemon was swept.
	found := false
	for _, sd := range staleDaemons {
		if sd.ID == parseUUID(resp.Daemon.ID) {
			found = true
		}
	}
	if !found {
		t.Fatal("expected stale 'updating' daemon to be swept to offline")
	}

	// Verify DB status is now offline.
	testPool.QueryRow(ctx, `SELECT status FROM daemon WHERE id = $1`, resp.Daemon.ID).Scan(&status)
	if status != "offline" {
		t.Fatalf("expected daemon status 'offline' after sweep, got '%s'", status)
	}

	// Now sweep runtimes too.
	staleRuntimes, err := queries.MarkStaleRuntimesOffline(ctx, 45.0)
	if err != nil {
		t.Fatalf("MarkStaleRuntimesOffline failed: %v", err)
	}

	wsRuntimes := resp.runtimesInTestWorkspace()
	if len(wsRuntimes) == 0 {
		t.Fatal("expected at least one runtime projected into test workspace")
	}
	runtimeSwept := false
	for _, sr := range staleRuntimes {
		rtID := fmt.Sprintf("%x-%x-%x-%x-%x", sr.ID.Bytes[0:4], sr.ID.Bytes[4:6], sr.ID.Bytes[6:8], sr.ID.Bytes[8:10], sr.ID.Bytes[10:16])
		if rtID == wsRuntimes[0].ID {
			runtimeSwept = true
		}
	}
	if !runtimeSwept {
		t.Fatal("expected stale 'updating' runtime to be swept to offline")
	}

	testPool.QueryRow(ctx, `SELECT status FROM agent_runtime WHERE daemon_ref = $1`, resp.Daemon.ID).Scan(&status)
	if status != "offline" {
		t.Fatalf("expected runtime status 'offline' after sweep, got '%s'", status)
	}
}

// TestStaleOnlineDaemonGetsSwept verifies the original sweep behavior
// for daemons in "online" status with stale heartbeats.
func TestStaleOnlineDaemonGetsSwept(t *testing.T) {
	resp := registerDaemon(t, "stale-online-daemon", "stale-online-machine", []map[string]any{
		{"type": "codex", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	ctx := context.Background()

	// Backdate last_seen_at to simulate missed heartbeats.
	testPool.Exec(ctx, `UPDATE daemon SET last_seen_at = now() - interval '2 minutes' WHERE id = $1`, resp.Daemon.ID)
	testPool.Exec(ctx, `UPDATE agent_runtime SET last_seen_at = now() - interval '2 minutes' WHERE daemon_ref = $1`, resp.Daemon.ID)

	queries := db.New(testPool)
	staleDaemons, err := queries.MarkStaleDaemonsOffline(ctx, 45.0)
	if err != nil {
		t.Fatalf("MarkStaleDaemonsOffline failed: %v", err)
	}

	found := false
	for _, sd := range staleDaemons {
		if sd.ID == parseUUID(resp.Daemon.ID) {
			found = true
		}
	}
	if !found {
		t.Fatal("expected stale 'online' daemon to be swept to offline")
	}

	var status string
	testPool.QueryRow(ctx, `SELECT status FROM daemon WHERE id = $1`, resp.Daemon.ID).Scan(&status)
	if status != "offline" {
		t.Fatalf("expected daemon status 'offline', got '%s'", status)
	}
}

// TestRecentDaemonNotSwept verifies that daemons with recent heartbeats
// are NOT swept, regardless of status.
func TestRecentDaemonNotSwept(t *testing.T) {
	resp := registerDaemon(t, "recent-daemon", "recent-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	// last_seen_at is now() from registration — well within threshold.
	queries := db.New(testPool)
	staleDaemons, err := queries.MarkStaleDaemonsOffline(context.Background(), 45.0)
	if err != nil {
		t.Fatalf("MarkStaleDaemonsOffline failed: %v", err)
	}

	for _, sd := range staleDaemons {
		if sd.ID == parseUUID(resp.Daemon.ID) {
			t.Fatal("recently registered daemon should NOT be swept")
		}
	}
}

// TestOfflineDaemonNotSwept verifies that already-offline daemons
// are not touched by the sweeper (no duplicate events).
func TestOfflineDaemonNotSwept(t *testing.T) {
	resp := registerDaemon(t, "already-offline-daemon", "offline-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	ctx := context.Background()

	// Mark offline and backdate.
	testPool.Exec(ctx, `UPDATE daemon SET status = 'offline', last_seen_at = now() - interval '10 minutes' WHERE id = $1`, resp.Daemon.ID)

	queries := db.New(testPool)
	staleDaemons, err := queries.MarkStaleDaemonsOffline(ctx, 45.0)
	if err != nil {
		t.Fatalf("MarkStaleDaemonsOffline failed: %v", err)
	}

	for _, sd := range staleDaemons {
		if sd.ID == parseUUID(resp.Daemon.ID) {
			t.Fatal("already-offline daemon should NOT appear in sweep results")
		}
	}
}

// TestDaemonStatusConsistencyWithRuntimes verifies that SetDaemonAndRuntimesOffline
// atomically sets both daemon and all runtimes to offline.
func TestDaemonStatusConsistencyWithRuntimes(t *testing.T) {
	resp := registerDaemon(t, "consistency-daemon", "consistency-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		{"type": "codex", "version": "2.0.0", "status": "online", "auth_status": "ready"},
		{"type": "opencode", "version": "3.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	ctx := context.Background()
	queries := db.New(testPool)

	// Use SetDaemonAndRuntimesOffline (what deregister calls).
	err := queries.SetDaemonAndRuntimesOffline(ctx, parseUUID(resp.Daemon.ID))
	if err != nil {
		t.Fatalf("SetDaemonAndRuntimesOffline failed: %v", err)
	}

	// Verify daemon is offline.
	var daemonStatus string
	testPool.QueryRow(ctx, `SELECT status FROM daemon WHERE id = $1`, resp.Daemon.ID).Scan(&daemonStatus)
	if daemonStatus != "offline" {
		t.Fatalf("expected daemon 'offline', got '%s'", daemonStatus)
	}

	// Verify ALL runtimes are offline.
	var onlineCount int
	testPool.QueryRow(ctx, `SELECT count(*) FROM agent_runtime WHERE daemon_ref = $1 AND status != 'offline'`, resp.Daemon.ID).Scan(&onlineCount)
	if onlineCount != 0 {
		t.Fatalf("expected 0 non-offline runtimes, got %d", onlineCount)
	}
}

// TestDaemonUpdatingStatusConsistencyWithRuntimes verifies that
// SetDaemonAndRuntimesUpdating atomically sets both to "updating".
func TestDaemonUpdatingStatusConsistencyWithRuntimes(t *testing.T) {
	resp := registerDaemon(t, "updating-daemon", "updating-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		{"type": "codex", "version": "2.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	ctx := context.Background()
	queries := db.New(testPool)

	// Set updating.
	err := queries.SetDaemonAndRuntimesUpdating(ctx, parseUUID(resp.Daemon.ID))
	if err != nil {
		t.Fatalf("SetDaemonAndRuntimesUpdating failed: %v", err)
	}

	// Verify daemon is updating.
	var daemonStatus string
	testPool.QueryRow(ctx, `SELECT status FROM daemon WHERE id = $1`, resp.Daemon.ID).Scan(&daemonStatus)
	if daemonStatus != "updating" {
		t.Fatalf("expected daemon 'updating', got '%s'", daemonStatus)
	}

	// Verify ALL runtimes are updating.
	var updatingCount int
	testPool.QueryRow(ctx, `SELECT count(*) FROM agent_runtime WHERE daemon_ref = $1 AND status = 'updating'`, resp.Daemon.ID).Scan(&updatingCount)
	if updatingCount != 2 {
		t.Fatalf("expected 2 runtimes in 'updating', got %d", updatingCount)
	}
}

// TestHeartbeatAfterUpdatingRestoresOnline verifies that a heartbeat
// after the daemon was in "updating" status restores everything to "online".
func TestHeartbeatAfterUpdatingRestoresOnline(t *testing.T) {
	resp := registerDaemon(t, "hb-updating-daemon", "hb-updating-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	ctx := context.Background()
	queries := db.New(testPool)

	// Set to updating.
	queries.SetDaemonAndRuntimesUpdating(ctx, parseUUID(resp.Daemon.ID))

	// Heartbeat.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/heartbeat", map[string]any{
		"daemon_id": resp.Daemon.ID,
	})
	testHandler.DaemonHeartbeat(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonHeartbeat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify daemon back to online.
	var status string
	testPool.QueryRow(ctx, `SELECT status FROM daemon WHERE id = $1`, resp.Daemon.ID).Scan(&status)
	if status != "online" {
		t.Fatalf("expected daemon 'online' after heartbeat, got '%s'", status)
	}

	// Verify runtime back to online.
	testPool.QueryRow(ctx, `SELECT status FROM agent_runtime WHERE daemon_ref = $1`, resp.Daemon.ID).Scan(&status)
	if status != "online" {
		t.Fatalf("expected runtime 'online' after heartbeat, got '%s'", status)
	}
}

// TestEffectiveAuthStatusWithWorkspaceAPIKey verifies that a runtime
// reported as "unauthenticated" shows "ready" when the workspace has
// an API key configured for that provider.
func TestEffectiveAuthStatusWithWorkspaceAPIKey(t *testing.T) {
	ctx := context.Background()

	// Set workspace provider settings with a Claude API key.
	settings := `{"providers":{"claude":{"enabled":true,"api_key":"sk-test-key"}}}`
	testPool.Exec(ctx, `UPDATE workspace SET settings = $1::jsonb WHERE id = $2`, settings, testWorkspaceID)
	t.Cleanup(func() {
		testPool.Exec(ctx, `UPDATE workspace SET settings = '{}'::jsonb WHERE id = $1`, testWorkspaceID)
	})

	resp := registerDaemon(t, "effective-auth-daemon", "auth-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "unauthenticated"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	wsRuntimes := resp.runtimesInTestWorkspace()
	if len(wsRuntimes) == 0 {
		t.Fatal("expected at least one runtime projected into test workspace")
	}
	// The registration response should already compute effective auth.
	if wsRuntimes[0].AuthStatus != "ready" {
		t.Fatalf("expected effective auth_status 'ready' (workspace has API key), got '%s'", wsRuntimes[0].AuthStatus)
	}

	// ListAgentRuntimes should also return effective auth.
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/runtimes?workspace_id="+testWorkspaceID, nil)
	testHandler.ListAgentRuntimes(w, req)
	var runtimes []AgentRuntimeResponse
	json.NewDecoder(w.Body).Decode(&runtimes)

	for _, rt := range runtimes {
		if rt.ID == wsRuntimes[0].ID {
			if rt.AuthStatus != "ready" {
				t.Fatalf("ListRuntimes: expected effective auth_status 'ready', got '%s'", rt.AuthStatus)
			}
		}
	}
}

// TestDaemonAutoEnabledForMemberWorkspaces verifies that a first-time
// daemon registration auto-enables the daemon for every workspace the
// authenticated user is currently a member of.
func TestDaemonAutoEnabledForMemberWorkspaces(t *testing.T) {
	resp := registerDaemon(t, "auto-enable-daemon", "auto-machine", []map[string]any{
		{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	// Should include the user's test workspace.
	var found bool
	for _, ws := range resp.Workspaces {
		if ws.WorkspaceID == testWorkspaceID && ws.Enabled {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected daemon to be auto-enabled for test workspace; got workspaces: %+v", resp.Workspaces)
	}

	// daemon_workspace row must exist and be enabled.
	ctx := context.Background()
	var enabled bool
	if err := testPool.QueryRow(ctx,
		`SELECT enabled FROM daemon_workspace WHERE daemon_id = $1 AND workspace_id = $2`,
		resp.Daemon.ID, testWorkspaceID,
	).Scan(&enabled); err != nil {
		t.Fatalf("expected daemon_workspace row, got error: %v", err)
	}
	if !enabled {
		t.Fatal("expected daemon to be enabled for workspace")
	}
}

// TestDisableDaemonForWorkspacePreventsDispatch verifies that disabling a
// daemon for a workspace excludes its runtimes from FindAvailableRuntime
// lookups even though the runtime row still exists. Uses a provider not
// served by the workspace's pre-existing cloud runtime so dispatch goes
// through the daemon-side path.
func TestDisableDaemonForWorkspacePreventsDispatch(t *testing.T) {
	const provider = "claude" // test fixture uses 'codex' for the cloud runtime
	resp := registerDaemon(t, "dispatch-disable-daemon", "dispatch-machine", []map[string]any{
		{"type": provider, "version": "1.0.0", "status": "online", "auth_status": "ready"},
	})
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	wsRuntimes := resp.runtimesInTestWorkspace()
	if len(wsRuntimes) == 0 {
		t.Fatal("expected at least one runtime projected into test workspace")
	}

	ctx := context.Background()
	queries := db.New(testPool)

	// Sanity: the runtime is dispatchable while enabled.
	rt, err := queries.FindAvailableRuntimeForProvider(ctx, db.FindAvailableRuntimeForProviderParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Provider:    provider,
	})
	if err != nil {
		t.Fatalf("expected to find runtime while enabled, got error: %v", err)
	}
	if uuidToString(rt.ID) != wsRuntimes[0].ID {
		t.Fatalf("found unexpected runtime: %s vs %s", uuidToString(rt.ID), wsRuntimes[0].ID)
	}

	// Disable via API.
	req := newRequest("PUT", "/api/me/daemons/"+resp.Daemon.ID+"/workspaces/"+testWorkspaceID, map[string]any{
		"enabled": false,
	})
	req = withURLParam(req, "daemonId", resp.Daemon.ID)
	req = withURLParam(req, "workspaceId", testWorkspaceID)
	w := httptest.NewRecorder()
	testHandler.SetMyDaemonWorkspaceEnabled(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SetMyDaemonWorkspaceEnabled: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// FindAvailableRuntime must now skip this runtime.
	if _, err := queries.FindAvailableRuntimeForProvider(ctx, db.FindAvailableRuntimeForProviderParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Provider:    provider,
	}); err == nil {
		t.Fatal("expected no runtime after disable, but one was returned")
	}
}

// TestRemovingMemberBlocksDispatch verifies that removing a daemon owner from
// a workspace prevents their daemon from being selected for tasks even when
// the assignment row says enabled — the dispatch query joins through member.
func TestRemovingMemberBlocksDispatch(t *testing.T) {
	ctx := context.Background()

	// Create a secondary user and workspace; make user a member, register a
	// daemon owned by that user, then remove the membership.
	var secondaryUserID, secondaryWorkspaceID string
	if err := testPool.QueryRow(ctx,
		`INSERT INTO "user" (name, email) VALUES ('Secondary', 'daemon-membership-test@multica.ai') RETURNING id`,
	).Scan(&secondaryUserID); err != nil {
		t.Fatalf("create secondary user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, secondaryUserID)
	})

	if err := testPool.QueryRow(ctx,
		`INSERT INTO workspace (name, slug, description, issue_prefix) VALUES ('Sec WS', 'daemon-memb-test', '', 'SEC') RETURNING id`,
	).Scan(&secondaryWorkspaceID); err != nil {
		t.Fatalf("create secondary workspace: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, secondaryWorkspaceID)
	})

	if _, err := testPool.Exec(ctx,
		`INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`,
		secondaryWorkspaceID, secondaryUserID,
	); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	// Register a daemon owned by secondaryUser via direct headers.
	body := map[string]any{
		"daemon_id":   "membership-test-daemon",
		"device_name": "membership-machine",
		"runtimes": []map[string]any{
			{"type": "codex", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		},
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/api/daemon/register", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", secondaryUserID)
	w := httptest.NewRecorder()
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonRegister: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp daemonRegisterResponse
	json.NewDecoder(w.Body).Decode(&resp)
	t.Cleanup(func() { cleanupDaemon(t, resp.Daemon.ID) })

	queries := db.New(testPool)

	// Sanity: dispatch finds the runtime while the owner is still a member.
	if _, err := queries.FindAvailableRuntimeForProvider(ctx, db.FindAvailableRuntimeForProviderParams{
		WorkspaceID: parseUUID(secondaryWorkspaceID),
		Provider:    "codex",
	}); err != nil {
		t.Fatalf("expected runtime while owner is member, got error: %v", err)
	}

	// Remove the daemon owner from the workspace.
	testPool.Exec(ctx,
		`DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`,
		secondaryWorkspaceID, secondaryUserID,
	)

	// Dispatch must now reject the daemon even though daemon_workspace.enabled is TRUE.
	if _, err := queries.FindAvailableRuntimeForProvider(ctx, db.FindAvailableRuntimeForProviderParams{
		WorkspaceID: parseUUID(secondaryWorkspaceID),
		Provider:    "codex",
	}); err == nil {
		t.Fatal("expected no runtime after owner was removed from workspace, but one was returned")
	}
}

// TestDaemonMutationsRequireOwnership verifies that workspace members who
// don't own a daemon cannot modify it through the workspace-scoped routes
// (PATCH / archive / restore). Only the daemon's owner may manage it.
func TestDaemonMutationsRequireOwnership(t *testing.T) {
	ctx := context.Background()

	// Register a daemon owned by a secondary user.
	var ownerID string
	if err := testPool.QueryRow(ctx,
		`INSERT INTO "user" (name, email) VALUES ('Daemon Owner', 'daemon-owner-test@multica.ai') RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, ownerID)
	})

	// Make the owner a member of the test workspace so the daemon assignment
	// auto-enables there — gives the non-owner test user a shared workspace
	// to attack from.
	if _, err := testPool.Exec(ctx,
		`INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'admin')`,
		testWorkspaceID, ownerID,
	); err != nil {
		t.Fatalf("create owner membership: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, testWorkspaceID, ownerID)
	})

	body := map[string]any{
		"daemon_id":   "ownership-test-daemon",
		"device_name": "ownership-machine",
		"env_vars":    map[string]string{"SECRET": "owner-only"},
		"runtimes": []map[string]any{
			{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		},
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/api/daemon/register", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", ownerID)
	w := httptest.NewRecorder()
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonRegister: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp daemonRegisterResponse
	json.NewDecoder(w.Body).Decode(&resp)
	daemonUUID := resp.Daemon.ID
	t.Cleanup(func() { cleanupDaemon(t, daemonUUID) })

	// PATCH as a workspace member who is not the owner must fail.
	w = httptest.NewRecorder()
	req = newRequest("PATCH", "/api/daemons/"+daemonUUID, map[string]any{"device_name": "hijacked"})
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.UpdateDaemon(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("UpdateDaemon by non-owner: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Archive must fail too.
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/daemons/"+daemonUUID+"/archive", nil)
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.ArchiveDaemon(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("ArchiveDaemon by non-owner: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Restore must fail too.
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/daemons/"+daemonUUID+"/restore", nil)
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.RestoreDaemon(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("RestoreDaemon by non-owner: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Confirm the daemon was not mutated.
	var deviceName string
	var archivedAt *string
	if err := testPool.QueryRow(ctx,
		`SELECT device_name, archived_at::text FROM daemon WHERE id = $1`, daemonUUID,
	).Scan(&deviceName, &archivedAt); err != nil {
		t.Fatalf("verify daemon row: %v", err)
	}
	if deviceName != "ownership-machine" {
		t.Fatalf("device_name was mutated by non-owner: %q", deviceName)
	}
	if archivedAt != nil {
		t.Fatalf("daemon was archived by non-owner: %v", *archivedAt)
	}

	// As the owner the same request must succeed.
	w = httptest.NewRecorder()
	req = httptest.NewRequest("PATCH", "/api/daemons/"+daemonUUID, bytes.NewBufferString(`{"device_name":"renamed-by-owner"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", ownerID)
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.UpdateDaemon(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateDaemon by owner: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDaemonEnvHiddenWhenDisabled verifies that env vars are not readable by
// workspace members once the owner disables the daemon for that workspace,
// even though the daemon_workspace row is retained.
func TestDaemonEnvHiddenWhenDisabled(t *testing.T) {
	ctx := context.Background()

	var ownerID string
	if err := testPool.QueryRow(ctx,
		`INSERT INTO "user" (name, email) VALUES ('Env Owner', 'daemon-env-test@multica.ai') RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, ownerID)
	})

	if _, err := testPool.Exec(ctx,
		`INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'admin')`,
		testWorkspaceID, ownerID,
	); err != nil {
		t.Fatalf("create owner membership: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, testWorkspaceID, ownerID)
	})

	body := map[string]any{
		"daemon_id":   "env-test-daemon",
		"device_name": "env-machine",
		"env_vars":    map[string]string{"SECRET": "do-not-leak"},
		"runtimes": []map[string]any{
			{"type": "claude", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		},
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/api/daemon/register", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", ownerID)
	w := httptest.NewRecorder()
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonRegister: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp daemonRegisterResponse
	json.NewDecoder(w.Body).Decode(&resp)
	daemonUUID := resp.Daemon.ID
	t.Cleanup(func() { cleanupDaemon(t, daemonUUID) })

	// While enabled, the workspace member can see env vars.
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/daemons/"+daemonUUID+"/env", nil)
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.GetDaemonEnv(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDaemonEnv while enabled: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var env map[string]string
	json.NewDecoder(w.Body).Decode(&env)
	if env["SECRET"] != "do-not-leak" {
		t.Fatalf("expected SECRET=do-not-leak while enabled, got %v", env)
	}

	// Owner disables the daemon for the workspace.
	if _, err := testPool.Exec(ctx,
		`UPDATE daemon_workspace SET enabled = FALSE WHERE daemon_id = $1 AND workspace_id = $2`,
		daemonUUID, testWorkspaceID,
	); err != nil {
		t.Fatalf("disable assignment: %v", err)
	}

	// Workspace member must now be locked out of env vars.
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/daemons/"+daemonUUID+"/env", nil)
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.GetDaemonEnv(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetDaemonEnv after disable: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Owner still has access.
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/daemons/"+daemonUUID+"/env", nil)
	req.Header.Set("X-User-ID", ownerID)
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.GetDaemonEnv(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDaemonEnv by owner after disable: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDaemonReadAccessRevokedWhenOwnerLeavesWorkspace verifies that a daemon
// disappears from workspace-scoped list and read-only endpoints once its
// owner is no longer a member of that workspace.
func TestDaemonReadAccessRevokedWhenOwnerLeavesWorkspace(t *testing.T) {
	ctx := context.Background()

	var ownerID, workspaceID string
	if err := testPool.QueryRow(ctx,
		`INSERT INTO "user" (name, email) VALUES ('Read Owner', 'daemon-read-owner@multica.ai') RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, ownerID)
	})

	if err := testPool.QueryRow(ctx,
		`INSERT INTO workspace (name, slug, description, issue_prefix) VALUES ('Read Access WS', 'daemon-read-access', '', 'DRA') RETURNING id`,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, workspaceID)
	})

	if _, err := testPool.Exec(ctx,
		`INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner'), ($1, $3, 'admin')`,
		workspaceID, ownerID, testUserID,
	); err != nil {
		t.Fatalf("create memberships: %v", err)
	}

	body := map[string]any{
		"daemon_id":   "read-access-daemon",
		"device_name": "read-machine",
		"env_vars":    map[string]string{"SECRET": "visible-while-member"},
		"runtimes": []map[string]any{
			{"type": "codex", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		},
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/api/daemon/register", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", ownerID)
	w := httptest.NewRecorder()
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonRegister: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp daemonRegisterResponse
	json.NewDecoder(w.Body).Decode(&resp)
	daemonUUID := resp.Daemon.ID
	t.Cleanup(func() { cleanupDaemon(t, daemonUUID) })

	newWorkspaceMemberRequest := func(method, path string) *http.Request {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("X-User-ID", testUserID)
		req.Header.Set("X-Workspace-ID", workspaceID)
		return req
	}

	// While the owner is still a member, the workspace member can list and read it.
	w = httptest.NewRecorder()
	req = newWorkspaceMemberRequest("GET", "/api/daemons")
	testHandler.ListDaemons(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListDaemons while owner member: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listed []DaemonResponse
	if err := json.NewDecoder(w.Body).Decode(&listed); err != nil {
		t.Fatalf("decode daemon list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != daemonUUID {
		t.Fatalf("expected daemon to be listed while owner is member, got %+v", listed)
	}

	w = httptest.NewRecorder()
	req = newWorkspaceMemberRequest("GET", "/api/daemons/"+daemonUUID)
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.GetDaemonByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDaemonByID while owner member: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newWorkspaceMemberRequest("GET", "/api/daemons/"+daemonUUID+"/env")
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.GetDaemonEnv(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDaemonEnv while owner member: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Once the owner leaves the workspace, the daemon must disappear everywhere.
	if _, err := testPool.Exec(ctx,
		`DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, ownerID,
	); err != nil {
		t.Fatalf("remove owner membership: %v", err)
	}

	w = httptest.NewRecorder()
	req = newWorkspaceMemberRequest("GET", "/api/daemons")
	testHandler.ListDaemons(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListDaemons after owner removal: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	listed = nil
	if err := json.NewDecoder(w.Body).Decode(&listed); err != nil {
		t.Fatalf("decode daemon list after removal: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected daemon list to be empty after owner removal, got %+v", listed)
	}

	w = httptest.NewRecorder()
	req = newWorkspaceMemberRequest("GET", "/api/daemons/"+daemonUUID)
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.GetDaemonByID(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetDaemonByID after owner removal: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newWorkspaceMemberRequest("GET", "/api/daemons/"+daemonUUID+"/env")
	req = withURLParam(req, "daemonId", daemonUUID)
	testHandler.GetDaemonEnv(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetDaemonEnv after owner removal: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRuntimeVisibilityRespectsAssignmentAndMembership verifies that local
// agent_runtime rows disappear from list / read / ping / update endpoints
// once the daemon is disabled for the workspace or its owner leaves, and
// that heartbeat does not silently bring them back online.
func TestRuntimeVisibilityRespectsAssignmentAndMembership(t *testing.T) {
	ctx := context.Background()

	var ownerID, workspaceID string
	if err := testPool.QueryRow(ctx,
		`INSERT INTO "user" (name, email) VALUES ('Runtime Owner', 'runtime-owner-test@multica.ai') RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, ownerID)
	})

	if err := testPool.QueryRow(ctx,
		`INSERT INTO workspace (name, slug, description, issue_prefix) VALUES ('RT Vis WS', 'runtime-vis-test', '', 'RTV') RETURNING id`,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, workspaceID)
	})

	if _, err := testPool.Exec(ctx,
		`INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner'), ($1, $3, 'admin')`,
		workspaceID, ownerID, testUserID,
	); err != nil {
		t.Fatalf("create memberships: %v", err)
	}

	// Register a daemon owned by ownerID; auto-enables in the new workspace.
	body := map[string]any{
		"daemon_id":   "rt-vis-daemon",
		"device_name": "rt-vis-machine",
		"runtimes": []map[string]any{
			{"type": "codex", "version": "1.0.0", "status": "online", "auth_status": "ready"},
		},
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/api/daemon/register", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", ownerID)
	w := httptest.NewRecorder()
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonRegister: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp daemonRegisterResponse
	json.NewDecoder(w.Body).Decode(&resp)
	daemonUUID := resp.Daemon.ID
	t.Cleanup(func() { cleanupDaemon(t, daemonUUID) })

	// Find the runtime projected into the new workspace.
	var runtimeID string
	for _, ws := range resp.Workspaces {
		if ws.WorkspaceID == workspaceID && len(ws.Runtimes) > 0 {
			runtimeID = ws.Runtimes[0].ID
		}
	}
	if runtimeID == "" {
		t.Fatalf("expected runtime projected into workspace; got %+v", resp.Workspaces)
	}

	wsMemberRequest := func(method, path string, body any) *http.Request {
		var b bytes.Buffer
		if body != nil {
			json.NewEncoder(&b).Encode(body)
		}
		req := httptest.NewRequest(method, path, &b)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", testUserID)
		req.Header.Set("X-Workspace-ID", workspaceID)
		return req
	}

	listRuntimes := func() []AgentRuntimeResponse {
		w := httptest.NewRecorder()
		req := wsMemberRequest("GET", "/api/runtimes", nil)
		testHandler.ListAgentRuntimes(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("ListAgentRuntimes: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var out []AgentRuntimeResponse
		json.NewDecoder(w.Body).Decode(&out)
		return out
	}

	containsRuntime := func(list []AgentRuntimeResponse, id string) bool {
		for _, rt := range list {
			if rt.ID == id {
				return true
			}
		}
		return false
	}

	// Baseline: workspace member can list the runtime, fetch usage/activity,
	// and queue a ping/update.
	if !containsRuntime(listRuntimes(), runtimeID) {
		t.Fatal("baseline: runtime should be listed while daemon is enabled and owner is member")
	}

	w = httptest.NewRecorder()
	req = wsMemberRequest("GET", "/api/runtimes/"+runtimeID+"/usage", nil)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.GetRuntimeUsage(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetRuntimeUsage baseline: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = wsMemberRequest("POST", "/api/runtimes/"+runtimeID+"/ping", nil)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.InitiatePing(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("InitiatePing baseline: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = wsMemberRequest("POST", "/api/runtimes/"+runtimeID+"/update", map[string]any{"target_version": "1.2.3"})
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.InitiateUpdate(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("InitiateUpdate baseline: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Disable the daemon for this workspace and force the runtime offline as
	// the disable handler would have done.
	if _, err := testPool.Exec(ctx,
		`UPDATE daemon_workspace SET enabled = FALSE WHERE daemon_id = $1 AND workspace_id = $2`,
		daemonUUID, workspaceID,
	); err != nil {
		t.Fatalf("disable assignment: %v", err)
	}
	if _, err := testPool.Exec(ctx,
		`UPDATE agent_runtime SET status = 'offline' WHERE id = $1`, runtimeID,
	); err != nil {
		t.Fatalf("force runtime offline: %v", err)
	}

	if containsRuntime(listRuntimes(), runtimeID) {
		t.Fatal("expected runtime to be hidden after daemon disabled for workspace")
	}

	w = httptest.NewRecorder()
	req = wsMemberRequest("GET", "/api/runtimes/"+runtimeID+"/usage", nil)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.GetRuntimeUsage(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetRuntimeUsage after disable: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = wsMemberRequest("POST", "/api/runtimes/"+runtimeID+"/ping", nil)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.InitiatePing(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("InitiatePing after disable: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = wsMemberRequest("POST", "/api/runtimes/"+runtimeID+"/update", map[string]any{"target_version": "1.2.4"})
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.InitiateUpdate(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("InitiateUpdate after disable: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Heartbeat from the owner must NOT bring the offline runtime in the
	// disabled workspace back online.
	hbBody := map[string]any{"daemon_id": daemonUUID}
	var hbBuf bytes.Buffer
	json.NewEncoder(&hbBuf).Encode(hbBody)
	hbReq := httptest.NewRequest("POST", "/api/daemon/heartbeat", &hbBuf)
	hbReq.Header.Set("Content-Type", "application/json")
	hbReq.Header.Set("X-User-ID", ownerID)
	hbW := httptest.NewRecorder()
	testHandler.DaemonHeartbeat(hbW, hbReq)
	if hbW.Code != http.StatusOK {
		t.Fatalf("DaemonHeartbeat: expected 200, got %d: %s", hbW.Code, hbW.Body.String())
	}
	var status string
	if err := testPool.QueryRow(ctx,
		`SELECT status FROM agent_runtime WHERE id = $1`, runtimeID,
	).Scan(&status); err != nil {
		t.Fatalf("read runtime status: %v", err)
	}
	if status != "offline" {
		t.Fatalf("heartbeat must not revive runtime in disabled workspace, got status=%q", status)
	}

	// Re-enable the daemon and remove the owner from the workspace — the
	// runtime should still be hidden / inoperable because the owner is gone.
	if _, err := testPool.Exec(ctx,
		`UPDATE daemon_workspace SET enabled = TRUE WHERE daemon_id = $1 AND workspace_id = $2`,
		daemonUUID, workspaceID,
	); err != nil {
		t.Fatalf("re-enable assignment: %v", err)
	}
	if _, err := testPool.Exec(ctx,
		`DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, ownerID,
	); err != nil {
		t.Fatalf("remove owner membership: %v", err)
	}

	if containsRuntime(listRuntimes(), runtimeID) {
		t.Fatal("expected runtime to be hidden after owner leaves workspace")
	}

	w = httptest.NewRecorder()
	req = wsMemberRequest("GET", "/api/runtimes/"+runtimeID+"/usage", nil)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.GetRuntimeUsage(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetRuntimeUsage after owner left: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = wsMemberRequest("POST", "/api/runtimes/"+runtimeID+"/ping", nil)
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.InitiatePing(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("InitiatePing after owner left: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Heartbeat must still leave the runtime in the workspace-owner-less
	// workspace offline.
	hbBuf.Reset()
	json.NewEncoder(&hbBuf).Encode(hbBody)
	hbReq = httptest.NewRequest("POST", "/api/daemon/heartbeat", &hbBuf)
	hbReq.Header.Set("Content-Type", "application/json")
	hbReq.Header.Set("X-User-ID", ownerID)
	hbW = httptest.NewRecorder()
	testHandler.DaemonHeartbeat(hbW, hbReq)
	if hbW.Code != http.StatusOK {
		t.Fatalf("DaemonHeartbeat after owner left: expected 200, got %d: %s", hbW.Code, hbW.Body.String())
	}
	if err := testPool.QueryRow(ctx,
		`SELECT status FROM agent_runtime WHERE id = $1`, runtimeID,
	).Scan(&status); err != nil {
		t.Fatalf("read runtime status: %v", err)
	}
	if status != "offline" {
		t.Fatalf("heartbeat must not revive runtime after owner left, got status=%q", status)
	}
}

func TestWebhookCreateIssueSubscribers(t *testing.T) {
	ctx := context.Background()

	// Create a subscriber user.
	var subscriberUserID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO "user" (name, email) VALUES ('Webhook Sub', 'webhook-sub-test@multica.ai') RETURNING id
	`).Scan(&subscriberUserID); err != nil {
		t.Fatalf("create subscriber user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = 'webhook-sub-test@multica.ai'`)
	})

	// Get the test agent ID.
	var agentID string
	if err := testPool.QueryRow(ctx, `
		SELECT id FROM agent WHERE workspace_id = $1 LIMIT 1
	`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("get test agent: %v", err)
	}

	// Create a webhook record.
	var webhookID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO webhook (workspace_id, name, source_type, token_hash, token_prefix, dedup_window_seconds, created_by, status)
		VALUES ($1, 'subscriber-test-wh', 'standard', 'testhash-sub', 'sub_', 600, $2, 'active')
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&webhookID); err != nil {
		t.Fatalf("create webhook: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM webhook WHERE id = $1`, webhookID)
	})

	// Build the webhook record as a db.Webhook for the handler call.
	wkRow, err := db.New(testPool).GetWebhook(ctx, parseUUID(webhookID))
	if err != nil {
		t.Fatalf("get webhook: %v", err)
	}

	cfg := CreateIssueActionConfig{
		AgentID:       agentID,
		SubscriberIDs: []string{subscriberUserID},
	}

	evt := whpkg.Event{
		Type: "standard",
		Data: map[string]string{"title": "Sub test issue"},
	}
	issueUUID, err := testHandler.executeCreateIssueAction(ctx, wkRow, cfg, evt, db.IssueLink{}, false)
	if err != nil {
		t.Fatalf("executeCreateIssueAction: %v", err)
	}
	issueIDStr := uuidToString(issueUUID)
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueIDStr)
	})

	// Verify the subscriber was inserted.
	var count int
	if err := testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM issue_subscriber
		WHERE issue_id = $1::uuid AND user_type = 'member' AND user_id = $2::uuid
	`, issueIDStr, subscriberUserID).Scan(&count); err != nil {
		t.Fatalf("query issue_subscriber: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 subscriber row, got %d", count)
	}
}
