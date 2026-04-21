package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/auth"
	gh "github.com/nullne/multica/server/internal/github"
	wh "github.com/nullne/multica/server/internal/webhook"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// GitHubInstallURL returns the GitHub App installation URL.
func (h *Handler) GitHubInstallURL(w http.ResponseWriter, r *http.Request) {
	if h.GitHubApp == nil {
		writeError(w, http.StatusNotImplemented, "GitHub App not configured")
		return
	}
	workspaceID := resolveWorkspaceID(r)
	url := h.GitHubApp.InstallURL(workspaceID)
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// GitHubConnect handles the POST request from the frontend after a user
// completes the GitHub App installation flow. The frontend page at
// /github/callback receives the redirect from GitHub (with installation_id
// and state query params) and calls this endpoint.
//
// Side effects (in one transaction):
//  1. Persist installation_id on the workspace.
//  2. Upsert a source_type='github' webhook record bound to installation_id.
//     This is what /api/github/events looks up to route incoming events
//     through the unified webhook pipeline.
func (h *Handler) GitHubConnect(w http.ResponseWriter, r *http.Request) {
	if h.GitHubApp == nil {
		writeError(w, http.StatusNotImplemented, "GitHub App not configured")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var body struct {
		InstallationID int64 `json:"installation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InstallationID == 0 {
		writeError(w, http.StatusBadRequest, "missing or invalid installation_id")
		return
	}

	workspaceID := resolveWorkspaceID(r)
	installationID := pgtype.Int8{Int64: body.InstallationID, Valid: true}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	ws, err := qtx.SetGitHubInstallation(r.Context(), db.SetGitHubInstallationParams{
		ID:                   parseUUID(workspaceID),
		GithubInstallationID: installationID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save installation")
		return
	}

	// Idempotency: if a webhook is already bound to this installation (e.g.
	// the user re-runs the install flow), skip creation.
	if _, err := qtx.GetWebhookByInstallationID(r.Context(), installationID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "failed to check existing webhook")
			return
		}
		// No existing webhook — create one. token_hash is required+unique by
		// schema even though GitHub webhooks authenticate via HMAC, so we
		// generate a random token to satisfy the constraint. The token is
		// never returned to the client.
		rawToken, tokErr := wh.GenerateToken()
		if tokErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate token")
			return
		}
		prefix := rawToken
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		if _, err := qtx.CreateWebhook(r.Context(), db.CreateWebhookParams{
			WorkspaceID:        ws.ID,
			Name:               "GitHub App",
			SourceType:         "github",
			TokenHash:          auth.HashToken(rawToken),
			TokenPrefix:        prefix,
			DedupWindowSeconds: 600,
			CreatedBy:          parseUUID(userID),
			InstallationID:     installationID,
		}); err != nil {
			slog.Warn("github connect: create webhook failed", "workspace_id", workspaceID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to create github webhook")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	writeJSON(w, http.StatusOK, workspaceToResponse(ws))
}

// GitHubStatus returns the GitHub App connection status for the workspace.
func (h *Handler) GitHubStatus(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	resp := map[string]any{
		"connected":       ws.GithubInstallationID.Valid,
		"installation_id": nil,
		"app_configured":  h.GitHubApp != nil,
	}
	if ws.GithubInstallationID.Valid {
		resp["installation_id"] = ws.GithubInstallationID.Int64
	}

	writeJSON(w, http.StatusOK, resp)
}

// GitHubDisconnect removes the GitHub App installation from the workspace
// and the matching webhook record (so /api/github/events stops routing
// events for this installation).
func (h *Handler) GitHubDisconnect(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	current, err := qtx.GetWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	if current.GithubInstallationID.Valid {
		if err := qtx.DeleteWebhookByInstallationID(r.Context(), current.GithubInstallationID); err != nil {
			slog.Warn("github disconnect: delete webhook failed",
				"workspace_id", workspaceID,
				"installation_id", current.GithubInstallationID.Int64,
				"error", err,
			)
		}
	}

	ws, err := qtx.ClearGitHubInstallation(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disconnect")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	writeJSON(w, http.StatusOK, workspaceToResponse(ws))
}

// generateGitHubTokenForAgent creates a scoped GitHub installation token
// for the given workspace and agent code access level. Returns "" if the
// workspace has no GitHub App installation or token generation fails.
func (h *Handler) generateGitHubTokenForAgent(r *http.Request, ws db.Workspace, codeAccess string, repoURLs []string) string {
	if h.GitHubApp == nil || !ws.GithubInstallationID.Valid {
		return ""
	}

	perms := gh.PermissionsForCodeAccess(codeAccess)
	repoNames := gh.RepoNamesFromURLs(repoURLs)

	token, err := h.GitHubApp.CreateInstallationToken(
		r.Context(),
		ws.GithubInstallationID.Int64,
		perms,
		repoNames,
	)
	if err != nil {
		slog.Warn("github: failed to create installation token",
			"workspace_id", uuidToString(ws.ID),
			"installation_id", ws.GithubInstallationID.Int64,
			"error", err,
		)
		return ""
	}
	return token.Token
}
