package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/events"
	"github.com/nullne/multica/server/internal/service"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// TestTemplateSubscriberPropagation seeds a template with member subscribers
// plus a dispatch provider/daemon, fires the scheduler, and verifies that the
// generated issue inherits both. Covers the requirement that scheduled issues
// pick up the template's subscribers and agent dispatch settings.
func TestTemplateSubscriberPropagation(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()
	queries := db.New(testPool)

	// Insert an agent so the template can be assigned to one.
	var agentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, runtime_mode, providers)
		VALUES ($1, 'Subscriber test agent', 'cloud', ARRAY['claude'])
		RETURNING id
	`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM agent WHERE id = $1`, agentID) })

	// Insert a daemon so we can assert the dispatch settings round-trip.
	var daemonID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO daemon (workspace_id, daemon_id, device_name, status)
		VALUES ($1, $2, 'Test Device', 'online')
		RETURNING id
	`, testWorkspaceID, "daemon-"+t.Name()).Scan(&daemonID); err != nil {
		t.Fatalf("insert daemon: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM daemon WHERE id = $1`, daemonID) })

	// Get a member to use as subscriber.
	var subscriberUserID string
	if err := testPool.QueryRow(ctx, `
		SELECT user_id::text FROM member WHERE workspace_id = $1 LIMIT 1
	`, testWorkspaceID).Scan(&subscriberUserID); err != nil {
		t.Fatalf("query member: %v", err)
	}

	// Insert template due now, assigned to the agent, with dispatch settings.
	oldNextRunAt := time.Now().Add(-time.Minute)
	var templateID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			next_run_at, created_by_id, created_by_type,
			assignee_type, assignee_id, dispatch_provider, dispatch_daemon_id
		)
		SELECT $1, 'Subscriber test', 'medium', '* * * * *', 'UTC', TRUE, $2,
		       m.user_id, 'member', 'agent', $3, 'claude', $4
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID, oldNextRunAt, agentID, daemonID).Scan(&templateID); err != nil {
		t.Fatalf("insert template: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, templateID)
	})

	// Add a subscriber row.
	if _, err := testPool.Exec(ctx, `
		INSERT INTO recurring_template_subscriber (template_id, user_id)
		VALUES ($1, $2)
	`, templateID, subscriberUserID); err != nil {
		t.Fatalf("insert template subscriber: %v", err)
	}

	// Sanity-check the subscribers query.
	subs, err := queries.ListRecurringTemplateSubscribers(ctx, parseUUID(templateID))
	if err != nil {
		t.Fatalf("ListRecurringTemplateSubscribers: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 template subscriber, got %d", len(subs))
	}

	// Trigger the scheduler.
	bus := events.New()
	taskSvc := service.NewTaskService(queries, nil, bus)
	fireRecurringTemplates(ctx, testPool, queries, taskSvc, bus)

	// Verify exactly one issue was created from this template — discover its id
	// by selecting on the template's title (unique to this test) and asserting
	// it inherited both the dispatch settings and the subscriber row.
	var issueID, dispatchProvider, dispatchDaemonID string
	if err := testPool.QueryRow(ctx, `
		SELECT id::text, COALESCE(dispatch_provider, ''), COALESCE(dispatch_daemon_id::text, '')
		FROM issue WHERE workspace_id = $1 AND title = 'Subscriber test'
		ORDER BY created_at DESC LIMIT 1
	`, testWorkspaceID).Scan(&issueID, &dispatchProvider, &dispatchDaemonID); err != nil {
		t.Fatalf("query generated issue: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID) })

	if dispatchProvider != "claude" {
		t.Errorf("expected dispatch_provider 'claude', got %q", dispatchProvider)
	}
	if dispatchDaemonID != daemonID {
		t.Errorf("expected dispatch_daemon_id %q, got %q", daemonID, dispatchDaemonID)
	}

	// Subscriber on the generated issue should match the template subscriber.
	var rowCount int
	if err := testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM issue_subscriber
		WHERE issue_id = $1 AND user_type = 'member' AND user_id = $2
	`, issueID, subscriberUserID).Scan(&rowCount); err != nil {
		t.Fatalf("query issue_subscriber: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected template subscriber to be propagated to issue, got %d rows", rowCount)
	}
}

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
	_, err = queries.ClaimRecurringTemplate(ctx, db.ClaimRecurringTemplateParams{
		ID:          id,
		NextRunAt:   oldTS,
		NextRunAt_2: newNextRunAt,
	})
	if err != nil {
		t.Fatalf("first ClaimRecurringTemplate failed unexpectedly: %v", err)
	}

	// Second claim with same old next_run_at: should fail (no rows).
	_, err = queries.ClaimRecurringTemplate(ctx, db.ClaimRecurringTemplateParams{
		ID:          id,
		NextRunAt:   oldTS,
		NextRunAt_2: newNextRunAt,
	})
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
	tmpl, err := queries.GetRecurringTemplateInWorkspace(ctx, db.GetRecurringTemplateInWorkspaceParams{
		ID:          parseUUID(templateID),
		WorkspaceID: parseUUID(testWorkspaceID),
	})
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

// TestSchedulerUnlimitedRuns verifies that a template with NULL max_runs is
// always considered eligible regardless of how many times it has fired.
func TestSchedulerUnlimitedRuns(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()

	var templateID string
	err := testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			next_run_at, created_by_id, created_by_type, successful_runs_count
		)
		SELECT $1, 'Unlimited test', 'medium', '* * * * *', 'UTC', TRUE,
		       now() - interval '1 minute',
		       m.user_id, 'member', 100
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID).Scan(&templateID)
	if err != nil {
		t.Fatalf("failed to insert template: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, templateID)
	})

	queries := db.New(testPool)
	due, err := queries.ListDueRecurringTemplates(ctx, 100)
	if err != nil {
		t.Fatalf("ListDueRecurringTemplates failed: %v", err)
	}

	found := false
	for _, d := range due {
		if d.ID.Bytes == parseUUIDBytes(templateID) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("unlimited template with high run count should still be due")
	}
}

// TestSchedulerCompletedTemplateSkipped verifies that a template whose
// successful_runs_count has reached max_runs is excluded from the due list and
// cannot be claimed.
func TestSchedulerCompletedTemplateSkipped(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()

	oldNextRunAt := time.Now().Add(-time.Minute)
	var templateID string
	err := testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			next_run_at, created_by_id, created_by_type, max_runs, successful_runs_count
		)
		SELECT $1, 'Completed test', 'medium', '* * * * *', 'UTC', TRUE, $2,
		       m.user_id, 'member', 1, 1
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID, oldNextRunAt).Scan(&templateID)
	if err != nil {
		t.Fatalf("failed to insert template: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, templateID)
	})

	queries := db.New(testPool)
	due, err := queries.ListDueRecurringTemplates(ctx, 100)
	if err != nil {
		t.Fatalf("ListDueRecurringTemplates failed: %v", err)
	}
	for _, d := range due {
		if d.ID.Bytes == parseUUIDBytes(templateID) {
			t.Fatal("completed template should not appear in due list")
		}
	}

	// Even an explicit claim must fail when max_runs has been reached.
	_, err = queries.ClaimRecurringTemplate(ctx, db.ClaimRecurringTemplateParams{
		ID:          parseUUID(templateID),
		NextRunAt:   pgtype.Timestamptz{Time: oldNextRunAt, Valid: true},
		NextRunAt_2: pgtype.Timestamptz{Time: time.Now().Add(time.Minute), Valid: true},
	})
	if err == nil {
		t.Fatal("ClaimRecurringTemplate should refuse a completed template")
	}
}

// TestSchedulerSingleRunCompletion verifies that incrementing the success count
// for a max_runs=1 template clears next_run_at, retiring the template from the
// due list. Combined with TestSchedulerCompletedTemplateSkipped this covers the
// "stop after N successful runs" requirement end to end.
func TestSchedulerSingleRunCompletion(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()

	oldNextRunAt := time.Now().Add(-time.Minute)
	var templateID string
	err := testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			next_run_at, created_by_id, created_by_type, max_runs
		)
		SELECT $1, 'Single-run test', 'medium', '* * * * *', 'UTC', TRUE, $2,
		       m.user_id, 'member', 1
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID, oldNextRunAt).Scan(&templateID)
	if err != nil {
		t.Fatalf("failed to insert template: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, templateID)
	})

	queries := db.New(testPool)
	id := parseUUID(templateID)

	// Claim should succeed for the first (and only) run.
	newNextRunAt := pgtype.Timestamptz{Time: time.Now().Add(time.Minute), Valid: true}
	if _, err := queries.ClaimRecurringTemplate(ctx, db.ClaimRecurringTemplateParams{
		ID:          id,
		NextRunAt:   pgtype.Timestamptz{Time: oldNextRunAt, Valid: true},
		NextRunAt_2: newNextRunAt,
	}); err != nil {
		t.Fatalf("first claim failed: %v", err)
	}

	// Record the successful run; this must clear next_run_at.
	updated, err := queries.IncrementRecurringTemplateSuccess(ctx, id)
	if err != nil {
		t.Fatalf("IncrementRecurringTemplateSuccess failed: %v", err)
	}
	if updated.SuccessfulRunsCount != 1 {
		t.Errorf("expected successful_runs_count=1, got %d", updated.SuccessfulRunsCount)
	}
	if updated.NextRunAt.Valid {
		t.Errorf("expected next_run_at to be NULL after final run, got %v", updated.NextRunAt.Time)
	}

	// Template should no longer be returned as due.
	due, err := queries.ListDueRecurringTemplates(ctx, 100)
	if err != nil {
		t.Fatalf("ListDueRecurringTemplates failed: %v", err)
	}
	for _, d := range due {
		if d.ID.Bytes == parseUUIDBytes(templateID) {
			t.Fatal("template that just finished its only run should not appear in due list")
		}
	}
}

// TestListActiveRecurringTemplatesFilters verifies that the active-only listing
// hides both disabled templates and templates that have reached their max_runs.
func TestListActiveRecurringTemplatesFilters(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	ctx := context.Background()

	// Active template: enabled, with available runs.
	var activeID, disabledID, completedID string
	err := testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			next_run_at, created_by_id, created_by_type
		)
		SELECT $1, 'Active', 'medium', '* * * * *', 'UTC', TRUE,
		       now() + interval '1 hour',
		       m.user_id, 'member'
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID).Scan(&activeID)
	if err != nil {
		t.Fatalf("insert active failed: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, activeID) })

	err = testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			created_by_id, created_by_type
		)
		SELECT $1, 'Disabled', 'medium', '* * * * *', 'UTC', FALSE,
		       m.user_id, 'member'
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID).Scan(&disabledID)
	if err != nil {
		t.Fatalf("insert disabled failed: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, disabledID) })

	err = testPool.QueryRow(ctx, `
		INSERT INTO recurring_issue_template (
			workspace_id, title, priority, schedule, timezone, enabled,
			created_by_id, created_by_type, max_runs, successful_runs_count
		)
		SELECT $1, 'Completed', 'medium', '* * * * *', 'UTC', TRUE,
		       m.user_id, 'member', 3, 3
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID).Scan(&completedID)
	if err != nil {
		t.Fatalf("insert completed failed: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM recurring_issue_template WHERE id = $1`, completedID) })

	queries := db.New(testPool)
	active, err := queries.ListActiveRecurringTemplates(ctx, parseUUID(testWorkspaceID))
	if err != nil {
		t.Fatalf("ListActiveRecurringTemplates failed: %v", err)
	}

	hasActive, hasDisabled, hasCompleted := false, false, false
	for _, t := range active {
		switch t.ID.Bytes {
		case parseUUIDBytes(activeID):
			hasActive = true
		case parseUUIDBytes(disabledID):
			hasDisabled = true
		case parseUUIDBytes(completedID):
			hasCompleted = true
		}
	}
	if !hasActive {
		t.Error("active template should be returned")
	}
	if hasDisabled {
		t.Error("disabled template should NOT be returned by active listing")
	}
	if hasCompleted {
		t.Error("completed template should NOT be returned by active listing")
	}

	// All three should be returned by the unfiltered listing.
	all, err := queries.ListRecurringTemplates(ctx, parseUUID(testWorkspaceID))
	if err != nil {
		t.Fatalf("ListRecurringTemplates failed: %v", err)
	}
	hasActive, hasDisabled, hasCompleted = false, false, false
	for _, t := range all {
		switch t.ID.Bytes {
		case parseUUIDBytes(activeID):
			hasActive = true
		case parseUUIDBytes(disabledID):
			hasDisabled = true
		case parseUUIDBytes(completedID):
			hasCompleted = true
		}
	}
	if !hasActive || !hasDisabled || !hasCompleted {
		t.Errorf("unfiltered listing should include all templates; got active=%v disabled=%v completed=%v",
			hasActive, hasDisabled, hasCompleted)
	}
}
