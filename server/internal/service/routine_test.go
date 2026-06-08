package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

func TestRoutineServiceCommentIssueCreatesCommentOnLinkedIssue(t *testing.T) {
	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("database not available: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	queries := db.New(pool)

	suffix := fmt.Sprintf("routine-service-%d", os.Getpid())
	var userID, botID, workspaceID, issueID, routineID, triggerID, actionID string
	if err := pool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('Routine User', $1) RETURNING id`, suffix+"@example.com").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, userID) })
	if err := pool.QueryRow(ctx, `INSERT INTO "user" (name, email, kind) VALUES ('Routine Bot', $1, 'bot') RETURNING id`, "bot-"+suffix+"@example.com").Scan(&botID); err != nil {
		t.Fatalf("insert bot: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO workspace (name, slug, issue_prefix) VALUES ('Routine Service', $1, 'RSV') RETURNING id`, suffix).Scan(&workspaceID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, workspaceID) })
	if _, err := pool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, workspaceID, userID); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number)
		VALUES ($1, 'Linked PR issue', 'todo', 'medium', 'member', $2, 1)
		RETURNING id
	`, workspaceID, userID).Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO issue_link (issue_id, workspace_id, source_type, kind, direction, url, external_id)
		VALUES ($1, $2, 'github', 'pr', 'source', 'https://github.com/org/repo/pull/42', 'org/repo#42')
	`, issueID, workspaceID); err != nil {
		t.Fatalf("insert issue link: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO routine (workspace_id, name, priority, enabled, created_by_id, created_by_type)
		VALUES ($1, 'Routine Comment', 'medium', TRUE, $2, 'member')
		RETURNING id
	`, workspaceID, userID).Scan(&routineID); err != nil {
		t.Fatalf("insert routine: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO routine_trigger (routine_id, trigger_type, enabled)
		VALUES ($1, 'github', TRUE)
		RETURNING id
	`, routineID).Scan(&triggerID); err != nil {
		t.Fatalf("insert trigger: %v", err)
	}
	cfg, _ := json.Marshal(RoutineCommentIssueConfig{
		BotUserID:       botID,
		ContentTemplate: "CI result: {{.conclusion}}",
	})
	if err := pool.QueryRow(ctx, `
		INSERT INTO routine_action (routine_id, action_type, config, enabled, position)
		VALUES ($1, 'comment_issue', $2, TRUE, 0)
		RETURNING id
	`, routineID, cfg).Scan(&actionID); err != nil {
		t.Fatalf("insert action: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO routine_run (routine_id, trigger_id, action_id, event_type, dedup_key, payload, status, issue_id)
		VALUES ($1, $2, $3, 'github.pull_request.opened', 'routine-comment-linked-issue', '{}'::jsonb, 'processed', $4)
	`, routineID, triggerID, actionID, issueID); err != nil {
		t.Fatalf("insert routine run: %v", err)
	}

	routine, err := queries.GetRoutine(ctx, parseUUID(routineID))
	if err != nil {
		t.Fatalf("GetRoutine: %v", err)
	}
	trigger, err := queries.GetRoutineTrigger(ctx, parseUUID(triggerID))
	if err != nil {
		t.Fatalf("GetRoutineTrigger: %v", err)
	}
	action, err := queries.GetRoutineAction(ctx, parseUUID(actionID))
	if err != nil {
		t.Fatalf("GetRoutineAction: %v", err)
	}

	svc := NewRoutineService(queries, pool, nil)
	result, err := svc.ExecuteAction(ctx, routine, trigger, action, RoutineEvent{
		Type:     "github.check_run.completed",
		DedupKey: "routine-comment-test",
		Data: map[string]string{
			"source_url": "https://github.com/org/repo/pull/42",
			"conclusion": "failure",
		},
		Payload: []byte(`{"check":"ci"}`),
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if !result.Ran || !result.CommentID.Valid {
		t.Fatalf("expected comment action to run, got %+v", result)
	}
	var content string
	if err := pool.QueryRow(ctx, `SELECT content FROM comment WHERE id = $1`, result.CommentID).Scan(&content); err != nil {
		t.Fatalf("query comment: %v", err)
	}
	if content != "CI result: failure" {
		t.Fatalf("comment content = %q", content)
	}
}

func parseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}
