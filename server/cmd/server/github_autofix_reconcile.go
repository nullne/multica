package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nullne/multica/server/internal/handler"
	"github.com/nullne/multica/server/internal/util"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// reconcileGitHubAutoFixRoutines ensures every workspace with a connected
// GitHub App has its managed auto-fix routine provisioned. It runs once at
// startup and is idempotent — workspaces that already have the routine are
// skipped. This backfills workspaces connected before the routine existed.
func reconcileGitHubAutoFixRoutines(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries) {
	workspaces, err := queries.ListConnectedGitHubWorkspaces(ctx)
	if err != nil {
		slog.Warn("auto-fix reconcile: list connected workspaces failed", "error", err)
		return
	}
	for _, ws := range workspaces {
		if !ws.GithubInstallationID.Valid {
			continue
		}
		if err := ensureManagedAutoFixRoutine(ctx, pool, queries, ws.ID, ws.GithubInstallationID); err != nil {
			slog.Warn("auto-fix reconcile: provision routine failed", "workspace_id", util.UUIDToString(ws.ID), "error", err)
		}
	}
}

// ensureManagedAutoFixRoutine provisions the managed routine for one
// workspace inside its own transaction so a partial failure does not leave a
// half-created routine that the idempotency check would later skip.
func ensureManagedAutoFixRoutine(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, workspaceID pgtype.UUID, installationID pgtype.Int8) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := queries.WithTx(tx)
	if err := handler.EnsureGitHubAutoFixRoutine(ctx, qtx, tx, workspaceID, installationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
