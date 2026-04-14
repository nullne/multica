package usage

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestParseClaudeFile_BasicAssistantMessage(t *testing.T) {
	tmp := t.TempDir()
	content := `{"type":"assistant","timestamp":"2026-03-15T10:00:00Z","requestId":"req-1","message":{"id":"msg-1","model":"claude-sonnet-4-20250514","usage":{"input_tokens":1000,"output_tokens":500,"cache_read_input_tokens":200,"cache_creation_input_tokens":100}}}
`
	filePath := filepath.Join(tmp, "test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	seen := make(map[string]bool)
	records := s.parseClaudeFile(filePath, seen)

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.Provider != "claude" {
		t.Errorf("provider = %q, want %q", r.Provider, "claude")
	}
	if r.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want %q", r.Model, "claude-sonnet-4-20250514")
	}
	if r.InputTokens != 1000 {
		t.Errorf("input_tokens = %d, want %d", r.InputTokens, 1000)
	}
	if r.OutputTokens != 500 {
		t.Errorf("output_tokens = %d, want %d", r.OutputTokens, 500)
	}
	if r.CacheReadTokens != 200 {
		t.Errorf("cache_read_tokens = %d, want %d", r.CacheReadTokens, 200)
	}
	if r.CacheWriteTokens != 100 {
		t.Errorf("cache_write_tokens = %d, want %d", r.CacheWriteTokens, 100)
	}
}

func TestParseClaudeFile_SkipsNonAssistant(t *testing.T) {
	tmp := t.TempDir()
	content := `{"type":"user","timestamp":"2026-03-15T10:00:00Z","message":{"id":"msg-1","model":"claude-sonnet-4-20250514","usage":{"input_tokens":1000,"output_tokens":500}}}
{"type":"system","timestamp":"2026-03-15T10:00:01Z","message":{"id":"msg-2","model":"claude-sonnet-4-20250514","usage":{"input_tokens":2000,"output_tokens":800}}}
`
	filePath := filepath.Join(tmp, "test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	seen := make(map[string]bool)
	records := s.parseClaudeFile(filePath, seen)

	if len(records) != 0 {
		t.Fatalf("expected 0 records for non-assistant messages, got %d", len(records))
	}
}

func TestParseClaudeFile_DeduplicatesMessages(t *testing.T) {
	tmp := t.TempDir()
	// Same message.id + requestId should be deduped (streaming produces multiple lines)
	content := `{"type":"assistant","timestamp":"2026-03-15T10:00:00Z","requestId":"req-1","message":{"id":"msg-1","model":"claude-sonnet-4-20250514","usage":{"input_tokens":500,"output_tokens":100}}}
{"type":"assistant","timestamp":"2026-03-15T10:00:01Z","requestId":"req-1","message":{"id":"msg-1","model":"claude-sonnet-4-20250514","usage":{"input_tokens":1000,"output_tokens":500}}}
`
	filePath := filepath.Join(tmp, "test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	seen := make(map[string]bool)
	records := s.parseClaudeFile(filePath, seen)

	if len(records) != 1 {
		t.Fatalf("expected 1 record (deduped), got %d", len(records))
	}
	// Should take the first occurrence
	if records[0].InputTokens != 500 {
		t.Errorf("input_tokens = %d, want %d (first occurrence)", records[0].InputTokens, 500)
	}
}

func TestParseClaudeFile_MultipleMessages(t *testing.T) {
	tmp := t.TempDir()
	content := `{"type":"assistant","timestamp":"2026-03-15T10:00:00Z","requestId":"req-1","message":{"id":"msg-1","model":"claude-sonnet-4-20250514","usage":{"input_tokens":1000,"output_tokens":500}}}
{"type":"assistant","timestamp":"2026-03-15T10:00:02Z","requestId":"req-2","message":{"id":"msg-2","model":"claude-sonnet-4-20250514","usage":{"input_tokens":2000,"output_tokens":800}}}
`
	filePath := filepath.Join(tmp, "test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	seen := make(map[string]bool)
	records := s.parseClaudeFile(filePath, seen)

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestParseClaudeFile_SkipsNoUsage(t *testing.T) {
	tmp := t.TempDir()
	content := `{"type":"assistant","timestamp":"2026-03-15T10:00:00Z","requestId":"req-1","message":{"id":"msg-1","model":"claude-sonnet-4-20250514"}}
`
	filePath := filepath.Join(tmp, "test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	seen := make(map[string]bool)
	records := s.parseClaudeFile(filePath, seen)

	if len(records) != 0 {
		t.Fatalf("expected 0 records when no usage, got %d", len(records))
	}
}

func TestParseClaudeFile_UnknownModel(t *testing.T) {
	tmp := t.TempDir()
	content := `{"type":"assistant","timestamp":"2026-03-15T10:00:00Z","requestId":"req-1","message":{"id":"msg-1","model":"","usage":{"input_tokens":100,"output_tokens":50}}}
`
	filePath := filepath.Join(tmp, "test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	seen := make(map[string]bool)
	records := s.parseClaudeFile(filePath, seen)

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Model != "unknown" {
		t.Errorf("model = %q, want %q", records[0].Model, "unknown")
	}
}

func TestParseClaudeFile_NonexistentFile(t *testing.T) {
	s := NewScanner(slog.Default())
	seen := make(map[string]bool)
	records := s.parseClaudeFile("/nonexistent/path.jsonl", seen)
	if len(records) != 0 {
		t.Fatalf("expected 0 records for nonexistent file, got %d", len(records))
	}
}

func TestNormalizeClaudeModel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude-sonnet-4-20250514", "claude-sonnet-4-20250514"},
		{"anthropic.claude-sonnet-4-20250514", "claude-sonnet-4-20250514"},
		{"us.anthropic.claude-sonnet-4-20250514", "claude-sonnet-4-20250514"},
		{"eu.anthropic.claude-opus-4-20250514", "claude-opus-4-20250514"},
		{"claude-haiku-4-5-20251001", "claude-haiku-4-5-20251001"},
	}
	for _, tt := range tests {
		got := normalizeClaudeModel(tt.input)
		if got != tt.want {
			t.Errorf("normalizeClaudeModel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMergeRecords(t *testing.T) {
	records := []Record{
		{Date: "2026-03-15", Provider: "claude", Model: "claude-sonnet-4-20250514", InputTokens: 1000, OutputTokens: 500},
		{Date: "2026-03-15", Provider: "claude", Model: "claude-sonnet-4-20250514", InputTokens: 2000, OutputTokens: 800},
		{Date: "2026-03-15", Provider: "codex", Model: "gpt-5", InputTokens: 500, OutputTokens: 200},
	}

	merged := mergeRecords(records)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged records, got %d", len(merged))
	}

	// Find the claude record
	var claudeRecord *Record
	for i := range merged {
		if merged[i].Provider == "claude" {
			claudeRecord = &merged[i]
			break
		}
	}
	if claudeRecord == nil {
		t.Fatal("expected to find claude record")
	}
	if claudeRecord.InputTokens != 3000 {
		t.Errorf("merged input_tokens = %d, want %d", claudeRecord.InputTokens, 3000)
	}
	if claudeRecord.OutputTokens != 1300 {
		t.Errorf("merged output_tokens = %d, want %d", claudeRecord.OutputTokens, 1300)
	}
}

func TestMergeRecords_Empty(t *testing.T) {
	merged := mergeRecords(nil)
	if len(merged) != 0 {
		t.Fatalf("expected 0 merged records, got %d", len(merged))
	}
}
