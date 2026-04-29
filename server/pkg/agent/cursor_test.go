package agent

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNewReturnsCursorBackend(t *testing.T) {
	t.Parallel()
	b, err := New("cursor", Config{ExecutablePath: "/nonexistent/agent"})
	if err != nil {
		t.Fatalf("New(cursor) error: %v", err)
	}
	if _, ok := b.(*cursorBackend); !ok {
		t.Fatalf("expected *cursorBackend, got %T", b)
	}
}

func TestCursorHandleAssistantText(t *testing.T) {
	t.Parallel()

	b := &cursorBackend{cfg: Config{Logger: slog.Default()}}
	ch := make(chan Message, 10)
	var output strings.Builder

	msg := cursorSDKMessage{
		Type: "assistant",
		Message: mustMarshal(t, cursorMessageContent{
			Role: "assistant",
			Content: []cursorContentBlock{
				{Type: "text", Text: "Hello from cursor"},
			},
		}),
	}

	b.handleAssistant(msg, ch, &output)

	if output.String() != "Hello from cursor" {
		t.Fatalf("expected output 'Hello from cursor', got %q", output.String())
	}
	select {
	case m := <-ch:
		if m.Type != MessageText || m.Content != "Hello from cursor" {
			t.Fatalf("unexpected message: %+v", m)
		}
	default:
		t.Fatal("expected message on channel")
	}
}

func TestCursorHandleAssistantToolUse(t *testing.T) {
	t.Parallel()

	b := &cursorBackend{cfg: Config{Logger: slog.Default()}}
	ch := make(chan Message, 10)
	var output strings.Builder

	msg := cursorSDKMessage{
		Type: "assistant",
		Message: mustMarshal(t, cursorMessageContent{
			Role: "assistant",
			Content: []cursorContentBlock{
				{
					Type:  "tool_use",
					ID:    "call-1",
					Name:  "Read",
					Input: mustMarshal(t, map[string]any{"path": "/tmp/foo"}),
				},
			},
		}),
	}

	b.handleAssistant(msg, ch, &output)

	if output.String() != "" {
		t.Fatalf("tool_use should not add to output, got %q", output.String())
	}
	select {
	case m := <-ch:
		if m.Type != MessageToolUse || m.Tool != "Read" || m.CallID != "call-1" {
			t.Fatalf("unexpected message: %+v", m)
		}
		if m.Input["path"] != "/tmp/foo" {
			t.Fatalf("expected input path /tmp/foo, got %v", m.Input["path"])
		}
	default:
		t.Fatal("expected message on channel")
	}
}

func TestCursorHandleAssistantThinking(t *testing.T) {
	t.Parallel()

	b := &cursorBackend{cfg: Config{Logger: slog.Default()}}
	ch := make(chan Message, 10)
	var output strings.Builder

	msg := cursorSDKMessage{
		Type: "assistant",
		Message: mustMarshal(t, cursorMessageContent{
			Role: "assistant",
			Content: []cursorContentBlock{
				{Type: "thinking", Text: "Let me think..."},
			},
		}),
	}

	b.handleAssistant(msg, ch, &output)

	if output.String() != "" {
		t.Fatalf("thinking should not add to output, got %q", output.String())
	}
	select {
	case m := <-ch:
		if m.Type != MessageThinking || m.Content != "Let me think..." {
			t.Fatalf("unexpected message: %+v", m)
		}
	default:
		t.Fatal("expected message on channel")
	}
}

func TestCursorHandleUserToolResult(t *testing.T) {
	t.Parallel()

	b := &cursorBackend{cfg: Config{Logger: slog.Default()}}
	ch := make(chan Message, 10)

	msg := cursorSDKMessage{
		Type: "user",
		Message: mustMarshal(t, cursorMessageContent{
			Role: "user",
			Content: []cursorContentBlock{
				{
					Type:      "tool_result",
					ToolUseID: "call-1",
					Content:   mustMarshal(t, "file contents here"),
				},
			},
		}),
	}

	b.handleUser(msg, ch)

	select {
	case m := <-ch:
		if m.Type != MessageToolResult || m.CallID != "call-1" {
			t.Fatalf("unexpected message: %+v", m)
		}
	default:
		t.Fatal("expected message on channel")
	}
}

func TestCursorHandleToolCallStarted(t *testing.T) {
	t.Parallel()

	b := &cursorBackend{cfg: Config{Logger: slog.Default()}}
	ch := make(chan Message, 10)
	var output strings.Builder

	// New Cursor format: call_id at top level, tool_call keyed by tool type.
	msg := cursorSDKMessage{
		Type:     "tool_call",
		Subtype:  "started",
		CallID:   "tc-1",
		ToolCall: mustMarshal(t, map[string]any{"shellToolCall": map[string]any{"command": "ls -la"}}),
	}

	b.handleToolCall(msg, ch, &output)

	select {
	case m := <-ch:
		if m.Type != MessageToolUse || m.Tool != "shellToolCall" || m.CallID != "tc-1" {
			t.Fatalf("unexpected message: %+v", m)
		}
		if m.Input["command"] != "ls -la" {
			t.Fatalf("expected input command 'ls -la', got %v", m.Input["command"])
		}
	default:
		t.Fatal("expected message on channel")
	}
}

func TestCursorHandleToolCallCompleted(t *testing.T) {
	t.Parallel()

	b := &cursorBackend{cfg: Config{Logger: slog.Default()}}
	ch := make(chan Message, 10)
	var output strings.Builder

	// New Cursor format: call_id at top level, tool_call keyed by tool type.
	msg := cursorSDKMessage{
		Type:    "tool_call",
		Subtype: "completed",
		CallID:  "tc-1",
		ToolCall: mustMarshal(t, map[string]any{
			"shellToolCall": map[string]any{
				"exitCode": 0,
				"stdout":   "total 42\ndrwxr-xr-x ...",
				"stderr":   "",
			},
		}),
	}

	b.handleToolCall(msg, ch, &output)

	select {
	case m := <-ch:
		if m.Type != MessageToolResult || m.Tool != "shellToolCall" || m.CallID != "tc-1" {
			t.Fatalf("unexpected message: %+v", m)
		}
		if m.Output == "" {
			t.Fatal("expected non-empty output")
		}
	default:
		t.Fatal("expected message on channel")
	}
}

func TestCursorHandleToolCallFixture(t *testing.T) {
	t.Parallel()

	b := &cursorBackend{cfg: Config{Logger: slog.Default()}}

	// Real modern Cursor stream-json tool_call events.
	startedLine := `{"type":"tool_call","subtype":"started","call_id":"curs-42","tool_call":{"shellToolCall":{"command":"echo hello"}}}`
	completedLine := `{"type":"tool_call","subtype":"completed","call_id":"curs-42","tool_call":{"shellToolCall":{"exitCode":0,"stdout":"hello\n","stderr":""}}}`

	for _, tc := range []struct {
		line        string
		wantMsgType MessageType
		wantTool    string
		wantCallID  string
	}{
		{startedLine, MessageToolUse, "shellToolCall", "curs-42"},
		{completedLine, MessageToolResult, "shellToolCall", "curs-42"},
	} {
		var msg cursorSDKMessage
		if err := json.Unmarshal([]byte(tc.line), &msg); err != nil {
			t.Fatalf("unmarshal fixture: %v", err)
		}

		ch := make(chan Message, 10)
		var out strings.Builder
		b.handleToolCall(msg, ch, &out)

		select {
		case m := <-ch:
			if m.Type != tc.wantMsgType {
				t.Errorf("line %q: Type = %q, want %q", tc.line, m.Type, tc.wantMsgType)
			}
			if m.Tool != tc.wantTool {
				t.Errorf("line %q: Tool = %q, want %q", tc.line, m.Tool, tc.wantTool)
			}
			if m.CallID != tc.wantCallID {
				t.Errorf("line %q: CallID = %q, want %q", tc.line, m.CallID, tc.wantCallID)
			}
		default:
			t.Fatalf("line %q: expected message on channel", tc.line)
		}
	}
}

func TestCursorHandleAssistantInvalidJSON(t *testing.T) {
	t.Parallel()

	b := &cursorBackend{cfg: Config{Logger: slog.Default()}}
	ch := make(chan Message, 10)
	var output strings.Builder

	msg := cursorSDKMessage{
		Type:    "assistant",
		Message: json.RawMessage(`invalid json`),
	}

	// Should not panic
	b.handleAssistant(msg, ch, &output)

	if output.String() != "" {
		t.Fatalf("expected empty output for invalid JSON, got %q", output.String())
	}
	select {
	case m := <-ch:
		t.Fatalf("expected no message, got %+v", m)
	default:
	}
}

func TestCursorHandleToolCallInvalidJSON(t *testing.T) {
	t.Parallel()

	b := &cursorBackend{cfg: Config{Logger: slog.Default()}}
	ch := make(chan Message, 10)
	var output strings.Builder

	msg := cursorSDKMessage{
		Type:     "tool_call",
		Subtype:  "started",
		ToolCall: json.RawMessage(`invalid json`),
	}

	// Should not panic
	b.handleToolCall(msg, ch, &output)

	select {
	case m := <-ch:
		t.Fatalf("expected no message, got %+v", m)
	default:
	}
}
