package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/nullne/multica/server/internal/codeagent"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

func TestAgentMaxConcurrentTasks_CreateDefault(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":      "concurrency-test-agent-default",
		"providers": []string{"claude"},
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp AgentResponse
	json.NewDecoder(w.Body).Decode(&resp)

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, resp.ID)
	})

	if resp.MaxConcurrentTasks != 6 {
		t.Fatalf("expected default max_concurrent_tasks=6, got %d", resp.MaxConcurrentTasks)
	}
}

func TestAgentMaxConcurrentTasks_CreateCustom(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":                 "concurrency-test-agent-custom",
		"providers":            []string{"claude"},
		"max_concurrent_tasks": 3,
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp AgentResponse
	json.NewDecoder(w.Body).Decode(&resp)

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, resp.ID)
	})

	if resp.MaxConcurrentTasks != 3 {
		t.Fatalf("expected max_concurrent_tasks=3, got %d", resp.MaxConcurrentTasks)
	}
}

func TestAgentMaxConcurrentTasks_Update(t *testing.T) {
	// Create agent
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":      "concurrency-test-agent-update",
		"providers": []string{"claude"},
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created AgentResponse
	json.NewDecoder(w.Body).Decode(&created)

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, created.ID)
	})

	// Update max_concurrent_tasks
	w = httptest.NewRecorder()
	req = newRequest("PATCH", "/api/agents/"+created.ID, map[string]any{
		"max_concurrent_tasks": 10,
	})
	req = withURLParam(req, "id", created.ID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated AgentResponse
	json.NewDecoder(w.Body).Decode(&updated)

	if updated.MaxConcurrentTasks != 10 {
		t.Fatalf("expected max_concurrent_tasks=10 after update, got %d", updated.MaxConcurrentTasks)
	}
}

func TestAgentMaxConcurrentTasks_InvalidUpdate(t *testing.T) {
	// Create agent
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":      "concurrency-test-agent-invalid",
		"providers": []string{"claude"},
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created AgentResponse
	json.NewDecoder(w.Body).Decode(&created)

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, created.ID)
	})

	// Try to set max_concurrent_tasks=0 (invalid)
	w = httptest.NewRecorder()
	req = newRequest("PATCH", "/api/agents/"+created.ID, map[string]any{
		"max_concurrent_tasks": 0,
	})
	req = withURLParam(req, "id", created.ID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("UpdateAgent with 0: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateAgent_ProvidersChangeClearsStaleDefaultProvider(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":             "default-provider-reset-agent",
		"providers":        []string{"claude", "codex"},
		"default_provider": "codex",
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created AgentResponse
	json.NewDecoder(w.Body).Decode(&created)

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, created.ID)
	})

	if created.DefaultProvider == nil || *created.DefaultProvider != "codex" {
		t.Fatalf("expected default_provider codex at creation, got %#v", created.DefaultProvider)
	}

	w = httptest.NewRecorder()
	req = newRequest("PATCH", "/api/agents/"+created.ID, map[string]any{
		"providers": []string{"claude"},
	})
	req = withURLParam(req, "id", created.ID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated AgentResponse
	json.NewDecoder(w.Body).Decode(&updated)

	if updated.DefaultProvider != nil {
		t.Fatalf("expected default_provider to be cleared, got %#v", updated.DefaultProvider)
	}
	if len(updated.Providers) != 1 || updated.Providers[0] != "claude" {
		t.Fatalf("expected providers to be updated to [claude], got %#v", updated.Providers)
	}
}

func TestCreateAgent_DefaultModelMustBelongToDefaultProvider(t *testing.T) {
	catalog, _ := codeagent.Catalog("codex")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":             "default-model-agent",
		"providers":        []string{"codex"},
		"default_provider": "codex",
		"default_model":    catalog.DefaultModel,
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp AgentResponse
	json.NewDecoder(w.Body).Decode(&resp)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, resp.ID)
	})

	if resp.DefaultModel == nil || *resp.DefaultModel != catalog.DefaultModel {
		t.Fatalf("expected default_model %q, got %#v", catalog.DefaultModel, resp.DefaultModel)
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":             "invalid-default-model-agent",
		"providers":        []string{"codex"},
		"default_provider": "codex",
		"default_model":    "not-a-tested-model",
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateAgent invalid model: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateAgent_ProvidersChangeClearsStaleDefaultModel(t *testing.T) {
	catalog, _ := codeagent.Catalog("codex")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":             "default-model-reset-agent",
		"providers":        []string{"claude", "codex"},
		"default_provider": "codex",
		"default_model":    catalog.DefaultModel,
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created AgentResponse
	json.NewDecoder(w.Body).Decode(&created)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, created.ID)
	})

	w = httptest.NewRecorder()
	req = newRequest("PATCH", "/api/agents/"+created.ID, map[string]any{
		"providers": []string{"claude"},
	})
	req = withURLParam(req, "id", created.ID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated AgentResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.DefaultModel != nil {
		t.Fatalf("expected default_model to be cleared, got %#v", updated.DefaultModel)
	}
}

func TestCreateIssue_FreezesAgentDefaultModelOnTask(t *testing.T) {
	catalog, _ := codeagent.Catalog("codex")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":             "task-model-agent",
		"providers":        []string{"codex"},
		"default_provider": "codex",
		"default_model":    catalog.DefaultModel,
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var agentResp AgentResponse
	json.NewDecoder(w.Body).Decode(&agentResp)

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Task freezes model",
		"status":        "in_progress",
		"assignee_type": "agent",
		"assignee_id":   agentResp.ID,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issueResp IssueResponse
	json.NewDecoder(w.Body).Decode(&issueResp)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentResp.ID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueResp.ID)
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentResp.ID)
	})

	var requestedModel string
	if err := testPool.QueryRow(context.Background(),
		`SELECT requested_model FROM agent_task_queue WHERE agent_id = $1 AND issue_id = $2`,
		agentResp.ID, issueResp.ID,
	).Scan(&requestedModel); err != nil {
		t.Fatalf("query task requested_model: %v", err)
	}
	if requestedModel != catalog.DefaultModel {
		t.Fatalf("requested_model = %q, want %q", requestedModel, catalog.DefaultModel)
	}
}

func TestCreateIssue_FreezesCatalogDefaultModelWhenAgentModelUnset(t *testing.T) {
	catalog, _ := codeagent.Catalog("codex")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/agents?workspace_id="+testWorkspaceID, map[string]any{
		"name":             "task-catalog-model-agent",
		"providers":        []string{"codex"},
		"default_provider": "codex",
	})
	testHandler.CreateAgent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAgent: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var agentResp AgentResponse
	json.NewDecoder(w.Body).Decode(&agentResp)

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Task freezes catalog default model",
		"status":        "in_progress",
		"assignee_type": "agent",
		"assignee_id":   agentResp.ID,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issueResp IssueResponse
	json.NewDecoder(w.Body).Decode(&issueResp)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentResp.ID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueResp.ID)
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentResp.ID)
	})

	var requestedModel string
	if err := testPool.QueryRow(context.Background(),
		`SELECT requested_model FROM agent_task_queue WHERE agent_id = $1 AND issue_id = $2`,
		agentResp.ID, issueResp.ID,
	).Scan(&requestedModel); err != nil {
		t.Fatalf("query task requested_model: %v", err)
	}
	if requestedModel != catalog.DefaultModel {
		t.Fatalf("requested_model = %q, want catalog default %q", requestedModel, catalog.DefaultModel)
	}
}

func TestCompleteTask_PersistsRequestedAndObservedModels(t *testing.T) {
	ctx := context.Background()

	var runtimeID string
	if err := testPool.QueryRow(ctx,
		`SELECT id FROM agent_runtime WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID,
	).Scan(&runtimeID); err != nil {
		t.Fatalf("get runtime: %v", err)
	}

	var agentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, description, visibility, owner_id, tools, triggers, providers, default_provider)
		VALUES ($1, 'complete-model-agent', '', 'workspace', $2, '[]'::jsonb, '[]'::jsonb, ARRAY['claude'], 'claude')
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position)
		VALUES ($1, 'complete task model', 'in_progress', 'medium', 'member', $2, 98989, 0)
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}

	var taskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, requested_model)
		VALUES ($1, $2, $3, 'running', 0, 'sonnet')
		RETURNING id
	`, agentID, runtimeID, issueID).Scan(&taskID); err != nil {
		t.Fatalf("create task: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
		testPool.Exec(ctx, `DELETE FROM agent WHERE id = $1`, agentID)
	})

	task, err := testHandler.TaskService.CompleteTask(ctx, parseUUID(taskID), []byte(`{"output":"done"}`), "session-1", "/tmp/work", "sonnet", "claude-sonnet-4-20250514")
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if !task.RequestedModel.Valid || task.RequestedModel.String != "sonnet" {
		t.Fatalf("requested_model = %#v, want sonnet", task.RequestedModel)
	}
	if !task.ObservedModel.Valid || task.ObservedModel.String != "claude-sonnet-4-20250514" {
		t.Fatalf("observed_model = %#v, want claude-sonnet-4-20250514", task.ObservedModel)
	}
}

// TestClaimAgentTask_RespectsPerAgentConcurrencyLimit verifies that ClaimAgentTask
// will not dispatch a new task once the agent's max_concurrent_tasks limit is reached.
func TestClaimAgentTask_RespectsPerAgentConcurrencyLimit(t *testing.T) {
	ctx := context.Background()

	// Get the runtime created by setupHandlerTestFixture
	var runtimeID string
	if err := testPool.QueryRow(ctx,
		`SELECT id FROM agent_runtime WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID,
	).Scan(&runtimeID); err != nil {
		t.Fatalf("get runtime: %v", err)
	}

	// Create agent with max_concurrent_tasks=1
	var agentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, description, visibility, owner_id, tools, triggers, providers, max_concurrent_tasks)
		VALUES ($1, 'claim-concurrency-agent', '', 'workspace', $2, '[]'::jsonb, '[]'::jsonb, ARRAY['claude'], 1)
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Create two issues via the handler (which properly increments the workspace issue counter,
	// avoiding duplicate number constraint violations from concurrent test runs).
	createIssue := func(title string) string {
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
			"title":         title,
			"status":        "in_progress",
			"assignee_type": "agent",
			"assignee_id":   agentID,
		})
		testHandler.CreateIssue(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("CreateIssue %q: expected 201, got %d: %s", title, w.Code, w.Body.String())
		}
		var resp IssueResponse
		json.NewDecoder(w.Body).Decode(&resp)
		return resp.ID
	}

	issueID1 := createIssue("Claim Test Issue 1")
	issueID2 := createIssue("Claim Test Issue 2")

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1 OR id = $2`, issueID1, issueID2)
		testPool.Exec(ctx, `DELETE FROM agent WHERE id = $1`, agentID)
	})

	agentUUID := parseUUID(agentID)
	runtimeUUID := parseUUID(runtimeID)
	issue1UUID := parseUUID(issueID1)
	issue2UUID := parseUUID(issueID2)

	// Enqueue tasks for both issues
	if _, err := testHandler.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agentUUID, RuntimeID: runtimeUUID, IssueID: issue1UUID, Priority: 2,
	}); err != nil {
		t.Fatalf("enqueue task 1: %v", err)
	}
	if _, err := testHandler.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agentUUID, RuntimeID: runtimeUUID, IssueID: issue2UUID, Priority: 2,
	}); err != nil {
		t.Fatalf("enqueue task 2: %v", err)
	}

	// First claim: 0 active < max=1, should succeed
	task1, err := testHandler.Queries.ClaimAgentTask(ctx, agentUUID)
	if err != nil {
		t.Fatalf("first ClaimAgentTask: expected success, got %v", err)
	}
	if task1.Status != "dispatched" {
		t.Fatalf("expected status 'dispatched', got %q", task1.Status)
	}

	// Second claim: 1 active == max=1, should return no rows
	_, err = testHandler.Queries.ClaimAgentTask(ctx, agentUUID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("second ClaimAgentTask: expected pgx.ErrNoRows (limit enforced), got %v", err)
	}
}
