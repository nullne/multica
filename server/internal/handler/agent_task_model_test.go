package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func seedTaskModelTestRun(t *testing.T, status string) (issueID string, taskID string) {
	t.Helper()
	ctx := t.Context()

	var agentID string
	if err := testPool.QueryRow(ctx,
		`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "Handler Test Agent",
	).Scan(&agentID); err != nil {
		t.Fatalf("load test agent: %v", err)
	}

	var runtimeID string
	if err := testPool.QueryRow(ctx,
		`SELECT id FROM agent_runtime WHERE workspace_id = $1 AND provider = 'codex' LIMIT 1`,
		testWorkspaceID,
	).Scan(&runtimeID); err != nil {
		t.Fatalf("load test runtime: %v", err)
	}

	number := int32(time.Now().UnixNano() % 1_000_000_000)
	if err := testPool.QueryRow(ctx,
		`INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position)
		 VALUES ($1, 'task model metadata test', 'todo', 'none', 'member', $2, $3, 0)
		 RETURNING id`,
		testWorkspaceID, testUserID, number,
	).Scan(&issueID); err != nil {
		t.Fatalf("create test issue: %v", err)
	}

	if err := testPool.QueryRow(ctx,
		`INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, provider, model, thinking_level)
		 VALUES ($1, $2, $3, $4, 0, 'codex', 'configured-model', 'medium')
		 RETURNING id`,
		agentID, runtimeID, issueID, status,
	).Scan(&taskID); err != nil {
		t.Fatalf("create test task: %v", err)
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
		_, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
	})

	return issueID, taskID
}

func TestReportTaskExecutionMetadataPersistsTaskModelFields(t *testing.T) {
	_, taskID := seedTaskModelTestRun(t, "running")

	req := withURLParam(newRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/metadata", map[string]any{
		"provider":       "codex",
		"model":          "gpt-5.5",
		"thinking_level": "high",
	}), "taskId", taskID)
	w := httptest.NewRecorder()

	testHandler.ReportTaskExecutionMetadata(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var provider, model, thinkingLevel string
	if err := testPool.QueryRow(t.Context(),
		`SELECT provider, model, thinking_level FROM agent_task_queue WHERE id = $1`,
		taskID,
	).Scan(&provider, &model, &thinkingLevel); err != nil {
		t.Fatalf("load task metadata: %v", err)
	}
	if provider != "codex" || model != "gpt-5.5" || thinkingLevel != "high" {
		t.Fatalf("unexpected metadata provider=%q model=%q thinking_level=%q", provider, model, thinkingLevel)
	}
}

func TestCompleteTaskPersistsActualModelAndTaskRunsReturnMetadata(t *testing.T) {
	issueID, taskID := seedTaskModelTestRun(t, "running")

	req := withURLParam(newRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output":         "done",
		"model":          "actual-model",
		"thinking_level": "high",
	}), "taskId", taskID)
	w := httptest.NewRecorder()

	testHandler.CompleteTask(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected complete 200, got %d: %s", w.Code, w.Body.String())
	}

	req = withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID+"/task-runs", nil), "id", issueID)
	w = httptest.NewRecorder()
	testHandler.ListTasksByIssue(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", w.Code, w.Body.String())
	}

	var runs []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &runs); err != nil {
		t.Fatalf("decode task runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 task run, got %d", len(runs))
	}
	if runs[0]["provider"] != "codex" || runs[0]["model"] != "actual-model" || runs[0]["thinking_level"] != "high" {
		t.Fatalf("unexpected task run metadata: %#v", runs[0])
	}
}
