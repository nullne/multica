package main

import (
	"context"
	"testing"
	"time"

	"github.com/nullne/multica/server/internal/events"
	"github.com/nullne/multica/server/internal/service"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

func TestRoutineSchedulerCreatesIssueAndRecordsSuccess(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()
	queries := db.New(testPool)

	var routineID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO routine (
			workspace_id, name, instructions, priority,
			enabled, created_by_id, created_by_type
		)
		SELECT $1, 'Routine scheduler test', 'Scheduled body', 'medium',
		       TRUE, m.user_id, 'member'
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID).Scan(&routineID); err != nil {
		t.Fatalf("insert routine: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM routine WHERE id = $1`, routineID) })

	var triggerID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO routine_trigger (
			routine_id, trigger_type, schedule, timezone,
			next_run_at, enabled
		)
		VALUES ($1, 'schedule', '* * * * *', 'UTC', $2, TRUE)
		RETURNING id
	`, routineID, time.Now().Add(-time.Minute)).Scan(&triggerID); err != nil {
		t.Fatalf("insert routine trigger: %v", err)
	}

	if _, err := testPool.Exec(ctx, `
		INSERT INTO routine_action (routine_id, action_type, config, enabled, position)
		VALUES ($1, 'create_issue', '{}'::jsonb, TRUE, 0)
	`, routineID); err != nil {
		t.Fatalf("insert routine action: %v", err)
	}

	taskSvc := service.NewTaskService(queries, nil, events.New())
	fireRoutineScheduleTriggers(ctx, testPool, queries, taskSvc)

	var issueID string
	if err := testPool.QueryRow(ctx, `
		SELECT id::text FROM issue
		WHERE workspace_id = $1 AND title = 'Routine scheduler test'
		ORDER BY created_at DESC LIMIT 1
	`, testWorkspaceID).Scan(&issueID); err != nil {
		t.Fatalf("query routine-created issue: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID) })

	var successCount int
	if err := testPool.QueryRow(ctx, `
		SELECT successful_runs_count FROM routine_trigger WHERE id = $1
	`, triggerID).Scan(&successCount); err != nil {
		t.Fatalf("query success count: %v", err)
	}
	if successCount != 1 {
		t.Fatalf("successful_runs_count = %d, want 1", successCount)
	}
}
