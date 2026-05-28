package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nullne/multica/server/internal/cron"
	"github.com/nullne/multica/server/internal/service"
	"github.com/nullne/multica/server/internal/util"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

const (
	routineSchedulerInterval  = 60 * time.Second
	routineSchedulerBatchSize = int32(50)
)

func runRoutineScheduler(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, taskSvc *service.TaskService) {
	ticker := time.NewTicker(routineSchedulerInterval)
	defer ticker.Stop()

	fireRoutineScheduleTriggers(ctx, pool, queries, taskSvc)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fireRoutineScheduleTriggers(ctx, pool, queries, taskSvc)
		}
	}
}

func fireRoutineScheduleTriggers(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, taskSvc *service.TaskService) {
	due, err := queries.ListDueRoutineScheduleTriggers(ctx, routineSchedulerBatchSize)
	if err != nil {
		slog.Warn("routine scheduler: list due triggers failed", "error", err)
		return
	}
	routineSvc := service.NewRoutineService(queries, pool, taskSvc)
	for _, trigger := range due {
		fireRoutineScheduleTrigger(ctx, queries, routineSvc, trigger)
	}
}

func fireRoutineScheduleTrigger(ctx context.Context, queries *db.Queries, routineSvc *service.RoutineService, trigger db.RoutineTrigger) {
	routine, err := queries.GetRoutine(ctx, trigger.RoutineID)
	if err != nil {
		slog.Warn("routine scheduler: load routine failed", "trigger_id", util.UUIDToString(trigger.ID), "error", err)
		return
	}
	loc, err := time.LoadLocation(trigger.Timezone)
	if err != nil {
		slog.Warn("routine scheduler: invalid timezone", "trigger_id", util.UUIDToString(trigger.ID), "timezone", trigger.Timezone, "error", err)
		loc = time.UTC
	}

	var newNextRunAt pgtype.Timestamptz
	if trigger.Schedule.Valid && trigger.Schedule.String != "" {
		sched, err := cron.Parse(trigger.Schedule.String)
		if err != nil {
			slog.Warn("routine scheduler: invalid schedule", "trigger_id", util.UUIDToString(trigger.ID), "schedule", trigger.Schedule.String, "error", err)
			return
		}
		if next := sched.Next(time.Now().In(loc)); !next.IsZero() {
			newNextRunAt = pgtype.Timestamptz{Time: next, Valid: true}
		}
	}

	claimed, err := queries.ClaimRoutineScheduleTrigger(ctx, db.ClaimRoutineScheduleTriggerParams{
		ID:          trigger.ID,
		NextRunAt:   trigger.NextRunAt,
		NextRunAt_2: newNextRunAt,
	})
	if err != nil {
		slog.Debug("routine scheduler: trigger already claimed or not due", "trigger_id", util.UUIDToString(trigger.ID))
		return
	}

	actions, err := queries.ListEnabledRoutineActions(ctx, routine.ID)
	if err != nil {
		slog.Warn("routine scheduler: list actions failed", "routine_id", util.UUIDToString(routine.ID), "error", err)
		return
	}
	evt := service.RoutineEvent{
		Type:     "schedule",
		DedupKey: util.UUIDToString(claimed.ID) + ":" + time.Now().UTC().Format(time.RFC3339),
		Data: map[string]string{
			"title": routine.Name,
			"body":  routine.Instructions.String,
		},
		Payload: []byte("{}"),
	}
	ran := false
	for _, action := range actions {
		result, err := routineSvc.ExecuteAction(ctx, routine, claimed, action, evt)
		if err != nil {
			slog.Warn("routine scheduler: action failed", "routine_id", util.UUIDToString(routine.ID), "action_id", util.UUIDToString(action.ID), "error", err)
			continue
		}
		ran = ran || result.Ran
	}
	if ran {
		if _, err := queries.IncrementRoutineTriggerSuccess(ctx, claimed.ID); err != nil {
			slog.Warn("routine scheduler: increment success failed", "trigger_id", util.UUIDToString(claimed.ID), "error", err)
		}
	}
}
