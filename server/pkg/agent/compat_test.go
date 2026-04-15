//go:build agentcompat

package agent

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"
)

// agentKeyEnv maps agent type to the environment variable holding the API key.
var agentKeyEnv = map[string]string{
	"claude":   "ANTHROPIC_API_KEY",
	"codex":    "OPENAI_API_KEY",
	"opencode": "OPENAI_API_KEY",
	"cursor":   "CURSOR_API_KEY",
}

// defaultExecName maps agent type to its default CLI binary name.
var defaultExecName = map[string]string{
	"claude":   "claude",
	"codex":    "codex",
	"opencode": "opencode",
	"cursor":   "agent",
}

// validMessageTypes is the set of message types our code defines.
var validMessageTypes = map[MessageType]bool{
	MessageText:       true,
	MessageThinking:   true,
	MessageToolUse:    true,
	MessageToolResult: true,
	MessageStatus:     true,
	MessageError:      true,
	MessageLog:        true,
}

// requireAgent skips the test if the API key is missing or the CLI is not installed.
// It returns a Config ready for agent.New().
func requireAgent(t *testing.T, agentType string) Config {
	t.Helper()

	envName := agentKeyEnv[agentType]
	key := os.Getenv(envName)
	if key == "" {
		t.Skipf("skipping %s: %s not set", agentType, envName)
	}

	binName := defaultExecName[agentType]
	p, err := exec.LookPath(binName)
	if err != nil {
		t.Skipf("skipping %s: CLI %q not found in PATH", agentType, binName)
	}

	return Config{
		ExecutablePath: p,
		Logger:         slog.Default(),
	}
}

// buildAgentEnv mirrors the environment setup in daemon.runTask:
// system MULTICA_* vars + provider API key injection.
func buildAgentEnv(agentType string) map[string]string {
	env := map[string]string{
		"MULTICA_TOKEN":        "test-token",
		"MULTICA_SERVER_URL":   "http://localhost:0",
		"MULTICA_DAEMON_PORT":  "0",
		"MULTICA_WORKSPACE_ID": "compat-test",
		"MULTICA_AGENT_NAME":   "compat-test-agent",
		"MULTICA_AGENT_ID":     "compat-test-agent-id",
		"MULTICA_TASK_ID":      "compat-test-task-id",
	}

	switch agentType {
	case "claude":
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			env["ANTHROPIC_API_KEY"] = key
		}
	case "codex":
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			env["OPENAI_API_KEY"] = key
		}
	case "opencode":
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			env["OPENAI_API_KEY"] = key
		}
	case "cursor":
		if key := os.Getenv("CURSOR_API_KEY"); key != "" {
			env["CURSOR_API_KEY"] = key
		}
	}

	return env
}

// drainSession reads all messages and the final result from a session.
func drainSession(t *testing.T, session *Session) ([]Message, Result) {
	t.Helper()

	var msgs []Message
	for m := range session.Messages {
		msgs = append(msgs, m)
	}
	result := <-session.Result
	return msgs, result
}

// runCompatTest is the shared test body for all agents.
// It walks the same path as daemon.runTask: build env, prepare workdir,
// create backend, execute a test prompt, drain messages, validate output.
func runCompatTest(t *testing.T, agentType string) {
	t.Helper()

	cfg := requireAgent(t, agentType)
	cfg.Env = buildAgentEnv(agentType)

	backend, err := New(agentType, cfg)
	if err != nil {
		t.Fatalf("New(%s) error: %v", agentType, err)
	}

	workDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	session, err := backend.Execute(ctx, "What is 2+2? Reply with just the number.", ExecOptions{
		Cwd:      workDir,
		MaxTurns: 1,
		Timeout:  2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	msgs, result := drainSession(t, session)

	// --- Validate Result ---
	if result.Status != "completed" {
		t.Errorf("Result.Status = %q, want %q (error: %s)", result.Status, "completed", result.Error)
	}
	if result.Output == "" {
		t.Error("Result.Output is empty, expected non-empty text")
	}
	if result.Error != "" {
		t.Errorf("Result.Error = %q, expected empty", result.Error)
	}
	if result.DurationMs <= 0 {
		t.Errorf("Result.DurationMs = %d, expected > 0", result.DurationMs)
	}

	// --- Validate Messages ---
	if len(msgs) == 0 {
		t.Fatal("no messages received, expected at least one")
	}

	hasText := false
	for _, m := range msgs {
		if !validMessageTypes[m.Type] {
			t.Errorf("unknown message type %q", m.Type)
		}
		if m.Type == MessageText {
			hasText = true
		}
	}
	if !hasText {
		t.Error("no MessageText in message stream, expected at least one")
	}

	t.Logf("%s: %d messages, result.Status=%s, output=%q",
		agentType, len(msgs), result.Status, truncate(result.Output, 100))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
