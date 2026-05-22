package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nullne/multica/server/internal/middleware"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

func routineRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	req := newRequest(method, path, body)
	member, err := testHandler.Queries.GetMemberByUserAndWorkspace(req.Context(), db.GetMemberByUserAndWorkspaceParams{
		UserID:      parseUUID(testUserID),
		WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load member: %v", err)
	}
	return req.WithContext(middleware.SetMemberContext(req.Context(), testWorkspaceID, member))
}

func TestRoutineCRUD_CreateGetAndRunList(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine CRUD test",
		"instructions":  "Create a routine issue",
		"priority":      "high",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers": []map[string]any{
			{
				"trigger_type": "schedule",
				"schedule":     "0 9 * * 1",
				"timezone":     "UTC",
			},
		},
	})
	testHandler.CreateRoutine(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateRoutine: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created RoutineResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created routine: %v", err)
	}
	if created.Name != "Routine CRUD test" {
		t.Fatalf("name = %q", created.Name)
	}
	if len(created.Triggers) != 1 || created.Triggers[0].TriggerType != "schedule" {
		t.Fatalf("expected one schedule trigger, got %+v", created.Triggers)
	}
	if len(created.Actions) != 1 || created.Actions[0].ActionType != "create_issue" {
		t.Fatalf("expected default create_issue action, got %+v", created.Actions)
	}
	t.Cleanup(func() {
		_ = testHandler.Queries.DeleteRoutine(context.Background(), db.DeleteRoutineParams{
			ID:          parseUUID(created.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})

	w = httptest.NewRecorder()
	req = routineRequest(t, "GET", "/api/routines/"+created.ID, nil)
	req = withURLParam(req, "id", created.ID)
	testHandler.GetRoutine(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetRoutine: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = routineRequest(t, "GET", "/api/routines/"+created.ID+"/runs", nil)
	req = withURLParam(req, "id", created.ID)
	testHandler.ListRoutineRuns(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRoutineRuns: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRoutineAPITriggerIngestCreatesIssue(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine API ingest",
		"instructions":  "Created from API",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers": []map[string]any{
			{
				"trigger_type": "api",
				"source_type":  "standard",
			},
		},
	})
	testHandler.CreateRoutine(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateRoutine: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var routine RoutineResponse
	if err := json.NewDecoder(w.Body).Decode(&routine); err != nil {
		t.Fatalf("decode routine: %v", err)
	}
	t.Cleanup(func() {
		_ = testHandler.Queries.DeleteRoutine(context.Background(), db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})
	if len(routine.Triggers) != 1 || routine.Triggers[0].Token == "" {
		t.Fatalf("expected API trigger token, got %+v", routine.Triggers)
	}

	body := map[string]any{
		"title":     "Routine API issue",
		"body":      "Created by API trigger",
		"dedup_key": "routine-api-ingest-test",
	}
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/routine-triggers/"+routine.Triggers[0].ID, body)
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+routine.Triggers[0].Token)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("IngestRoutineTrigger: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id::text FROM issue
		WHERE workspace_id = $1 AND title = 'Routine API issue'
		ORDER BY created_at DESC LIMIT 1
	`, testWorkspaceID).Scan(&issueID); err != nil {
		t.Fatalf("query created issue: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID) })

	runs, err := testHandler.Queries.ListRoutineRuns(context.Background(), db.ListRoutineRunsParams{
		RoutineID: parseUUID(routine.ID),
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListRoutineRuns: %v", err)
	}
	if len(runs) == 0 || runs[0].Status != "processed" {
		t.Fatalf("expected processed routine run, got %+v", runs)
	}
}

func TestCreateRoutineValidatesRequiredFields(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":         "Missing required fields",
		"instructions": "",
		"triggers":     []map[string]any{},
	})
	testHandler.CreateRoutine(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateRoutine: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
