package protocol

import (
	"encoding/json"
	"testing"
)

func TestMessage_MarshalUnmarshal(t *testing.T) {
	payload := TaskProgressPayload{
		TaskID:  "task-123",
		Summary: "Working on it",
		Step:    1,
		Total:   3,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := Message{
		Type:    EventTaskProgress,
		Payload: payloadBytes,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Type != EventTaskProgress {
		t.Errorf("type = %q, want %q", decoded.Type, EventTaskProgress)
	}

	var decodedPayload TaskProgressPayload
	if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
		t.Fatalf("Unmarshal payload error: %v", err)
	}
	if decodedPayload.TaskID != "task-123" {
		t.Errorf("task_id = %q, want %q", decodedPayload.TaskID, "task-123")
	}
	if decodedPayload.Step != 1 {
		t.Errorf("step = %d, want %d", decodedPayload.Step, 1)
	}
}

func TestTaskDispatchPayload_RoundTrip(t *testing.T) {
	original := TaskDispatchPayload{
		TaskID:      "task-abc",
		IssueID:     "issue-def",
		Title:       "Fix the bug",
		Description: "The login page is broken",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TaskDispatchPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip failed: got %+v, want %+v", decoded, original)
	}
}

func TestTaskCompletedPayload_OptionalFields(t *testing.T) {
	// Test with optional fields omitted
	original := TaskCompletedPayload{
		TaskID: "task-123",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Verify optional fields are omitted from JSON
	var raw map[string]any
	json.Unmarshal(data, &raw)
	if _, ok := raw["pr_url"]; ok {
		t.Error("pr_url should be omitted when empty")
	}
	if _, ok := raw["output"]; ok {
		t.Error("output should be omitted when empty")
	}
}

func TestTaskMessagePayload_RoundTrip(t *testing.T) {
	original := TaskMessagePayload{
		TaskID:  "task-123",
		IssueID: "issue-456",
		Seq:     5,
		Type:    "tool_use",
		Tool:    "Read",
		Content: "",
		Input:   map[string]any{"file_path": "/src/main.go"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TaskMessagePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.TaskID != original.TaskID {
		t.Errorf("task_id = %q, want %q", decoded.TaskID, original.TaskID)
	}
	if decoded.Seq != 5 {
		t.Errorf("seq = %d, want %d", decoded.Seq, 5)
	}
	if decoded.Tool != "Read" {
		t.Errorf("tool = %q, want %q", decoded.Tool, "Read")
	}
	if decoded.Input["file_path"] != "/src/main.go" {
		t.Errorf("input file_path = %v, want %q", decoded.Input["file_path"], "/src/main.go")
	}
}

func TestDaemonRegisterPayload_RoundTrip(t *testing.T) {
	original := DaemonRegisterPayload{
		DaemonID: "daemon-1",
		AgentID:  "agent-1",
		Runtimes: []RuntimeInfo{
			{Type: "claude", Version: "1.0.0", Status: "ready"},
			{Type: "codex", Version: "2.0.0", Status: "ready"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded DaemonRegisterPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.DaemonID != "daemon-1" {
		t.Errorf("daemon_id = %q, want %q", decoded.DaemonID, "daemon-1")
	}
	if len(decoded.Runtimes) != 2 {
		t.Fatalf("runtimes count = %d, want %d", len(decoded.Runtimes), 2)
	}
	if decoded.Runtimes[0].Type != "claude" {
		t.Errorf("runtime[0].type = %q, want %q", decoded.Runtimes[0].Type, "claude")
	}
}

func TestHeartbeatPayload_RoundTrip(t *testing.T) {
	original := HeartbeatPayload{
		DaemonID:     "daemon-1",
		AgentID:      "agent-1",
		CurrentTasks: 3,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded HeartbeatPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip failed: got %+v, want %+v", decoded, original)
	}
}

func TestEventConstants(t *testing.T) {
	// Verify event constants have expected format
	events := map[string]string{
		"EventIssueCreated":    EventIssueCreated,
		"EventCommentCreated":  EventCommentCreated,
		"EventTaskDispatch":    EventTaskDispatch,
		"EventTaskProgress":    EventTaskProgress,
		"EventTaskCompleted":   EventTaskCompleted,
		"EventDaemonHeartbeat": EventDaemonHeartbeat,
		"EventDaemonRegister":  EventDaemonRegister,
	}

	for name, value := range events {
		if value == "" {
			t.Errorf("%s should not be empty", name)
		}
	}

	// Verify specific values
	if EventIssueCreated != "issue:created" {
		t.Errorf("EventIssueCreated = %q, want %q", EventIssueCreated, "issue:created")
	}
	if EventTaskDispatch != "task:dispatch" {
		t.Errorf("EventTaskDispatch = %q, want %q", EventTaskDispatch, "task:dispatch")
	}
}
