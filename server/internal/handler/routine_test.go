package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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
		"triggers": []map[string]any{
			{
				"trigger_type": "api",
				"source_type":  "standard",
			},
		},
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
	req = newRequest("POST", "/api/routine-triggers/"+routine.Triggers[0].ID, map[string]any{
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
	req.Header.Set("Authorization", "Bearer "+routine.Triggers[0].Token)
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

func TestRoutineAPITriggerAuthRegenerateFilterAndDedup(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine API auth filter dedup",
		"instructions":  "Created from API",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"enabled":       true,
		"triggers": []map[string]any{
			{
				"trigger_type":         "api",
				"source_type":          "standard",
				"dedup_window_seconds": 600,
			},
		},
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
	oldToken := trigger.Token

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/routine-triggers/"+trigger.ID, map[string]any{"title": "No token"})
	req = withURLParam(req, "id", trigger.ID)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: expected 401, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/routine-triggers/"+trigger.ID, map[string]any{"title": "Wrong token"})
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
	req = newRequest("POST", "/api/routine-triggers/"+trigger.ID, map[string]any{"title": "Old token"})
	req = withURLParam(req, "id", trigger.ID)
	req.Header.Set("Authorization", "Bearer "+oldToken)
	testHandler.IngestRoutineTrigger(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old token: expected 401 after regenerate, got %d", w.Code)
	}

	ingest := func(eventType, dedupKey, title string) {
		t.Helper()
		w = httptest.NewRecorder()
		req = newRequest("POST", "/api/routine-triggers/"+trigger.ID, map[string]any{
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
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine API regenerate",
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
		"name":          "Routine manual trigger",
		"instructions":  "Created manually",
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
	if err := testPool.QueryRow(context.Background(), `
		SELECT id::text FROM issue
		WHERE workspace_id = $1 AND title = 'Routine manual trigger'
		ORDER BY created_at DESC LIMIT 1
	`, testWorkspaceID).Scan(&issueID); err != nil {
		t.Fatalf("query created issue: %v", err)
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
	w := httptest.NewRecorder()
	req := routineRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine isolation",
		"instructions":  "Only workspace A",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"triggers":      []map[string]any{{"trigger_type": "api", "source_type": "standard"}},
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
