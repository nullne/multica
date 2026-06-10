package main

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/nullne/multica/server/internal/events"
	"github.com/nullne/multica/server/internal/service"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// createDeferredIssue creates a todo issue assigned to the agent with a
// future dispatch_after, returning the issue ID.
func createDeferredIssue(t *testing.T, title, agentID string, dispatchAfter time.Time) string {
	t.Helper()
	resp := authRequest(t, "POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":          title,
		"status":         "todo",
		"assignee_type":  "agent",
		"assignee_id":    agentID,
		"dispatch_after": dispatchAfter.Format(time.RFC3339),
	})
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("create deferred issue: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var issue map[string]any
	readJSON(t, resp, &issue)
	return issue["id"].(string)
}

func deleteIssue(t *testing.T, issueID string) {
	t.Helper()
	clearTasks(t, issueID)
	resp := authRequest(t, "DELETE", "/api/issues/"+issueID, nil)
	resp.Body.Close()
}

// backdateDispatchAfter moves an issue's dispatch_after into the past so the
// scheduler considers it due without waiting in the test.
func backdateDispatchAfter(t *testing.T, issueID string) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(),
		`UPDATE issue SET dispatch_after = now() - interval '1 minute' WHERE id = $1`, issueID); err != nil {
		t.Fatalf("backdate dispatch_after: %v", err)
	}
}

func dispatchFiredAt(t *testing.T, issueID string) *time.Time {
	t.Helper()
	var firedAt *time.Time
	if err := testPool.QueryRow(context.Background(),
		`SELECT dispatch_after_fired_at FROM issue WHERE id = $1`, issueID).Scan(&firedAt); err != nil {
		t.Fatalf("query dispatch_after_fired_at: %v", err)
	}
	return firedAt
}

// TestIssueDispatchAfterDefersAssignment verifies that assigning an agent
// with a future dispatch_after does not enqueue a task inline, and that the
// scheduler enqueues it (exactly once) when the time arrives.
func TestIssueDispatchAfterDefersAssignment(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}
	ctx := context.Background()
	queries := db.New(testPool)
	taskSvc := service.NewTaskService(queries, nil, events.New())

	agentID := getAgentID(t)
	issueID := createDeferredIssue(t, "Dispatch after deferral test", agentID, time.Now().Add(time.Hour))
	t.Cleanup(func() { deleteIssue(t, issueID) })

	// Assignment with a future dispatch_after must not enqueue inline.
	if n := countPendingTasks(t, issueID); n != 0 {
		t.Fatalf("expected 0 pending tasks for deferred assignment, got %d", n)
	}

	// Not due yet — scheduler must not fire.
	fireDueIssueDispatches(ctx, queries, taskSvc)
	if n := countPendingTasks(t, issueID); n != 0 {
		t.Fatalf("expected 0 pending tasks before dispatch_after, got %d", n)
	}

	// Due — scheduler fires and enqueues exactly one task.
	backdateDispatchAfter(t, issueID)
	fireDueIssueDispatches(ctx, queries, taskSvc)
	if n := countPendingTasks(t, issueID); n != 1 {
		t.Fatalf("expected 1 pending task after dispatch_after, got %d", n)
	}
	if dispatchFiredAt(t, issueID) == nil {
		t.Fatal("expected dispatch_after_fired_at to be set after firing")
	}

	// Firing again must not duplicate (one-shot CAS).
	fireDueIssueDispatches(ctx, queries, taskSvc)
	if n := countPendingTasks(t, issueID); n != 1 {
		t.Fatalf("expected 1 pending task after second fire, got %d", n)
	}
}

// TestIssueDispatchAfterClearedEnqueuesImmediately verifies that removing a
// pending dispatch_after schedule dispatches the deferred assignment inline.
func TestIssueDispatchAfterClearedEnqueuesImmediately(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	agentID := getAgentID(t)
	issueID := createDeferredIssue(t, "Dispatch after clear test", agentID, time.Now().Add(time.Hour))
	t.Cleanup(func() { deleteIssue(t, issueID) })

	if n := countPendingTasks(t, issueID); n != 0 {
		t.Fatalf("expected 0 pending tasks for deferred assignment, got %d", n)
	}

	// Explicit null clears the schedule and dispatches now.
	resp := authRequest(t, "PUT", "/api/issues/"+issueID, map[string]any{
		"dispatch_after": nil,
	})
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("clear dispatch_after: expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	if n := countPendingTasks(t, issueID); n != 1 {
		t.Fatalf("expected 1 pending task after clearing dispatch_after, got %d", n)
	}
}

// TestIssueDispatchAfterRescheduleResetsFiredMarker verifies that changing
// dispatch_after re-arms the one-shot schedule.
func TestIssueDispatchAfterRescheduleResetsFiredMarker(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}
	ctx := context.Background()
	queries := db.New(testPool)
	taskSvc := service.NewTaskService(queries, nil, events.New())

	agentID := getAgentID(t)
	issueID := createDeferredIssue(t, "Dispatch after reschedule test", agentID, time.Now().Add(time.Hour))
	t.Cleanup(func() { deleteIssue(t, issueID) })

	// Fire the first schedule.
	backdateDispatchAfter(t, issueID)
	fireDueIssueDispatches(ctx, queries, taskSvc)
	if n := countPendingTasks(t, issueID); n != 1 {
		t.Fatalf("expected 1 pending task after first fire, got %d", n)
	}
	if dispatchFiredAt(t, issueID) == nil {
		t.Fatal("expected fired marker after first fire")
	}

	// Rescheduling to a new future time must reset the fired marker.
	resp := authRequest(t, "PUT", "/api/issues/"+issueID, map[string]any{
		"dispatch_after": time.Now().Add(2 * time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("reschedule dispatch_after: expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	if firedAt := dispatchFiredAt(t, issueID); firedAt != nil {
		t.Fatalf("expected fired marker to reset after reschedule, got %v", firedAt)
	}

	// The rescheduled time fires again once due.
	clearTasks(t, issueID)
	backdateDispatchAfter(t, issueID)
	fireDueIssueDispatches(ctx, queries, taskSvc)
	if n := countPendingTasks(t, issueID); n != 1 {
		t.Fatalf("expected 1 pending task after rescheduled fire, got %d", n)
	}
}

// TestIssueDispatchAfterSkipsTerminalStatus verifies the scheduler ignores
// done/cancelled issues even when their dispatch_after is due.
func TestIssueDispatchAfterSkipsTerminalStatus(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}
	ctx := context.Background()
	queries := db.New(testPool)
	taskSvc := service.NewTaskService(queries, nil, events.New())

	agentID := getAgentID(t)
	issueID := createDeferredIssue(t, "Dispatch after terminal status test", agentID, time.Now().Add(time.Hour))
	t.Cleanup(func() { deleteIssue(t, issueID) })

	resp := authRequest(t, "PUT", "/api/issues/"+issueID, map[string]any{"status": "done"})
	resp.Body.Close()

	backdateDispatchAfter(t, issueID)
	fireDueIssueDispatches(ctx, queries, taskSvc)
	if n := countPendingTasks(t, issueID); n != 0 {
		t.Fatalf("expected 0 pending tasks for done issue, got %d", n)
	}
	if firedAt := dispatchFiredAt(t, issueID); firedAt != nil {
		t.Fatalf("expected fired marker to remain unset for done issue, got %v", firedAt)
	}
}

// TestIssueDispatchAfterRetriesWhenEnqueueFails verifies that a failed
// enqueue (no matching runtime) releases the claim so the next tick retries.
func TestIssueDispatchAfterRetriesWhenEnqueueFails(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}
	ctx := context.Background()
	queries := db.New(testPool)
	taskSvc := service.NewTaskService(queries, nil, events.New())

	agentID := getAgentID(t)
	issueID := createDeferredIssue(t, "Dispatch after retry test", agentID, time.Now().Add(time.Hour))
	t.Cleanup(func() { deleteIssue(t, issueID) })

	// Take all runtimes offline so enqueue fails with "no online runtime".
	if _, err := testPool.Exec(ctx,
		`UPDATE agent_runtime SET status = 'offline' WHERE workspace_id = $1`, testWorkspaceID); err != nil {
		t.Fatalf("take runtimes offline: %v", err)
	}
	restoreRuntimes := func() {
		if _, err := testPool.Exec(ctx,
			`UPDATE agent_runtime SET status = 'online' WHERE workspace_id = $1`, testWorkspaceID); err != nil {
			t.Fatalf("restore runtimes: %v", err)
		}
	}
	t.Cleanup(restoreRuntimes)

	backdateDispatchAfter(t, issueID)
	fireDueIssueDispatches(ctx, queries, taskSvc)
	if n := countPendingTasks(t, issueID); n != 0 {
		t.Fatalf("expected 0 pending tasks when enqueue fails, got %d", n)
	}
	if firedAt := dispatchFiredAt(t, issueID); firedAt != nil {
		t.Fatalf("expected claim released for retry, got fired_at %v", firedAt)
	}

	// Bring runtimes back — next tick succeeds.
	restoreRuntimes()
	fireDueIssueDispatches(ctx, queries, taskSvc)
	if n := countPendingTasks(t, issueID); n != 1 {
		t.Fatalf("expected 1 pending task after retry, got %d", n)
	}
}
