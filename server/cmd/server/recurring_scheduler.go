package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nullne/multica/server/internal/cron"
	"github.com/nullne/multica/server/internal/events"
	"github.com/nullne/multica/server/internal/service"
	"github.com/nullne/multica/server/internal/util"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

const (
	schedulerInterval = 60 * time.Second
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
	claimed, err := queries.ClaimRecurringTemplate(ctx, tmpl.ID, tmpl.NextRunAt, newNextRunAt)
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

	slog.Info("recurring scheduler: issue created", "issue_id", util.UUIDToString(issue.ID), "template_id", util.UUIDToString(claimed.ID))

	bus.Publish(events.Event{
		Type:        protocol.EventIssueCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		Payload: map[string]any{
			"issue_id":    util.UUIDToString(issue.ID),
			"template_id": util.UUIDToString(claimed.ID),
			"number":      issue.Number,
		},
	})

	// Enqueue task when the issue is assigned to an agent.
	if issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" && issue.AssigneeID.Valid {
		if _, err := taskSvc.EnqueueTaskForIssue(ctx, issue); err != nil {
			slog.Warn("recurring scheduler: failed to enqueue task", "issue_id", util.UUIDToString(issue.ID), "error", err)
		}
	}
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
