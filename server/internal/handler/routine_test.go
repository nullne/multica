package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

func routineAPITokenDraft(t *testing.T) (map[string]any, string) {
	t.Helper()
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routine-trigger-token-drafts", nil)
	testHandler.GenerateRoutineTriggerTokenDraft(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("GenerateRoutineTriggerTokenDraft: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var draft RoutineTriggerTokenDraftResponse
	if err := json.NewDecoder(w.Body).Decode(&draft); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	return map[string]any{
		"id":             draft.DraftID,
		"trigger_type":   "api",
		"source_type":    "standard",
		"token_draft_id": draft.DraftID,
	}, draft.Token
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

func TestDeleteRoutineArchivesAndPreservesRuns(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine archive test",
		"instructions":  "Archive instead of delete",
		"priority":      "medium",
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
	var routine RoutineResponse
	if err := json.NewDecoder(w.Body).Decode(&routine); err != nil {
		t.Fatalf("decode routine: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM routine WHERE id = $1`, routine.ID)
	})

	run, err := testHandler.Queries.CreateRoutineRun(ctx, db.CreateRoutineRunParams{
		RoutineID: parseUUID(routine.ID),
		TriggerID: parseUUID(routine.Triggers[0].ID),
		ActionID:  parseUUID(routine.Actions[0].ID),
		EventType: "routine.archive.test",
		DedupKey:  "routine-archive-test",
		Payload:   []byte(`{}`),
		Status:    "processed",
	})
	if err != nil {
		t.Fatalf("CreateRoutineRun: %v", err)
	}

	w = httptest.NewRecorder()
	req = routineRequest(t, "DELETE", "/api/routines/"+routine.ID, nil)
	req = withURLParam(req, "id", routine.ID)
	testHandler.DeleteRoutine(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteRoutine: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	var archived bool
	if err := testPool.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM routine WHERE id = $1`, routine.ID).Scan(&archived); err != nil {
		t.Fatalf("query archived routine: %v", err)
	}
	if !archived {
		t.Fatal("expected routine to be archived")
	}

	routines, err := testHandler.Queries.ListRoutines(ctx, parseUUID(testWorkspaceID))
	if err != nil {
		t.Fatalf("ListRoutines: %v", err)
	}
	for _, listed := range routines {
		if uuidToString(listed.ID) == routine.ID {
			t.Fatalf("archived routine %s should not appear in ListRoutines", routine.ID)
		}
	}

	var runExists bool
	if err := testPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM routine_run WHERE id = $1)`, uuidToString(run.ID)).Scan(&runExists); err != nil {
		t.Fatalf("query routine run: %v", err)
	}
	if !runExists {
		t.Fatal("expected routine run history to be preserved")
	}
}

func TestListRoutineEventsPaginatesWorkspaceEventLog(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	created := make([]db.RoutineEvent, 0, 3)
	for i, eventType := range []string{"routine.test.first", "routine.test.second", "routine.test.third"} {
		event, err := testHandler.Queries.CreateRoutineEvent(ctx, db.CreateRoutineEventParams{
			WorkspaceID:        parseUUID(testWorkspaceID),
			SourceType:         "api",
			EventType:          eventType,
			DedupKey:           eventType,
			ExternalDeliveryID: pgtype.Text{String: "delivery-" + eventType, Valid: true},
			Data:               []byte(`{"headers":{"x-test":"true"}}`),
			Payload:            []byte(`{"title":"Routine event page"}`),
			Status:             "processed",
		})
		if err != nil {
			t.Fatalf("create routine event %d: %v", i, err)
		}
		created = append(created, event)
	}

	w := httptest.NewRecorder()
	req := routineRequest(t, "GET", "/api/routine-events?limit=2&offset=1", nil)
	testHandler.ListRoutineEvents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRoutineEvents: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var events []RoutineEventResponse
	if err := json.NewDecoder(w.Body).Decode(&events); err != nil {
		t.Fatalf("decode routine events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %+v", len(events), events)
	}
	if events[0].ID != uuidToString(created[1].ID) || events[1].ID != uuidToString(created[0].ID) {
		t.Fatalf("unexpected paginated order: got %q, %q", events[0].EventType, events[1].EventType)
	}
	if events[0].Payload.(map[string]any)["title"] != "Routine event page" {
		t.Fatalf("payload not decoded: %+v", events[0].Payload)
	}
}

func TestListRoutineEventsFiltersByStatusSourceAndEventType(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	matching, err := testHandler.Queries.CreateRoutineEvent(ctx, db.CreateRoutineEventParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		SourceType:  "github",
		EventType:   "github.pull_request.opened",
		DedupKey:    "routine-event-filter-match",
		Data:        []byte(`{}`),
		Payload:     []byte(`{"title":"Matched event"}`),
		Status:      "error",
	})
	if err != nil {
		t.Fatalf("create matching routine event: %v", err)
	}
	for _, fixture := range []struct {
		sourceType string
		eventType  string
		status     string
		dedupKey   string
	}{
		{"github", "github.pull_request.closed", "error", "routine-event-filter-wrong-event"},
		{"api", "github.pull_request.opened", "error", "routine-event-filter-wrong-source"},
		{"github", "github.pull_request.opened", "processed", "routine-event-filter-wrong-status"},
	} {
		_, err := testHandler.Queries.CreateRoutineEvent(ctx, db.CreateRoutineEventParams{
			WorkspaceID: parseUUID(testWorkspaceID),
			SourceType:  fixture.sourceType,
			EventType:   fixture.eventType,
			DedupKey:    fixture.dedupKey,
			Data:        []byte(`{}`),
			Payload:     []byte(`{}`),
			Status:      fixture.status,
		})
		if err != nil {
			t.Fatalf("create routine event fixture %s: %v", fixture.dedupKey, err)
		}
	}

	w := httptest.NewRecorder()
	req := routineRequest(t, "GET", "/api/routine-events?status=error&source_type=github&event_type=github.pull_request.opened", nil)
	testHandler.ListRoutineEvents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListRoutineEvents: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var events []RoutineEventResponse
	if err := json.NewDecoder(w.Body).Decode(&events); err != nil {
		t.Fatalf("decode routine events: %v", err)
	}
	if len(events) != 1 || events[0].ID != uuidToString(matching.ID) {
		t.Fatalf("expected only matching event, got %+v", events)
	}
}

func TestRoutineAPITriggerRequiresGeneratedTokenDraft(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}

	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routine-trigger-token-drafts", nil)
	testHandler.GenerateRoutineTriggerTokenDraft(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("GenerateRoutineTriggerTokenDraft: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var draft RoutineTriggerTokenDraftResponse
	if err := json.NewDecoder(w.Body).Decode(&draft); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if draft.DraftID == "" || draft.Token == "" || draft.TokenPrefix == "" {
		t.Fatalf("expected generated draft token, got %+v", draft)
	}

	w = httptest.NewRecorder()
	req = routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine API draft",
		"instructions":  "Created from generated API token",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers": []map[string]any{
			{
				"id":             draft.DraftID,
				"trigger_type":   "api",
				"source_type":    "standard",
				"token_draft_id": draft.DraftID,
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
	if len(routine.Triggers) != 1 || routine.Triggers[0].ID != draft.DraftID || routine.Triggers[0].Token != "" {
		t.Fatalf("expected consumed API trigger without re-revealed token, got %+v", routine.Triggers)
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/webhook/"+draft.DraftID, map[string]any{
		"title":     "Routine API draft issue",
		"dedup_key": "routine-api-draft",
	})
	req = withURLParam(req, "id", draft.DraftID)
	req.Header.Set("Authorization", "Bearer "+draft.Token)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("IngestRoutineTrigger: expected 202, got %d: %s", w.Code, w.Body.String())
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
	apiTrigger, apiToken := routineAPITokenDraft(t)
	apiTrigger["dedup_window_seconds"] = 600
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine API ingest",
		"instructions":  "Created from API",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers":      []map[string]any{apiTrigger},
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
	if len(routine.Triggers) != 1 {
		t.Fatalf("expected API trigger, got %+v", routine.Triggers)
	}

	body := map[string]any{
		"title":     "Routine API issue",
		"body":      "Created by API trigger",
		"dedup_key": "routine-api-ingest-test",
	}
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, body)
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+apiToken)
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

func TestRoutineAPITriggerStoresOneRoutineEventForMultipleRuns(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	apiTrigger, apiToken := routineAPITokenDraft(t)
	suffix := time.Now().Format("150405000000000")
	firstTitle := "Routine event log first " + suffix
	secondTitle := "Routine event log second " + suffix

	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine event log fanout",
		"instructions":  "Create multiple issues from one event",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers":      []map[string]any{apiTrigger},
		"actions": []map[string]any{
			{
				"action_type": "create_issue",
				"config": map[string]any{
					"title_template": firstTitle,
				},
			},
			{
				"action_type": "create_issue",
				"config": map[string]any{
					"title_template": secondTitle,
				},
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
		_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
		_, _ = testPool.Exec(ctx, `
			DELETE FROM issue
			WHERE workspace_id = $1 AND title = ANY($2::text[])
		`, testWorkspaceID, []string{firstTitle, secondTitle})
	})

	dedupKey := "routine-event-log-fanout-" + suffix
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, map[string]any{
		"title":     "Fanout source event",
		"body":      "One event should back both runs",
		"dedup_key": dedupKey,
	})
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("IngestRoutineTrigger: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var eventID string
	var eventStatus string
	var eventData []byte
	if err := testPool.QueryRow(ctx, `
		SELECT id::text, status, data
		FROM routine_event
		WHERE workspace_id = $1 AND dedup_key = $2
	`, testWorkspaceID, dedupKey).Scan(&eventID, &eventStatus, &eventData); err != nil {
		t.Fatalf("query routine_event: %v", err)
	}
	if eventStatus != "processed" {
		t.Fatalf("routine_event status = %q, want processed", eventStatus)
	}
	var data map[string]string
	if err := json.Unmarshal(eventData, &data); err != nil {
		t.Fatalf("unmarshal routine_event data: %v", err)
	}
	if data["title"] != "Fanout source event" {
		t.Fatalf("routine_event data title = %q", data["title"])
	}

	var runCount int
	var linkedEventCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*), count(DISTINCT routine_event_id)
		FROM routine_run
		WHERE routine_id = $1 AND routine_event_id = $2
	`, routine.ID, eventID).Scan(&runCount, &linkedEventCount); err != nil {
		t.Fatalf("count routine runs linked to event: %v", err)
	}
	if runCount != 2 || linkedEventCount != 1 {
		t.Fatalf("runs linked to event count = %d distinct events = %d, want 2/1", runCount, linkedEventCount)
	}
}

func TestRoutineAPITriggerAcceptsFlexibleJSONPayload(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	apiTrigger, apiToken := routineAPITokenDraft(t)
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine flexible payload",
		"instructions":  "Investigate raw payload",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers":      []map[string]any{apiTrigger},
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
		_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})

	payload := []byte(`{"deployment":{"service":"api","status":"failed"},"metadata":{"source":"curl"}}`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, bytes.NewReader(payload))
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("IngestRoutineTrigger: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var issueID, title, description string
	if err := testPool.QueryRow(ctx, `
		SELECT id::text, title, COALESCE(description, '')
		FROM issue
		WHERE workspace_id = $1 AND description = 'Investigate raw payload'
		ORDER BY created_at DESC LIMIT 1
	`, testWorkspaceID).Scan(&issueID, &title, &description); err != nil {
		t.Fatalf("query created issue: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID) })
	if title != "" {
		t.Fatalf("issue title = %q, want empty", title)
	}
	if description != "Investigate raw payload" {
		t.Fatalf("issue description = %q", description)
	}

	runs, err := testHandler.Queries.ListRoutineRuns(ctx, db.ListRoutineRunsParams{
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
	var storedPayload map[string]any
	if err := json.Unmarshal(runs[0].Payload, &storedPayload); err != nil {
		t.Fatalf("unmarshal run payload: %v", err)
	}
	deployment, ok := storedPayload["deployment"].(map[string]any)
	if !ok || deployment["service"] != "api" {
		t.Fatalf("stored payload missing deployment.service: %#v", storedPayload)
	}

	var taskContext []byte
	if err := testPool.QueryRow(ctx, `
		SELECT context FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, issueID).Scan(&taskContext); err != nil {
		t.Fatalf("query task context: %v", err)
	}
	var contextData map[string]any
	if err := json.Unmarshal(taskContext, &contextData); err != nil {
		t.Fatalf("unmarshal task context: %v", err)
	}
	routineEvent, ok := contextData["routine_event"].(map[string]any)
	if !ok {
		t.Fatalf("routine_event missing from task context: %#v", contextData)
	}
	if routineEvent["raw_payload"] != string(payload) {
		t.Fatalf("raw_payload = %q, want %q", routineEvent["raw_payload"], string(payload))
	}

	arrayPayload := []byte(`[{"status":"failed"},{"status":"retrying"}]`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, bytes.NewReader(arrayPayload))
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("array payload: expected 202, got %d: %s", w.Code, w.Body.String())
	}
	runs, err = testHandler.Queries.ListRoutineRuns(ctx, db.ListRoutineRunsParams{
		RoutineID: parseUUID(routine.ID),
		Limit:     1,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListRoutineRuns after array payload: %v", err)
	}
	var storedArray []any
	if len(runs) == 0 || json.Unmarshal(runs[0].Payload, &storedArray) != nil || len(storedArray) != 2 {
		t.Fatalf("expected array payload in latest run, got %+v", runs)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, bytes.NewBufferString(`{"event":`))
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, strings.NewReader(`"`+strings.Repeat("x", 1<<20)+`"`))
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized JSON: expected 413, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRoutineIssueTemplateFieldsAreApplied(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	var subscriberID string
	if err := testPool.QueryRow(ctx, `SELECT user_id::text FROM member WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&subscriberID); err != nil {
		t.Fatalf("find subscriber: %v", err)
	}
	label, err := testHandler.Queries.CreateLabel(ctx, db.CreateLabelParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Name:        "routine-template-test",
		Color:       "blue",
	})
	if err != nil {
		t.Fatalf("create label: %v", err)
	}
	t.Cleanup(func() {
		_ = testHandler.Queries.DeleteLabel(ctx, db.DeleteLabelParams{ID: label.ID, WorkspaceID: parseUUID(testWorkspaceID)})
	})

	dueOffset := int32(24)
	apiTrigger, apiToken := routineAPITokenDraft(t)
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":                  "Routine template fields",
		"instructions":          "Fallback body",
		"priority":              "low",
		"assignee_type":         "agent",
		"assignee_id":           agentID,
		"due_date_offset_hours": dueOffset,
		"dispatch_provider":     "codex",
		"dispatch_daemon_label": "local-dev",
		"subscriber_ids":        []string{subscriberID},
		"label_ids":             []string{uuidToString(label.ID)},
		"triggers":              []map[string]any{apiTrigger},
		"actions": []map[string]any{
			{
				"action_type": "create_issue",
				"config": map[string]any{
					"title_template":       "Deploy {{.fields.service}} failed",
					"description_template": "See {{.source_url}}",
				},
				"enabled":  true,
				"position": 0,
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
		_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})

	sourceURL := "https://alerts.example.com/incidents/42"
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, map[string]any{
		"title":     "ignored by template",
		"body":      "payload body",
		"dedup_key": "routine-template-fields",
		"fields": map[string]string{
			"service":     "api",
			"source_url":  sourceURL,
			"source_kind": "alert",
			"external_id": "incident-42",
		},
	})
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("IngestRoutineTrigger: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var issue struct {
		ID                  string
		Title               string
		Description         string
		Priority            string
		AssigneeType        string
		AssigneeID          string
		DispatchProvider    string
		DispatchDaemonLabel string
		DueDate             time.Time
	}
	if err := testPool.QueryRow(ctx, `
		SELECT id::text, title, description, priority, assignee_type, assignee_id::text,
		       dispatch_provider, dispatch_daemon_label, due_date
		FROM issue
		WHERE workspace_id = $1 AND title = 'Deploy api failed'
		ORDER BY created_at DESC LIMIT 1
	`, testWorkspaceID).Scan(
		&issue.ID,
		&issue.Title,
		&issue.Description,
		&issue.Priority,
		&issue.AssigneeType,
		&issue.AssigneeID,
		&issue.DispatchProvider,
		&issue.DispatchDaemonLabel,
		&issue.DueDate,
	); err != nil {
		t.Fatalf("query created issue: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issue.ID) })
	if issue.Description != "See "+sourceURL {
		t.Fatalf("description = %q", issue.Description)
	}
	if issue.Priority != "low" || issue.AssigneeType != "agent" || issue.AssigneeID != agentID {
		t.Fatalf("unexpected issue assignment/priority: %+v", issue)
	}
	if issue.DispatchProvider != "codex" || issue.DispatchDaemonLabel != "local-dev" {
		t.Fatalf("unexpected dispatch config: %+v", issue)
	}
	if time.Until(issue.DueDate) < 23*time.Hour || time.Until(issue.DueDate) > 25*time.Hour {
		t.Fatalf("due date %s is not about 24h from now", issue.DueDate)
	}
	if _, err := testHandler.Queries.GetIssueLinkByURL(ctx, db.GetIssueLinkByURLParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Url:         sourceURL,
	}); err != nil {
		t.Fatalf("expected source issue link for API event: %v", err)
	}

	var labelCount, subscriberCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM issue_to_label WHERE issue_id = $1 AND label_id = $2`, issue.ID, uuidToString(label.ID)).Scan(&labelCount); err != nil {
		t.Fatalf("query issue label: %v", err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM issue_subscriber WHERE issue_id = $1 AND user_id = $2`, issue.ID, subscriberID).Scan(&subscriberCount); err != nil {
		t.Fatalf("query issue subscriber: %v", err)
	}
	if labelCount != 1 || subscriberCount != 1 {
		t.Fatalf("labelCount=%d subscriberCount=%d, want 1/1", labelCount, subscriberCount)
	}
}

func TestRoutineAPITriggerCreatesSeparateIssuesPerRoutineForSameSourceURL(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}

	createRoutine := func(name, title string) (RoutineResponse, string) {
		t.Helper()
		apiTrigger, apiToken := routineAPITokenDraft(t)
		w := httptest.NewRecorder()
		req := routineRequest(t, "POST", "/api/routines", map[string]any{
			"name":          name,
			"instructions":  "Create routine-scoped issue",
			"assignee_type": "agent",
			"assignee_id":   agentID,
			"triggers":      []map[string]any{apiTrigger},
			"actions": []map[string]any{
				{
					"action_type": "create_issue",
					"config": map[string]any{
						"title_template": title,
					},
				},
			},
		})
		testHandler.CreateRoutine(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("CreateRoutine %s: expected 201, got %d: %s", name, w.Code, w.Body.String())
		}
		var routine RoutineResponse
		if err := json.NewDecoder(w.Body).Decode(&routine); err != nil {
			t.Fatalf("decode routine: %v", err)
		}
		t.Cleanup(func() {
			_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
				ID:          parseUUID(routine.ID),
				WorkspaceID: parseUUID(testWorkspaceID),
			})
		})
		return routine, apiToken
	}

	suffix := time.Now().Format("150405000000000")
	stagingTitle := "Routine scoped links staging " + suffix
	prodTitle := "Routine scoped links prod " + suffix
	stagingRoutine, stagingToken := createRoutine("Routine scoped staging", stagingTitle)
	prodRoutine, prodToken := createRoutine("Routine scoped prod", prodTitle)
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `
			DELETE FROM issue
			WHERE workspace_id = $1
			  AND title = ANY($2::text[])
		`, testWorkspaceID, []string{stagingTitle, prodTitle})
	})

	sourceURL := "https://github.com/nullne/multica/pull/routine-scoped-" + suffix
	ingest := func(routine RoutineResponse, token, dedupKey string) {
		t.Helper()
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, map[string]any{
			"title":     "ignored by template",
			"dedup_key": dedupKey,
			"fields": map[string]string{
				"source_url":  sourceURL,
				"source_kind": "pr",
			},
		})
		req = withURLParam(req, "id", routine.Triggers[0].ID)
		req.Header.Set("Authorization", "Bearer "+token)
		testHandler.IngestRoutineTrigger(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("IngestRoutineTrigger: expected 202, got %d: %s", w.Code, w.Body.String())
		}
	}

	ingest(stagingRoutine, stagingToken, "routine-scoped-staging")
	if _, err := testHandler.Queries.GetIssueLinkByURL(ctx, db.GetIssueLinkByURLParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Url:         sourceURL,
	}); err != nil {
		t.Fatalf("expected first routine to create source link: %v", err)
	}
	var stagingIssueID string
	if err := testPool.QueryRow(ctx, `
		SELECT id::text
		FROM issue
		WHERE workspace_id = $1 AND title = $2
	`, testWorkspaceID, stagingTitle).Scan(&stagingIssueID); err != nil {
		t.Fatalf("query staging issue: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO routine_run (routine_id, trigger_id, action_id, event_type, dedup_key, payload, status, issue_id)
		VALUES ($1, $2, $3, 'custom', 'legacy-collapsed-prod-run', '{}'::jsonb, 'processed', $4)
	`, prodRoutine.ID, prodRoutine.Triggers[0].ID, prodRoutine.Actions[0].ID, stagingIssueID); err != nil {
		t.Fatalf("insert legacy collapsed prod run: %v", err)
	}
	ingest(prodRoutine, prodToken, "routine-scoped-prod")

	assertIssueCount(t, stagingTitle, 1)
	assertIssueCount(t, prodTitle, 1)

	var linkCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*)
		FROM issue_link
		WHERE workspace_id = $1 AND url = $2
	`, testWorkspaceID, sourceURL).Scan(&linkCount); err != nil {
		t.Fatalf("count issue links: %v", err)
	}
	if linkCount != 2 {
		t.Fatalf("issue link count = %d, want 2", linkCount)
	}
}

func TestRoutineAPITriggerCanAssignIssueToMember(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	apiTrigger, apiToken := routineAPITokenDraft(t)
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine member assignee",
		"instructions":  "Created for a member",
		"priority":      "high",
		"assignee_type": "member",
		"assignee_id":   testUserID,
		"enabled":       true,
		"triggers":      []map[string]any{apiTrigger},
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
		_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, map[string]any{
		"title":     "Routine member assigned issue",
		"dedup_key": "routine-member-assignee",
	})
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("IngestRoutineTrigger: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var issue struct {
		ID           string
		AssigneeType string
		AssigneeID   string
		Priority     string
	}
	if err := testPool.QueryRow(ctx, `
		SELECT id::text, assignee_type, assignee_id::text, priority
		FROM issue
		WHERE workspace_id = $1 AND title = 'Routine member assigned issue'
		ORDER BY created_at DESC LIMIT 1
	`, testWorkspaceID).Scan(&issue.ID, &issue.AssigneeType, &issue.AssigneeID, &issue.Priority); err != nil {
		t.Fatalf("query created issue: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issue.ID) })
	if issue.AssigneeType != "member" || issue.AssigneeID != testUserID || issue.Priority != "high" {
		t.Fatalf("unexpected member assignment/priority: %+v", issue)
	}
}

func TestRoutineDisabledAPITriggersDoNotCreateIssuesUntilEnabled(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	apiTrigger, apiToken := routineAPITokenDraft(t)
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine disabled API",
		"instructions":  "Disabled routines should not run",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       false,
		"triggers":      []map[string]any{apiTrigger},
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
		_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})
	trigger := routine.Triggers[0]

	ingest := func(title, dedupKey string) {
		t.Helper()
		w = httptest.NewRecorder()
		req = newRequest("POST", "/api/webhook/"+trigger.ID, map[string]any{
			"title":     title,
			"dedup_key": dedupKey,
		})
		req = withURLParam(req, "id", trigger.ID)
		req.Header.Set("Authorization", "Bearer "+apiToken)
		testHandler.IngestRoutineTrigger(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("IngestRoutineTrigger: expected 202, got %d: %s", w.Code, w.Body.String())
		}
	}

	ingest("Disabled routine should not create", "routine-disabled")
	assertIssueCount(t, "Disabled routine should not create", 0)
	assertRoutineRunCount(t, routine.ID, 0)

	if _, err := testPool.Exec(ctx, `UPDATE routine SET enabled = TRUE WHERE id = $1`, routine.ID); err != nil {
		t.Fatalf("enable routine: %v", err)
	}
	ingest("Enabled routine should create", "routine-enabled")
	assertIssueCount(t, "Enabled routine should create", 1)
	assertRoutineRunStatus(t, routine.ID, "processed")

	if _, err := testPool.Exec(ctx, `DELETE FROM issue WHERE workspace_id = $1 AND title IN ('Disabled routine should not create', 'Enabled routine should create')`, testWorkspaceID); err != nil {
		t.Fatalf("cleanup issues: %v", err)
	}

	if _, err := testPool.Exec(ctx, `UPDATE routine_trigger SET enabled = FALSE WHERE id = $1`, trigger.ID); err != nil {
		t.Fatalf("disable trigger: %v", err)
	}
	ingest("Disabled trigger should not create", "routine-disabled-trigger")
	assertIssueCount(t, "Disabled trigger should not create", 0)
}

func TestRoutineRunHistoryRecordsActionErrors(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	var linkedIssueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number)
		VALUES ($1, 'Routine linked issue for error', 'todo', 'medium', 'member', $2, 9981)
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&linkedIssueID); err != nil {
		t.Fatalf("insert linked issue: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, linkedIssueID) })
	sourceURL := "https://github.com/acme/widgets/pull/routine-error"
	if _, err := testPool.Exec(ctx, `
		INSERT INTO issue_link (issue_id, workspace_id, source_type, kind, direction, url, external_id)
		VALUES ($1, $2, 'github', 'pr', 'source', $3, 'routine-error')
	`, linkedIssueID, testWorkspaceID, sourceURL); err != nil {
		t.Fatalf("insert issue link: %v", err)
	}

	apiTrigger, apiToken := routineAPITokenDraft(t)
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine action error",
		"instructions":  "This action is intentionally misconfigured",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers":      []map[string]any{apiTrigger},
		"actions": []map[string]any{
			{
				"action_type": "comment_issue",
				"config":      map[string]any{},
				"enabled":     true,
				"position":    0,
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
		_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/webhook/"+routine.Triggers[0].ID, map[string]any{
		"title":     "Routine action error event",
		"dedup_key": "routine-action-error",
		"fields": map[string]string{
			"source_url": sourceURL,
		},
	})
	req = withURLParam(req, "id", routine.Triggers[0].ID)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("IngestRoutineTrigger: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	runs, err := testHandler.Queries.ListRoutineRuns(ctx, db.ListRoutineRunsParams{
		RoutineID: parseUUID(routine.ID),
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListRoutineRuns: %v", err)
	}
	for _, run := range runs {
		if run.Status == "error" && run.ErrorMessage.Valid && run.ErrorMessage.String == "comment_issue requires bot_user_id" {
			return
		}
	}
	t.Fatalf("expected readable error run, got %+v", runs)
}

func TestRoutineAPITriggerAuthRegenerateFilterAndDedup(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	apiTrigger, oldToken := routineAPITokenDraft(t)
	apiTrigger["dedup_window_seconds"] = 600
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine API auth filter dedup",
		"instructions":  "Created from API",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers":      []map[string]any{apiTrigger},
		"actions": []map[string]any{
			{
				"action_type": "create_issue",
				"config": map[string]any{
					"event_types": []string{"deploy.passed"},
				},
				"enabled":  true,
				"position": 0,
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
	trigger := routine.Triggers[0]

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/webhook/"+trigger.ID, map[string]any{"title": "No token"})
	req = withURLParam(req, "id", trigger.ID)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: expected 401, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/webhook/"+trigger.ID, map[string]any{"title": "Wrong token"})
	req = withURLParam(req, "id", trigger.ID)
	req.Header.Set("Authorization", "Bearer wrong")
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: expected 401, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = routineRequest(t, "POST", "/api/routines/"+routine.ID+"/triggers/"+trigger.ID+"/regenerate-token", nil)
	req = withURLParam(req, "id", routine.ID)
	req = withURLParam(req, "triggerId", trigger.ID)
	testHandler.RegenerateRoutineTriggerToken(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RegenerateRoutineTriggerToken: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var regenerated RoutineTriggerTokenResponse
	if err := json.NewDecoder(w.Body).Decode(&regenerated); err != nil {
		t.Fatalf("decode regenerated token: %v", err)
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/webhook/"+trigger.ID, map[string]any{"title": "Old token"})
	req = withURLParam(req, "id", trigger.ID)
	req.Header.Set("Authorization", "Bearer "+oldToken)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old token: expected 401 after regenerate, got %d", w.Code)
	}

	ingest := func(eventType, dedupKey, title string) {
		t.Helper()
		w = httptest.NewRecorder()
		req = newRequest("POST", "/api/webhook/"+trigger.ID, map[string]any{
			"title":     title,
			"type":      eventType,
			"dedup_key": dedupKey,
		})
		req = withURLParam(req, "id", trigger.ID)
		req.Header.Set("Authorization", "Bearer "+regenerated.Token)
		testHandler.IngestRoutineTrigger(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("ingest %s: expected 202, got %d: %s", eventType, w.Code, w.Body.String())
		}
	}
	ingest("deploy.failed", "routine-filtered", "Filtered event")
	ingest("deploy.passed", "routine-dedup", "Processed event")
	ingest("deploy.passed", "routine-dedup", "Duplicate event")

	runs, err := testHandler.Queries.ListRoutineRuns(context.Background(), db.ListRoutineRunsParams{
		RoutineID: parseUUID(routine.ID),
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListRoutineRuns: %v", err)
	}
	seen := map[string]bool{}
	for _, run := range runs {
		seen[run.Status] = true
	}
	for _, status := range []string{"filtered", "processed", "deduped"} {
		if !seen[status] {
			t.Fatalf("expected %s run in %+v", status, runs)
		}
	}
}

func TestRegenerateRoutineTriggerToken(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	apiTrigger, _ := routineAPITokenDraft(t)
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine API regenerate",
		"instructions":  "Created from API",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers":      []map[string]any{apiTrigger},
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
	oldPrefix := routine.Triggers[0].TokenPrefix

	w = httptest.NewRecorder()
	req = routineRequest(t, "POST", "/api/routines/"+routine.ID+"/triggers/"+routine.Triggers[0].ID+"/regenerate-token", nil)
	req = withURLParam(req, "id", routine.ID)
	req = withURLParam(req, "triggerId", routine.Triggers[0].ID)
	testHandler.RegenerateRoutineTriggerToken(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RegenerateRoutineTriggerToken: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Trigger RoutineTriggerResponse `json:"trigger"`
		Token   string                 `json:"token"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode regenerate response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected regenerated token")
	}
	if resp.Trigger.TokenPrefix == "" || resp.Trigger.TokenPrefix == oldPrefix {
		t.Fatalf("expected new token prefix, old=%q new=%q", oldPrefix, resp.Trigger.TokenPrefix)
	}
}

func TestRoutineGitHubTriggerEvents(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	installationID := time.Now().UnixNano()
	if _, err := testPool.Exec(ctx, `UPDATE workspace SET github_installation_id = $1 WHERE id = $2`, installationID, testWorkspaceID); err != nil {
		t.Fatalf("set github installation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `UPDATE workspace SET github_installation_id = NULL WHERE id = $1`, testWorkspaceID)
	})

	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine GitHub events",
		"instructions":  "Created from GitHub",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers": []map[string]any{
			{
				"trigger_type": "github",
			},
		},
		"actions": []map[string]any{
			{
				"action_type": "create_issue",
				"config": map[string]any{
					"event_types": []string{
						"github.pull_request.opened",
						"github.pull_request.merged",
						"github.issues.opened",
						"github.release.published",
					},
					"title_template": "GitHub {{.action}}: {{.title}}",
				},
				"enabled":  true,
				"position": 0,
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
		_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})
	if len(routine.Triggers) != 1 || routine.Triggers[0].InstallationID == nil || *routine.Triggers[0].InstallationID != installationID {
		t.Fatalf("expected github trigger installation %d, got %+v", installationID, routine.Triggers)
	}

	trigger, err := testHandler.Queries.GetRoutineTrigger(ctx, parseUUID(routine.Triggers[0].ID))
	if err != nil {
		t.Fatalf("GetRoutineTrigger: %v", err)
	}
	headers := http.Header{}
	cases := []struct {
		name        string
		githubEvent string
		body        []byte
		wantTitle   string
	}{
		{
			name:        "pull_request.opened",
			githubEvent: "pull_request",
			body:        []byte(`{"action":"opened","pull_request":{"number":1,"title":"Open PR","html_url":"https://github.com/acme/widgets/pull/801","user":{"login":"alice"},"head":{"ref":"feat"},"base":{"ref":"main"},"body":"body"},"repository":{"full_name":"acme/widgets"}}`),
			wantTitle:   "GitHub opened: Open PR",
		},
		{
			name:        "pull_request.merged",
			githubEvent: "pull_request",
			body:        []byte(`{"action":"closed","pull_request":{"number":2,"title":"Merge PR","merged":true,"html_url":"https://github.com/acme/widgets/pull/802","user":{"login":"bob"},"head":{"ref":"feat"},"base":{"ref":"main"},"body":"body"},"repository":{"full_name":"acme/widgets"}}`),
			wantTitle:   "GitHub merged: Merge PR",
		},
		{
			name:        "issues.opened",
			githubEvent: "issues",
			body:        []byte(`{"action":"opened","issue":{"number":3,"title":"Open issue","html_url":"https://github.com/acme/widgets/issues/803","user":{"login":"carol"},"body":"body"},"repository":{"full_name":"acme/widgets"}}`),
			wantTitle:   "GitHub opened: Open issue",
		},
		{
			name:        "release.published",
			githubEvent: "release",
			body:        []byte(`{"action":"published","release":{"name":"v1.0.0","tag_name":"v1.0.0","html_url":"https://github.com/acme/widgets/releases/tag/v1.0.0","author":{"login":"dana"},"body":"notes"},"repository":{"full_name":"acme/widgets"}}`),
			wantTitle:   "GitHub published: v1.0.0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers.Set("X-GitHub-Event", tc.githubEvent)
			received, ran := testHandler.ingestRoutineTriggers(ctx, []db.RoutineTrigger{trigger}, "github", tc.body, headers)
			if received != 1 || ran != 1 {
				t.Fatalf("received=%d ran=%d, want 1/1", received, ran)
			}
			var issueID string
			if err := testPool.QueryRow(ctx, `
				SELECT id::text FROM issue
				WHERE workspace_id = $1 AND title = $2
				ORDER BY created_at DESC LIMIT 1
			`, testWorkspaceID, tc.wantTitle).Scan(&issueID); err != nil {
				t.Fatalf("query issue %q: %v", tc.wantTitle, err)
			}
			t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID) })
		})
	}
}

func TestRoutineGitHubTriggerConfigFiltersEventsBeforeActions(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	installationID := time.Now().UnixNano()
	if _, err := testPool.Exec(ctx, `UPDATE workspace SET github_installation_id = $1 WHERE id = $2`, installationID, testWorkspaceID); err != nil {
		t.Fatalf("set github installation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `UPDATE workspace SET github_installation_id = NULL WHERE id = $1`, testWorkspaceID)
	})

	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine GitHub trigger config",
		"instructions":  "Created from matching GitHub trigger",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers": []map[string]any{
			{
				"trigger_type": "github",
				"config": map[string]any{
					"event_types": []string{"github.pull_request.closed"},
					"filters": []map[string]any{
						{
							"field":    "is_merged",
							"operator": "equals",
							"value":    "true",
						},
					},
				},
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
		_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})

	trigger, err := testHandler.Queries.GetRoutineTrigger(ctx, parseUUID(routine.Triggers[0].ID))
	if err != nil {
		t.Fatalf("GetRoutineTrigger: %v", err)
	}
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "pull_request")

	openedBody := []byte(`{"action":"opened","pull_request":{"number":1,"title":"Open target","merged":false,"html_url":"https://github.com/acme/widgets/pull/901","user":{"login":"alice"},"head":{"ref":"feat"},"base":{"ref":"main"},"body":"body"},"repository":{"full_name":"acme/widgets"}}`)
	received, ran := testHandler.ingestRoutineTriggers(ctx, []db.RoutineTrigger{trigger}, "github", openedBody, headers)
	if received != 1 || ran != 0 {
		t.Fatalf("opened PR received=%d ran=%d, want 1/0", received, ran)
	}
	runs, err := testHandler.Queries.ListRoutineRuns(ctx, db.ListRoutineRunsParams{
		RoutineID: parseUUID(routine.ID),
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListRoutineRuns after unmatched trigger event: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("unmatched trigger event should not create routine runs, got %+v", runs)
	}
	var eventStatus string
	if err := testPool.QueryRow(ctx, `
		SELECT status
		FROM routine_event
		WHERE workspace_id = $1
		  AND source_type = 'github'
		  AND event_type = 'github.pull_request.opened'
		  AND dedup_key = 'github:pull_request:opened:https://github.com/acme/widgets/pull/901'
	`, testWorkspaceID).Scan(&eventStatus); err != nil {
		t.Fatalf("query routine_event for unmatched trigger event: %v", err)
	}
	if eventStatus != "no_matching_trigger" {
		t.Fatalf("unmatched trigger event status = %q, want no_matching_trigger", eventStatus)
	}

	mergedBody := []byte(`{"action":"closed","pull_request":{"number":2,"title":"Merge target","merged":true,"html_url":"https://github.com/acme/widgets/pull/902","user":{"login":"bob"},"head":{"ref":"feat"},"base":{"ref":"main"},"body":"body"},"repository":{"full_name":"acme/widgets"}}`)
	received, ran = testHandler.ingestRoutineTriggers(ctx, []db.RoutineTrigger{trigger}, "github", mergedBody, headers)
	if received != 1 || ran != 1 {
		t.Fatalf("merged PR received=%d ran=%d, want 1/1", received, ran)
	}

	var issueID string
	if err := testPool.QueryRow(ctx, `
		SELECT id::text FROM issue
		WHERE workspace_id = $1 AND title = 'Merge target'
		ORDER BY created_at DESC LIMIT 1
	`, testWorkspaceID).Scan(&issueID); err != nil {
		t.Fatalf("query merged issue: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID) })
}

func TestManualTriggerRoutineCreatesIssue(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":                    "Routine manual trigger",
		"instructions":            "Created manually",
		"priority":                "medium",
		"assignee_type":           "agent",
		"assignee_id":             agentID,
		"enabled":                 true,
		"github_auto_fix_enabled": true,
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

	w = httptest.NewRecorder()
	req = routineRequest(t, "POST", "/api/routines/"+routine.ID+"/trigger", nil)
	req = withURLParam(req, "id", routine.ID)
	testHandler.TriggerRoutine(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("TriggerRoutine: expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Ran int `json:"ran"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode trigger response: %v", err)
	}
	if resp.Ran != 1 {
		t.Fatalf("expected one action to run, got %d", resp.Ran)
	}

	var issueID string
	var autoFixEnabled bool
	if err := testPool.QueryRow(context.Background(), `
		SELECT id::text, github_auto_fix_enabled FROM issue
		WHERE workspace_id = $1 AND title = 'Routine manual trigger'
		ORDER BY created_at DESC LIMIT 1
	`, testWorkspaceID).Scan(&issueID, &autoFixEnabled); err != nil {
		t.Fatalf("query created issue: %v", err)
	}
	if !autoFixEnabled {
		t.Fatalf("expected routine-created issue to inherit github_auto_fix_enabled")
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID) })
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

func TestRoutineWorkspaceIsolation(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	apiTrigger, _ := routineAPITokenDraft(t)
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine isolation",
		"instructions":  "Only workspace A",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"triggers":      []map[string]any{apiTrigger},
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
		_ = testHandler.Queries.DeleteRoutine(ctx, db.DeleteRoutineParams{
			ID:          parseUUID(routine.ID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})

	var otherUserID, otherWorkspaceID string
	if err := testPool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('Routine Other', $1) RETURNING id`, "routine-other-"+time.Now().Format("150405.000000000")+"@example.com").Scan(&otherUserID); err != nil {
		t.Fatalf("insert other user: %v", err)
	}
	if err := testPool.QueryRow(ctx, `INSERT INTO workspace (name, slug, issue_prefix) VALUES ('Routine Other', $1, 'ROT') RETURNING id`, "routine-other-"+time.Now().Format("150405000000000")).Scan(&otherWorkspaceID); err != nil {
		t.Fatalf("insert other workspace: %v", err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, otherWorkspaceID, otherUserID); err != nil {
		t.Fatalf("insert other member: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, otherWorkspaceID)
		_, _ = testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, otherUserID)
	})

	member, err := testHandler.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      parseUUID(otherUserID),
		WorkspaceID: parseUUID(otherWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load other member: %v", err)
	}
	otherReq := func(method, path string, body any) *http.Request {
		req := newRequest(method, path, body)
		req.Header.Set("X-User-ID", otherUserID)
		req.Header.Set("X-Workspace-ID", otherWorkspaceID)
		return req.WithContext(middleware.SetMemberContext(req.Context(), otherWorkspaceID, member))
	}

	w = httptest.NewRecorder()
	testHandler.ListRoutines(w, otherReq("GET", "/api/routines", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("ListRoutines other workspace: expected 200, got %d", w.Code)
	}
	var routines []RoutineResponse
	if err := json.NewDecoder(w.Body).Decode(&routines); err != nil {
		t.Fatalf("decode routines: %v", err)
	}
	for _, item := range routines {
		if item.ID == routine.ID {
			t.Fatalf("routine from workspace A leaked into workspace B list")
		}
	}

	w = httptest.NewRecorder()
	req = otherReq("GET", "/api/routines/"+routine.ID, nil)
	req = withURLParam(req, "id", routine.ID)
	testHandler.GetRoutine(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetRoutine other workspace: expected 404, got %d", w.Code)
	}

	if _, err := testHandler.Queries.GetIssueLinkByURL(ctx, db.GetIssueLinkByURLParams{
		WorkspaceID: parseUUID(otherWorkspaceID),
		Url:         "https://alerts.example.com/incidents/42",
	}); err != pgx.ErrNoRows {
		t.Fatalf("source link should not be visible in other workspace, err=%v", err)
	}
}

func assertIssueCount(t *testing.T, title string, want int) {
	t.Helper()
	var count int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM issue
		WHERE workspace_id = $1 AND title = $2
	`, testWorkspaceID, title).Scan(&count); err != nil {
		t.Fatalf("count issue %q: %v", title, err)
	}
	if count != want {
		t.Fatalf("issue count for %q = %d, want %d", title, count, want)
	}
}

func assertRoutineRunCount(t *testing.T, routineID string, want int) {
	t.Helper()
	var count int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM routine_run
		WHERE routine_id = $1
	`, routineID).Scan(&count); err != nil {
		t.Fatalf("count routine runs: %v", err)
	}
	if count != want {
		t.Fatalf("routine run count = %d, want %d", count, want)
	}
}

func assertRoutineRunStatus(t *testing.T, routineID, status string) {
	t.Helper()
	var count int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM routine_run
		WHERE routine_id = $1 AND status = $2
	`, routineID, status).Scan(&count); err != nil {
		t.Fatalf("count %s routine runs: %v", status, err)
	}
	if count == 0 {
		t.Fatalf("expected at least one %s routine run", status)
	}
}
