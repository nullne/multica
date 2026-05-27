package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestRoutineRoutesEnforceMemberReadOnly(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("find agent: %v", err)
	}

	createResp := authRequest(t, "POST", "/api/routines", map[string]any{
		"name":          "Routine permissions",
		"instructions":  "Members can view only",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   agentID,
		"triggers": []map[string]any{
			{"trigger_type": "schedule", "schedule": "0 9 * * 1", "timezone": "UTC"},
		},
	})
	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		createResp.Body.Close()
		t.Fatalf("CreateRoutine: expected 201, got %d: %s", createResp.StatusCode, body)
	}
	var routine struct {
		ID       string `json:"id"`
		Triggers []struct {
			ID string `json:"id"`
		} `json:"triggers"`
	}
	readJSON(t, createResp, &routine)
	t.Cleanup(func() {
		resp := authRequest(t, "DELETE", "/api/routines/"+routine.ID, nil)
		resp.Body.Close()
	})

	var memberID string
	email := fmt.Sprintf("routine-member-%d@example.com", time.Now().UnixNano())
	if err := testPool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('Routine Member', $1) RETURNING id`, email).Scan(&memberID); err != nil {
		t.Fatalf("insert member user: %v", err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, testWorkspaceID, memberID); err != nil {
		t.Fatalf("insert member role: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, memberID) })
	memberToken, err := generateTestJWT(memberID, email, "Routine Member")
	if err != nil {
		t.Fatalf("generate member token: %v", err)
	}

	if resp := routineRequestWithToken(t, memberToken, "GET", "/api/routines", nil); resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("member list routines: expected 200, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp := routineRequestWithToken(t, memberToken, "GET", "/api/routines/"+routine.ID, nil); resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("member get routine: expected 200, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	for _, tc := range []struct {
		method string
		path   string
		body   any
	}{
		{"POST", "/api/routines", map[string]any{"name": "Nope"}},
		{"POST", "/api/routine-trigger-token-drafts", nil},
		{"POST", "/api/routines/" + routine.ID + "/trigger", nil},
		{"POST", "/api/routines/" + routine.ID + "/triggers/" + routine.Triggers[0].ID + "/regenerate-token", nil},
		{"PATCH", "/api/routines/" + routine.ID, map[string]any{"name": "Nope"}},
		{"DELETE", "/api/routines/" + routine.ID, nil},
	} {
		resp := routineRequestWithToken(t, memberToken, tc.method, tc.path, tc.body)
		if resp.StatusCode != http.StatusForbidden {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("%s %s: expected 403, got %d: %s", tc.method, tc.path, resp.StatusCode, body)
		}
		resp.Body.Close()
	}
}

func routineRequestWithToken(t *testing.T, token, method, path string, body any) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, testServer.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}
