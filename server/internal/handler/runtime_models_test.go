package handler

import (
	"testing"
	"time"
)

func TestModelListStoreLifecycle(t *testing.T) {
	s := NewModelListStore()

	req := s.Create("rt-1")
	if req.ID == "" || req.RuntimeID != "rt-1" || req.Status != ModelListPending {
		t.Fatalf("unexpected created request: %+v", req)
	}
	if !req.Supported {
		t.Fatalf("supported should default to true")
	}

	// Pop claims the request and moves it to running.
	popped := s.PopPending("rt-1")
	if popped == nil || popped.ID != req.ID || popped.Status != ModelListRunning {
		t.Fatalf("unexpected popped request: %+v", popped)
	}
	// A second pop finds nothing.
	if again := s.PopPending("rt-1"); again != nil {
		t.Fatalf("expected nil on second pop, got %+v", again)
	}

	// Complete stores the models and supported flag.
	s.Complete(req.ID, []ModelEntry{{ID: "m1", Label: "M1", Default: true}}, true)
	got := s.Get(req.ID)
	if got == nil || got.Status != ModelListCompleted || len(got.Models) != 1 || !got.Models[0].Default {
		t.Fatalf("unexpected completed request: %+v", got)
	}
}

func TestModelListStorePopPendingForRuntimes(t *testing.T) {
	s := NewModelListStore()
	a := s.Create("rt-a")
	s.Create("rt-b")

	popped := s.PopPendingForRuntimes([]string{"rt-a", "rt-b", "rt-c"})
	if len(popped) != 2 {
		t.Fatalf("expected 2 popped requests, got %d", len(popped))
	}
	if popped[0].ID != a.ID {
		t.Fatalf("expected rt-a request first, got %+v", popped[0])
	}
}

func TestModelListStoreFail(t *testing.T) {
	s := NewModelListStore()
	req := s.Create("rt-1")
	s.Fail(req.ID, "boom")
	got := s.Get(req.ID)
	if got.Status != ModelListFailed || got.Error != "boom" {
		t.Fatalf("unexpected failed request: %+v", got)
	}
	if !modelListRequestTerminal(got.Status) {
		t.Fatalf("failed should be terminal")
	}
}

func TestApplyModelListTimeout(t *testing.T) {
	now := time.Now()

	pending := &ModelListRequest{Status: ModelListPending, CreatedAt: now.Add(-time.Minute)}
	if !applyModelListTimeout(pending, now) || pending.Status != ModelListTimeout {
		t.Fatalf("stale pending should time out: %+v", pending)
	}

	fresh := &ModelListRequest{Status: ModelListPending, CreatedAt: now}
	if applyModelListTimeout(fresh, now) {
		t.Fatalf("fresh pending should not time out")
	}

	started := now.Add(-2 * time.Minute)
	running := &ModelListRequest{Status: ModelListRunning, CreatedAt: started, runStartedAt: &started}
	if !applyModelListTimeout(running, now) || running.Status != ModelListTimeout {
		t.Fatalf("stuck running should time out: %+v", running)
	}

	done := &ModelListRequest{Status: ModelListCompleted, CreatedAt: now.Add(-time.Hour)}
	if applyModelListTimeout(done, now) {
		t.Fatalf("terminal request should never transition")
	}
}

func TestValidateModelConfig(t *testing.T) {
	cases := []struct {
		name   string
		mc     map[string]AgentModelConfig
		wantOK bool
	}{
		{"nil config", nil, true},
		{"empty config", map[string]AgentModelConfig{}, true},
		{"model only", map[string]AgentModelConfig{"claude": {Model: "claude-fable-5"}}, true},
		{"valid claude thinking", map[string]AgentModelConfig{"claude": {Model: "claude-opus-4-8", ThinkingLevel: "xhigh"}}, true},
		{"valid codex thinking", map[string]AgentModelConfig{"codex": {ThinkingLevel: "minimal"}}, true},
		{"invalid claude thinking", map[string]AgentModelConfig{"claude": {ThinkingLevel: "warp9"}}, false},
		{"thinking on unknown provider", map[string]AgentModelConfig{"gemini": {ThinkingLevel: "high"}}, false},
		{"opencode variant name", map[string]AgentModelConfig{"opencode": {ThinkingLevel: "fast-mode"}}, true},
		{"opencode invalid variant", map[string]AgentModelConfig{"opencode": {ThinkingLevel: "no spaces allowed"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := validateModelConfig(tc.mc)
			if (msg == "") != tc.wantOK {
				t.Fatalf("validateModelConfig(%+v) = %q, wantOK=%v", tc.mc, msg, tc.wantOK)
			}
		})
	}
}
