package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nullne/multica/server/internal/cron"
	"github.com/nullne/multica/server/internal/events"
	"github.com/nullne/multica/server/internal/handler"
	"github.com/nullne/multica/server/internal/service"
	"github.com/nullne/multica/server/internal/util"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

const (
	schedulerInterval  = 60 * time.Second
	schedulerBatchSize = int32(50)
)

// runRecurringScheduler ticks every minute and fires any due recurring issue templates.
func runRecurringScheduler(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, taskSvc *service.TaskService, bus *events.Bus) {
	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()

	// Run once immediately on startup to catch any templates that were due
	// while the server was down.
	fireRecurringTemplates(ctx, pool, queries, taskSvc, bus)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fireRecurringTemplates(ctx, pool, queries, taskSvc, bus)
		}
	}
}

// fireRecurringTemplates processes all currently-due templates.
func fireRecurringTemplates(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, taskSvc *service.TaskService, bus *events.Bus) {
	due, err := queries.ListDueRecurringTemplates(ctx, schedulerBatchSize)
	if err != nil {
		slog.Warn("recurring scheduler: list due templates failed", "error", err)
		return
	}
	for _, tmpl := range due {
		fireTemplate(ctx, pool, queries, taskSvc, bus, tmpl)
	}
}

// fireTemplate claims a single template (optimistic locking) and creates the issue.
func fireTemplate(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, taskSvc *service.TaskService, bus *events.Bus, tmpl db.RecurringIssueTemplate) {
	loc, err := time.LoadLocation(tmpl.Timezone)
	if err != nil {
		slog.Warn("recurring scheduler: invalid timezone", "template_id", util.UUIDToString(tmpl.ID), "timezone", tmpl.Timezone, "error", err)
		loc = time.UTC
	}

	sched, err := cron.Parse(tmpl.Schedule)
	if err != nil {
		slog.Warn("recurring scheduler: invalid schedule", "template_id", util.UUIDToString(tmpl.ID), "schedule", tmpl.Schedule, "error", err)
		return
	}

	// Compute next run time from NOW (not from previous next_run_at) to avoid
	// drift when the server was down for multiple periods.
	newNextRunAt := pgtype.Timestamptz{}
	if next := sched.Next(time.Now().In(loc)); !next.IsZero() {
		newNextRunAt = pgtype.Timestamptz{Time: next, Valid: true}
	}

	// Atomically claim the template. If another server instance already processed
	// this occurrence, the UPDATE returns no rows and we skip it.
	claimed, err := queries.ClaimRecurringTemplate(ctx, db.ClaimRecurringTemplateParams{
		ID:          tmpl.ID,
		NextRunAt:   tmpl.NextRunAt,
		NextRunAt_2: newNextRunAt,
	})
	if err != nil {
		// ErrNoRows means another instance claimed it first — not an error.
		slog.Debug("recurring scheduler: template already claimed or not due", "template_id", util.UUIDToString(tmpl.ID))
		return
	}

	slog.Info("recurring scheduler: firing template", "template_id", util.UUIDToString(claimed.ID), "title", claimed.Title)

	issue, err := createIssueFromTemplate(ctx, pool, queries, claimed)
	if err != nil {
		slog.Warn("recurring scheduler: failed to create issue", "template_id", util.UUIDToString(claimed.ID), "error", err)
		return
	}

	// Increment the successful run count only after the issue is committed —
	// matching "completion based on successful issue creation, not merely a
	// scheduler claim attempt." When the count reaches max_runs, the query
	// also clears next_run_at so the template won't be picked up again.
	if _, err := queries.IncrementRecurringTemplateSuccess(ctx, claimed.ID); err != nil {
		slog.Warn("recurring scheduler: failed to record successful run", "template_id", util.UUIDToString(claimed.ID), "error", err)
	}

	slog.Info("recurring scheduler: issue created", "issue_id", util.UUIDToString(issue.ID), "template_id", util.UUIDToString(claimed.ID))

	// Copy template member subscribers onto the new issue so they receive
	// notifications matching admin intent. Reason "manual" — the listener
	// reasons (creator/assignee/mentioned) describe automatic propagation
	// rules, while template subscribers were chosen explicitly by an admin.
	templateSubs, err := queries.ListRecurringTemplateSubscribers(ctx, claimed.ID)
	if err != nil {
		slog.Warn("recurring scheduler: list template subscribers failed", "template_id", util.UUIDToString(claimed.ID), "error", err)
	}
	for _, sub := range templateSubs {
		if err := queries.AddIssueSubscriber(ctx, db.AddIssueSubscriberParams{
			IssueID:  issue.ID,
			UserType: "member",
			UserID:   sub.UserID,
			Reason:   "manual",
		}); err != nil {
			slog.Warn("recurring scheduler: add issue subscriber failed", "issue_id", util.UUIDToString(issue.ID), "user_id", util.UUIDToString(sub.UserID), "error", err)
		}
	}

	// Publish using the standard IssueResponse payload shape so existing
	// subscriber and notification listeners behave consistently with normal
	// issue creation (auto-subscribe creator/assignee/mentioned, fan out
	// inbox + Telegram notifications).
	prefix := getIssuePrefix(ctx, queries, issue.WorkspaceID)
	resp := handler.IssueToResponse(issue, prefix)
	bus.Publish(events.Event{
		Type:        protocol.EventIssueCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		Payload: map[string]any{
			"issue":       resp,
			"template_id": util.UUIDToString(claimed.ID),
		},
	})

	// Re-publish subscriber:added events so connected clients see the
	// template-driven subscriptions in real time, mirroring how the
	// auto-subscribe listener publishes for creator/assignee/mentioned.
	for _, sub := range templateSubs {
		bus.Publish(events.Event{
			Type:        protocol.EventSubscriberAdded,
			WorkspaceID: util.UUIDToString(issue.WorkspaceID),
			Payload: map[string]any{
				"issue_id":  util.UUIDToString(issue.ID),
				"user_type": "member",
				"user_id":   util.UUIDToString(sub.UserID),
				"reason":    "manual",
			},
		})
	}

	// Enqueue task when the issue is assigned to an agent.
	if issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" && issue.AssigneeID.Valid {
		if _, err := taskSvc.EnqueueTaskForIssue(ctx, issue); err != nil {
			slog.Warn("recurring scheduler: failed to enqueue task", "issue_id", util.UUIDToString(issue.ID), "error", err)
		}
	}
}

// getIssuePrefix returns the issue prefix for a workspace, falling back to
// the empty string when the lookup fails (matching how other call sites
// degrade silently for the prefix).
func getIssuePrefix(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID) string {
	ws, err := queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return ""
	}
	return ws.IssuePrefix
}

// createIssueFromTemplate opens a transaction, increments the workspace issue counter,
// and inserts the new issue.
func createIssueFromTemplate(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, tmpl db.RecurringIssueTemplate) (db.Issue, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return db.Issue{}, err
	}
	defer tx.Rollback(ctx)

	qtx := queries.WithTx(tx)

	number, err := qtx.IncrementIssueCounter(ctx, tmpl.WorkspaceID)
	if err != nil {
		return db.Issue{}, err
	}

	var dueDate pgtype.Timestamptz
	if tmpl.DueDateOffsetHours.Valid {
		offset := time.Duration(tmpl.DueDateOffsetHours.Int32) * time.Hour
		dueDate = pgtype.Timestamptz{Time: time.Now().Add(offset), Valid: true}
	}

	issue, err := qtx.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID:           tmpl.WorkspaceID,
		Title:                 tmpl.Title,
		Description:           tmpl.Description,
		Status:                "todo",
		Priority:              tmpl.Priority,
		AssigneeType:          tmpl.AssigneeType,
		AssigneeID:            tmpl.AssigneeID,
		CreatorType:           tmpl.CreatedByType,
		CreatorID:             tmpl.CreatedByID,
		VerifierAgentID:       pgtype.UUID{},
		ParentIssueID:         pgtype.UUID{},
		Position:              0,
		DueDate:               dueDate,
		Number:                number,
		MaxVerificationRounds: pgtype.Int4{},
		DispatchProvider:      tmpl.DispatchProvider,
		DispatchDaemonID:      tmpl.DispatchDaemonID,
		DispatchDaemonLabel:   tmpl.DispatchDaemonLabel,
	})
	if err != nil {
		return db.Issue{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Issue{}, err
	}
	return issue, nil
}
