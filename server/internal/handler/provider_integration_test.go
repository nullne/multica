package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// GetWorkspaceProviders & UpdateWorkspaceProviders (integration — needs DB)
// ---------------------------------------------------------------------------

func TestProvidersCRUD(t *testing.T) {
	// 1. GET providers — empty initially
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/providers", nil)
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.GetWorkspaceProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetWorkspaceProviders: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var initial WorkspaceProviderSettings
	json.NewDecoder(w.Body).Decode(&initial)
	if initial.Providers != nil && len(initial.Providers) > 0 {
		t.Fatalf("expected no providers initially, got %+v", initial.Providers)
	}

	// 2. PATCH — set up claude with API key
	w = httptest.NewRecorder()
	req = newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/providers", WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"claude": {Enabled: true, APIKey: "sk-ant-secret-key-12345", TargetVersion: "2.1.0"},
		},
		MulticaTargetVersion: "0.1.15",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateWorkspaceProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateWorkspaceProviders: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated WorkspaceProviderSettings
	json.NewDecoder(w.Body).Decode(&updated)

	// Response should have redacted API key.
	claude := updated.Providers["claude"]
	if !claude.Enabled {
		t.Fatal("claude should be enabled in response")
	}
	if claude.APIKey == "sk-ant-secret-key-12345" {
		t.Fatal("API key should be redacted in response, got the original")
	}
	if claude.APIKey == "" {
		t.Fatal("API key should not be empty (should be masked)")
	}
	if claude.TargetVersion != "2.1.0" {
		t.Fatalf("expected target version 2.1.0, got %q", claude.TargetVersion)
	}

	// 3. GET — verify persisted and redacted
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/providers", nil)
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.GetWorkspaceProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetWorkspaceProviders after update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fetched WorkspaceProviderSettings
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.MulticaTargetVersion != "0.1.15" {
		t.Fatalf("expected multica version 0.1.15, got %q", fetched.MulticaTargetVersion)
	}
	if fetched.Providers["claude"].APIKey == "sk-ant-secret-key-12345" {
		t.Fatal("GET response should have redacted API key")
	}
	if !fetched.Providers["claude"].Enabled {
		t.Fatal("claude should be enabled")
	}

	// 4. Cleanup: reset settings to empty
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `UPDATE workspace SET settings = '{}'::jsonb WHERE id = $1`, testWorkspaceID)
	})
}

func TestUpdateProviders_RedactedKeyPreservation(t *testing.T) {
	// Step 1: Store a real API key.
	w := httptest.NewRecorder()
	req := newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/providers", WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"claude": {Enabled: true, APIKey: "sk-ant-original-key-999"},
		},
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateWorkspaceProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Step 1: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Step 2: Send back with redacted (all-asterisk) API key — should KEEP the original.
	w = httptest.NewRecorder()
	req = newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/providers", WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"claude": {Enabled: true, APIKey: "****", TargetVersion: "3.0.0"},
		},
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateWorkspaceProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Step 2: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Step 3: Verify the original key was preserved in DB.
	var rawSettings []byte
	err := testPool.QueryRow(context.Background(), `SELECT settings FROM workspace WHERE id = $1`, testWorkspaceID).Scan(&rawSettings)
	if err != nil {
		t.Fatalf("failed to read workspace settings: %v", err)
	}

	ps := parseProviderSettings(rawSettings)
	if ps.Providers["claude"].APIKey != "sk-ant-original-key-999" {
		t.Fatalf("API key should be preserved as original, got %q", ps.Providers["claude"].APIKey)
	}
	// Target version should have been updated.
	if ps.Providers["claude"].TargetVersion != "3.0.0" {
		t.Fatalf("target version should be updated to 3.0.0, got %q", ps.Providers["claude"].TargetVersion)
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `UPDATE workspace SET settings = '{}'::jsonb WHERE id = $1`, testWorkspaceID)
	})
}

func TestUpdateProviders_NewKeyReplacesOld(t *testing.T) {
	// Store an initial key.
	w := httptest.NewRecorder()
	req := newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/providers", WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"claude": {Enabled: true, APIKey: "sk-ant-old-key"},
		},
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateWorkspaceProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Step 1: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Now send a new (non-redacted) key — should replace.
	w = httptest.NewRecorder()
	req = newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/providers", WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"claude": {Enabled: true, APIKey: "sk-ant-new-key"},
		},
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateWorkspaceProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Step 2: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify new key in DB.
	var rawSettings []byte
	err := testPool.QueryRow(context.Background(), `SELECT settings FROM workspace WHERE id = $1`, testWorkspaceID).Scan(&rawSettings)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	ps := parseProviderSettings(rawSettings)
	if ps.Providers["claude"].APIKey != "sk-ant-new-key" {
		t.Fatalf("expected new key, got %q", ps.Providers["claude"].APIKey)
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `UPDATE workspace SET settings = '{}'::jsonb WHERE id = $1`, testWorkspaceID)
	})
}

func TestUpdateProviders_UnsupportedProvider(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/providers", WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"gpt5": {Enabled: true, APIKey: "sk-invalid"},
		},
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateWorkspaceProviders(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported provider, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "unsupported provider: gpt5" {
		t.Fatalf("expected unsupported provider error, got %q", errResp["error"])
	}
}

func TestUpdateProviders_PreservesOtherSettings(t *testing.T) {
	// Store a non-provider setting first.
	_, err := testPool.Exec(context.Background(),
		`UPDATE workspace SET settings = '{"custom_field": "keep_me"}'::jsonb WHERE id = $1`,
		testWorkspaceID,
	)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Update providers.
	w := httptest.NewRecorder()
	req := newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/providers", WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"codex": {Enabled: true},
		},
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateWorkspaceProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify custom_field is still there.
	var rawSettings []byte
	testPool.QueryRow(context.Background(), `SELECT settings FROM workspace WHERE id = $1`, testWorkspaceID).Scan(&rawSettings)

	var full map[string]json.RawMessage
	json.Unmarshal(rawSettings, &full)
	if _, ok := full["custom_field"]; !ok {
		t.Fatal("custom_field was lost after provider update")
	}
	if _, ok := full["providers"]; !ok {
		t.Fatal("providers not written")
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `UPDATE workspace SET settings = '{}'::jsonb WHERE id = $1`, testWorkspaceID)
	})
}

func TestGetProviders_WorkspaceNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/workspaces/00000000-0000-0000-0000-000000000000/providers", nil)
	req = withURLParam(req, "id", "00000000-0000-0000-0000-000000000000")
	testHandler.GetWorkspaceProviders(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent workspace, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// UpdateAllDaemons
// ---------------------------------------------------------------------------

func TestUpdateAllDaemons_NoTargets(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/update-all-daemons", map[string]any{
		"targets": []any{},
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateAllDaemons(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty targets, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateAllDaemons_NoDaemonsOnline(t *testing.T) {
	// Insert a runtime with a daemon_id but offline status.
	var runtimeID string
	err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata
		) VALUES ($1, 'offline-daemon', 'Offline Test', 'local', 'claude', 'offline', 'test', '{}'::jsonb)
		RETURNING id
	`, testWorkspaceID).Scan(&runtimeID)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/update-all-daemons", map[string]any{
		"targets": []map[string]string{
			{"target": "multica", "version": "0.2.0"},
		},
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateAllDaemons(w, req)

	// The fixture runtime has NULL daemon_id, the new one is offline.
	// Neither qualifies → 400.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no online daemons, got %d: %s", w.Code, w.Body.String())
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
	})
}

func TestUpdateAllDaemons_QueuesUpdates(t *testing.T) {
	// Insert an online runtime with a daemon_id so UpdateAllDaemons can find it.
	var runtimeID string
	err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at
		) VALUES ($1, 'test-daemon-1', 'Test Daemon Claude', 'local', 'claude', 'online', 'test-machine', '{}'::jsonb, now())
		RETURNING id
	`, testWorkspaceID).Scan(&runtimeID)
	if err != nil {
		t.Fatalf("setup runtime: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/update-all-daemons", map[string]any{
		"targets": []map[string]string{
			{"target": "multica", "version": "0.2.0"},
			{"target": "claude", "version": "2.1.0"},
		},
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.UpdateAllDaemons(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	daemonCount := int(resp["daemons_count"].(float64))
	updatesQueued := int(resp["updates_queued"].(float64))

	if daemonCount < 1 {
		t.Fatalf("expected at least 1 daemon, got %d", daemonCount)
	}
	if updatesQueued < 2 {
		t.Fatalf("expected at least 2 updates queued (multica + claude), got %d", updatesQueued)
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
	})
}

// ---------------------------------------------------------------------------
// DaemonRegister — provider_config in response
// ---------------------------------------------------------------------------

func TestDaemonRegister_IncludesProviderConfig(t *testing.T) {
	// Set up workspace with a provider config first.
	_, err := testPool.Exec(context.Background(), `
		UPDATE workspace SET settings = $1 WHERE id = $2
	`, `{"providers":{"claude":{"enabled":true,"api_key":"sk-ant-ws-key","target_version":"2.0.0"}}}`, testWorkspaceID)
	if err != nil {
		t.Fatalf("setup settings: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/register", DaemonRegisterRequest{
		WorkspaceID: testWorkspaceID,
		DaemonID:    "test-register-daemon",
		DeviceName:  "test-device",
		CLIVersion:  "0.1.0",
		Runtimes: []struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Version string `json:"version"`
			Status  string `json:"status"`
		}{
			{Name: "Claude", Type: "claude", Version: "2.0.0", Status: "online"},
		},
	})
	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("DaemonRegister: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	// Check provider_config is present and unredacted.
	pc, ok := resp["provider_config"].(map[string]any)
	if !ok {
		t.Fatalf("expected provider_config in response, got %+v", resp)
	}
	claudeCfg, ok := pc["claude"].(map[string]any)
	if !ok {
		t.Fatalf("expected claude in provider_config, got %+v", pc)
	}
	if claudeCfg["api_key"] != "sk-ant-ws-key" {
		t.Fatalf("expected unredacted API key in daemon response, got %v", claudeCfg["api_key"])
	}
	if claudeCfg["enabled"] != true {
		t.Fatalf("expected enabled=true, got %v", claudeCfg["enabled"])
	}

	// Check multica_target_version is absent when not set.
	// (We didn't set it in settings above.)

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `UPDATE workspace SET settings = '{}'::jsonb WHERE id = $1`, testWorkspaceID)
		testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE daemon_id = 'test-register-daemon'`)
	})
}

// ---------------------------------------------------------------------------
// ClaimTaskByRuntime — provider API key injection
// ---------------------------------------------------------------------------

func TestClaimTask_ProviderAPIKeyInjected(t *testing.T) {
	// 1. Set workspace provider config with API key.
	_, err := testPool.Exec(context.Background(), `
		UPDATE workspace SET settings = $1 WHERE id = $2
	`, `{"providers":{"claim_test_provider":{"enabled":true,"api_key":"sk-test-claim-key"}}}`, testWorkspaceID)
	if err != nil {
		t.Fatalf("setup settings: %v", err)
	}

	// 2. Create a runtime with provider = "claim_test_provider".
	var runtimeID string
	err = testPool.QueryRow(context.Background(), `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at
		) VALUES ($1, 'claim-daemon', 'Claim Test', 'local', 'claim_test_provider', 'online', 'test', '{}'::jsonb, now())
		RETURNING id
	`, testWorkspaceID).Scan(&runtimeID)
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	// 3. Create an agent pointing to this runtime.
	var agentID string
	err = testPool.QueryRow(context.Background(), `
		INSERT INTO agent (
			workspace_id, name, runtime_mode, runtime_config, runtime_id,
			visibility, max_concurrent_tasks, owner_id, tools, triggers
		) VALUES ($1, 'Claim Agent', 'local', '{}'::jsonb, $2,
			'workspace', 1, $3, '[]'::jsonb, '[]'::jsonb)
		RETURNING id
	`, testWorkspaceID, runtimeID, testUserID).Scan(&agentID)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// 4. Create an issue and task.
	var issueID string
	err = testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position)
		VALUES ($1, 'Claim Test Issue', 'todo', 'medium', 'member', $2, 9998, 0)
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&issueID)
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	_, err = testPool.Exec(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority)
		VALUES ($1, $2, $3, 'queued', 0)
	`, agentID, runtimeID, issueID)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// 5. Claim the task.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", map[string]any{})
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.ClaimTaskByRuntime(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ClaimTask: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var claimResp map[string]any
	json.NewDecoder(w.Body).Decode(&claimResp)

	task, ok := claimResp["task"].(map[string]any)
	if !ok || task == nil {
		t.Fatal("expected a task in claim response")
	}

	apiKey, _ := task["provider_api_key"].(string)
	if apiKey != "sk-test-claim-key" {
		t.Fatalf("expected provider_api_key = 'sk-test-claim-key', got %q", apiKey)
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
		testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
		testPool.Exec(context.Background(), `UPDATE workspace SET settings = '{}'::jsonb WHERE id = $1`, testWorkspaceID)
	})
}
