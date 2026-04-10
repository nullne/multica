package handler

import (
	"testing"
)

func TestUpdateStoreCreateWithTarget(t *testing.T) {
	t.Parallel()

	store := NewUpdateStore()

	// Create an update for multica CLI.
	req, err := store.CreateWithTarget("runtime-1", "multica", "0.2.0")
	if err != nil {
		t.Fatalf("CreateWithTarget failed: %v", err)
	}
	if req.Target != "multica" {
		t.Fatalf("expected target 'multica', got %q", req.Target)
	}
	if req.TargetVersion != "0.2.0" {
		t.Fatalf("expected version '0.2.0', got %q", req.TargetVersion)
	}
	if req.Status != UpdatePending {
		t.Fatalf("expected status pending, got %v", req.Status)
	}

	// Create an update for a different target on the same runtime should succeed.
	req2, err := store.CreateWithTarget("runtime-1", "claude", "2.1.0")
	if err != nil {
		t.Fatalf("CreateWithTarget for different target should succeed: %v", err)
	}
	if req2.Target != "claude" {
		t.Fatalf("expected target 'claude', got %q", req2.Target)
	}

	// Creating a duplicate (same runtime + same target) should fail.
	_, err = store.CreateWithTarget("runtime-1", "multica", "0.3.0")
	if err == nil {
		t.Fatal("expected error for duplicate runtime+target, got nil")
	}
}

func TestUpdateStoreCreateBackwardCompat(t *testing.T) {
	t.Parallel()

	store := NewUpdateStore()

	// The old Create() method should default to "multica" target.
	req, err := store.Create("runtime-1", "0.2.0")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if req.Target != "multica" {
		t.Fatalf("expected default target 'multica', got %q", req.Target)
	}
}

func TestUpdateStorePopPendingReturnsTarget(t *testing.T) {
	t.Parallel()

	store := NewUpdateStore()

	_, _ = store.CreateWithTarget("runtime-1", "claude", "2.1.0")

	popped := store.PopPending("runtime-1")
	if popped == nil {
		t.Fatal("expected a pending update, got nil")
	}
	if popped.Target != "claude" {
		t.Fatalf("expected target 'claude', got %q", popped.Target)
	}
	if popped.Status != UpdateRunning {
		t.Fatalf("expected status running after pop, got %v", popped.Status)
	}
}

func TestUpdateStorePopAllPending(t *testing.T) {
	t.Parallel()

	store := NewUpdateStore()

	_, _ = store.CreateWithTarget("runtime-1", "multica", "0.2.0")
	_, _ = store.CreateWithTarget("runtime-1", "claude", "2.1.0")
	_, _ = store.CreateWithTarget("runtime-1", "codex", "0.5.0")

	// Pop all pending for runtime-1.
	popped := store.PopAllPending("runtime-1")
	if len(popped) != 3 {
		t.Fatalf("expected 3 pending updates, got %d", len(popped))
	}

	// All should now be running.
	for _, p := range popped {
		if p.Status != UpdateRunning {
			t.Fatalf("expected status running, got %v for target %s", p.Status, p.Target)
		}
	}

	// No more pending after popping all.
	remaining := store.PopAllPending("runtime-1")
	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining after PopAllPending, got %d", len(remaining))
	}
}

func TestUpdateStorePopPendingDifferentRuntime(t *testing.T) {
	t.Parallel()

	store := NewUpdateStore()

	_, _ = store.CreateWithTarget("runtime-1", "multica", "0.2.0")
	_, _ = store.CreateWithTarget("runtime-2", "multica", "0.2.0")

	// Pop for runtime-2 should not affect runtime-1.
	popped := store.PopPending("runtime-2")
	if popped == nil {
		t.Fatal("expected update for runtime-2")
	}
	if popped.RuntimeID != "runtime-2" {
		t.Fatalf("expected runtime-2, got %s", popped.RuntimeID)
	}

	// runtime-1 should still have a pending update.
	popped = store.PopPending("runtime-1")
	if popped == nil {
		t.Fatal("expected update for runtime-1 to still be pending")
	}
}
