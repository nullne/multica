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

func TestRoutineSchedulerRunAtFiresOnceAndMaxRunsStopsCron(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()
	queries := db.New(testPool)

	var userID string
	if err := testPool.QueryRow(ctx, `SELECT user_id::text FROM member WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&userID); err != nil {
		t.Fatalf("find member: %v", err)
	}
	var routineID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO routine (
			workspace_id, name, instructions, priority,
			enabled, created_by_id, created_by_type
		)
		VALUES ($1, 'Routine one-shot scheduler', 'Scheduled once', 'medium', TRUE, $2, 'member')
		RETURNING id
	`, testWorkspaceID, userID).Scan(&routineID); err != nil {
		t.Fatalf("insert routine: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM routine WHERE id = $1`, routineID) })

	var onceTriggerID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO routine_trigger (
			routine_id, trigger_type, timezone, run_at,
			next_run_at, max_runs, enabled
		)
		VALUES ($1, 'schedule', 'UTC', $2, $2, 1, TRUE)
		RETURNING id
	`, routineID, time.Now().Add(-time.Minute)).Scan(&onceTriggerID); err != nil {
		t.Fatalf("insert one-shot trigger: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO routine_action (routine_id, action_type, config, enabled, position)
		VALUES ($1, 'create_issue', '{}'::jsonb, TRUE, 0)
	`, routineID); err != nil {
		t.Fatalf("insert routine action: %v", err)
	}

	taskSvc := service.NewTaskService(queries, nil, events.New())
	fireRoutineScheduleTriggers(ctx, testPool, queries, taskSvc)
	fireRoutineScheduleTriggers(ctx, testPool, queries, taskSvc)

	var successCount int
	var nextRunAt *time.Time
	if err := testPool.QueryRow(ctx, `
		SELECT successful_runs_count, next_run_at FROM routine_trigger WHERE id = $1
	`, onceTriggerID).Scan(&successCount, &nextRunAt); err != nil {
		t.Fatalf("query one-shot trigger: %v", err)
	}
	if successCount != 1 {
		t.Fatalf("one-shot successful_runs_count = %d, want 1", successCount)
	}
	if nextRunAt != nil {
		t.Fatalf("one-shot next_run_at = %v, want NULL after max run", nextRunAt)
	}

	var issueCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM issue
		WHERE workspace_id = $1 AND title = 'Routine one-shot scheduler'
	`, testWorkspaceID).Scan(&issueCount); err != nil {
		t.Fatalf("count one-shot issues: %v", err)
	}
	if issueCount != 1 {
		t.Fatalf("one-shot issue count = %d, want 1", issueCount)
	}
}

func TestRoutineSchedulerTimezoneAffectsNextRun(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}
	now := time.Now().In(loc)
	nextHour := now.Add(time.Hour)
	cronExpr := time.Date(nextHour.Year(), nextHour.Month(), nextHour.Day(), nextHour.Hour(), 0, 0, 0, loc)
	if !cronExpr.After(now) {
		cronExpr = cronExpr.Add(time.Hour)
	}
	schedule := cronExpr.Format("4 15 * * *")

	ctx := context.Background()
	queries := db.New(testPool)
	var userID string
	if err := testPool.QueryRow(ctx, `SELECT user_id::text FROM member WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&userID); err != nil {
		t.Fatalf("find member: %v", err)
	}
	var routineID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO routine (
			workspace_id, name, instructions, priority,
			enabled, created_by_id, created_by_type
		)
		VALUES ($1, 'Routine timezone scheduler', 'Timezone body', 'medium', TRUE, $2, 'member')
		RETURNING id
	`, testWorkspaceID, userID).Scan(&routineID); err != nil {
		t.Fatalf("insert routine: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DELETE FROM routine WHERE id = $1`, routineID) })

	var triggerID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO routine_trigger (
			routine_id, trigger_type, schedule, timezone,
			next_run_at, enabled
		)
		VALUES ($1, 'schedule', $2, 'Asia/Tokyo', $3, TRUE)
		RETURNING id
	`, routineID, schedule, time.Now().Add(-time.Minute)).Scan(&triggerID); err != nil {
		t.Fatalf("insert trigger: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO routine_action (routine_id, action_type, config, enabled, position)
		VALUES ($1, 'create_issue', '{}'::jsonb, TRUE, 0)
	`, routineID); err != nil {
		t.Fatalf("insert routine action: %v", err)
	}

	taskSvc := service.NewTaskService(queries, nil, events.New())
	fireRoutineScheduleTriggers(ctx, testPool, queries, taskSvc)

	var nextRunAt time.Time
	if err := testPool.QueryRow(ctx, `SELECT next_run_at FROM routine_trigger WHERE id = $1`, triggerID).Scan(&nextRunAt); err != nil {
		t.Fatalf("query next_run_at: %v", err)
	}
	got := nextRunAt.In(loc)
	if got.Hour() != cronExpr.Hour() || got.Minute() != cronExpr.Minute() {
		t.Fatalf("next_run_at in Asia/Tokyo = %s, want hour/minute %02d:%02d", got, cronExpr.Hour(), cronExpr.Minute())
	}
}
