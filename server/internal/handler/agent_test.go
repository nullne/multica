package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
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

	// Create two issues assigned to the agent
	var issueID1, issueID2 string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, assignee_type, assignee_id, position)
		VALUES ($1, 'Claim Test Issue 1', 'in_progress', 'medium', 'member', $2, 'agent', $3, 0)
		RETURNING id
	`, testWorkspaceID, testUserID, agentID).Scan(&issueID1); err != nil {
		t.Fatalf("create issue 1: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, assignee_type, assignee_id, position)
		VALUES ($1, 'Claim Test Issue 2', 'in_progress', 'medium', 'member', $2, 'agent', $3, 0)
		RETURNING id
	`, testWorkspaceID, testUserID, agentID).Scan(&issueID2); err != nil {
		t.Fatalf("create issue 2: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)
		testPool.Exec(ctx, `DELETE FROM issue WHERE id IN ($1, $2)`, issueID1, issueID2)
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
