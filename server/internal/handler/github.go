package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	gh "github.com/nullne/multica/server/internal/github"
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
func (h *Handler) GitHubConnect(w http.ResponseWriter, r *http.Request) {
	if h.GitHubApp == nil {
		writeError(w, http.StatusNotImplemented, "GitHub App not configured")
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
	ws, err := h.Queries.SetGitHubInstallation(r.Context(), db.SetGitHubInstallationParams{
		ID:                   parseUUID(workspaceID),
		GithubInstallationID: pgtype.Int8{Int64: body.InstallationID, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save installation")
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

// GitHubDisconnect removes the GitHub App installation from the workspace.
func (h *Handler) GitHubDisconnect(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	ws, err := h.Queries.ClearGitHubInstallation(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disconnect")
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
