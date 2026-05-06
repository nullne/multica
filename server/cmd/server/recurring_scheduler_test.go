package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// TestSchedulerIdempotency verifies that a template claimed by one instance
// (next_run_at advanced) is skipped by a concurrent instance via ClaimRecurringTemplate.
// This test exercises the optimistic-locking logic without a database by simulating
// the claim check: a second claim with a stale next_run_at must return no rows.
func TestSchedulerIdempotency(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()

	// Insert a due template.
	var templateID string
	oldNextRunAt := time.Now().Add(-time.Minute) // in the past = due
	err := testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			next_run_at, created_by_id, created_by_type
		)
		SELECT $1, 'Idempotency test', 'medium', '* * * * *', 'UTC', TRUE, $2,
		       m.user_id, 'member'
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID, oldNextRunAt).Scan(&templateID)
	if err != nil {
		t.Fatalf("failed to insert test template: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, templateID)
	})

	queries := db.New(testPool)

	newNextRunAt := pgtype.Timestamptz{Time: time.Now().Add(time.Minute), Valid: true}
	oldTS := pgtype.Timestamptz{Time: oldNextRunAt, Valid: true}
	id := parseUUID(templateID)

	// First claim: should succeed.
	_, err = queries.ClaimRecurringTemplate(ctx, id, oldTS, newNextRunAt)
	if err != nil {
		t.Fatalf("first ClaimRecurringTemplate failed unexpectedly: %v", err)
	}

	// Second claim with same old next_run_at: should fail (no rows).
	_, err = queries.ClaimRecurringTemplate(ctx, id, oldTS, newNextRunAt)
	if err == nil {
		t.Fatal("second ClaimRecurringTemplate should have returned no rows, but succeeded")
	}
}

// TestSchedulerDisabledTemplateSkipped verifies that disabled templates are never
// returned by ListDueRecurringTemplates.
func TestSchedulerDisabledTemplateSkipped(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()

	// Insert a DISABLED template with a due next_run_at.
	var templateID string
	err := testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			next_run_at, created_by_id, created_by_type
		)
		SELECT $1, 'Disabled test', 'medium', '* * * * *', 'UTC', FALSE,
		       now() - interval '1 minute',
		       m.user_id, 'member'
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID).Scan(&templateID)
	if err != nil {
		t.Fatalf("failed to insert disabled template: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, templateID)
	})

	queries := db.New(testPool)
	due, err := queries.ListDueRecurringTemplates(ctx, 100)
	if err != nil {
		t.Fatalf("ListDueRecurringTemplates failed: %v", err)
	}

	for _, tmpl := range due {
		if parseUUIDBytes(templateID) == tmpl.ID.Bytes {
			t.Fatal("disabled template should not appear in due list")
		}
	}
}

// TestSchedulerTimezoneHandling verifies that a template with a non-UTC timezone
// correctly parses and stores its schedule.
func TestSchedulerTimezoneHandling(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()

	// Use New York timezone — this should parse correctly.
	var templateID string
	err := testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			next_run_at, created_by_id, created_by_type
		)
		SELECT $1, 'Timezone test', 'medium', '0 9 * * *', 'America/New_York', TRUE,
		       now() + interval '1 day',
		       m.user_id, 'member'
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID).Scan(&templateID)
	if err != nil {
		t.Fatalf("failed to insert timezone template: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, templateID)
	})

	queries := db.New(testPool)
	tmpl, err := queries.GetRecurringTemplateInWorkspace(ctx, parseUUID(templateID), parseUUID(testWorkspaceID))
	if err != nil {
		t.Fatalf("GetRecurringTemplateInWorkspace failed: %v", err)
	}

	if tmpl.Timezone != "America/New_York" {
		t.Errorf("expected timezone 'America/New_York', got %q", tmpl.Timezone)
	}
	if tmpl.Schedule != "0 9 * * *" {
		t.Errorf("expected schedule '0 9 * * *', got %q", tmpl.Schedule)
	}
	if !tmpl.Enabled {
		t.Error("template should be enabled")
	}
	// Template is due in the future — should NOT appear in due list.
	due, err := queries.ListDueRecurringTemplates(ctx, 100)
	if err != nil {
		t.Fatalf("ListDueRecurringTemplates failed: %v", err)
	}
	for _, d := range due {
		if d.ID.Bytes == parseUUIDBytes(templateID) {
			t.Fatal("future template should not appear in due list")
		}
	}
}
