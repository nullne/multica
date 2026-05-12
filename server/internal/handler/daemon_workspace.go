package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

// DaemonWorkspaceAssignmentResponse describes which workspaces a daemon is
// assigned to. eligible_workspaces lists workspaces the daemon's owner can
// enable the daemon for.
type DaemonWorkspaceAssignmentResponse struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	Enabled       bool   `json:"enabled"`
}

// ListMyDaemons returns daemons owned by the authenticated user.
func (h *Handler) ListMyDaemons(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	daemons, err := h.Queries.ListDaemonsForUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list daemons")
		return
	}

	resp := make([]DaemonResponse, len(daemons))
	for i, d := range daemons {
		resp[i] = daemonToResponse(d)
	}
	writeJSON(w, http.StatusOK, resp)
}

// requireOwnedDaemon resolves a daemon by URL parameter and verifies that the
// authenticated user owns it.
func (h *Handler) requireOwnedDaemon(w http.ResponseWriter, r *http.Request) (db.Daemon, bool) {
	daemonID := chi.URLParam(r, "daemonId")
	userID, ok := requireUserID(w, r)
	if !ok {
		return db.Daemon{}, false
	}
	d, err := h.Queries.GetDaemonForUser(r.Context(), db.GetDaemonForUserParams{
		ID:     parseUUID(daemonID),
		UserID: parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "daemon not found")
		return db.Daemon{}, false
	}
	return d, true
}

// ListMyDaemonWorkspaces lists workspace assignments for one of the user's
// daemons, showing every workspace the user is a member of plus whether the
// daemon is enabled there.
func (h *Handler) ListMyDaemonWorkspaces(w http.ResponseWriter, r *http.Request) {
	d, ok := h.requireOwnedDaemon(w, r)
	if !ok {
		return
	}

	assignments, err := h.Queries.ListWorkspacesForDaemon(r.Context(), d.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list assignments")
		return
	}
	enabled := make(map[string]bool, len(assignments))
	for _, a := range assignments {
		enabled[uuidToString(a.WorkspaceID)] = a.Enabled
	}

	workspaces, err := h.Queries.ListWorkspaces(r.Context(), d.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workspaces")
		return
	}

	resp := make([]DaemonWorkspaceAssignmentResponse, 0, len(workspaces))
	for _, ws := range workspaces {
		resp = append(resp, DaemonWorkspaceAssignmentResponse{
			WorkspaceID:   uuidToString(ws.ID),
			WorkspaceName: ws.Name,
			Enabled:       enabled[uuidToString(ws.ID)],
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// SetMyDaemonWorkspaceEnabled enables or disables a daemon for a workspace.
// The body must include {"enabled": bool}. The caller must own the daemon and
// must be a member of the target workspace.
func (h *Handler) SetMyDaemonWorkspaceEnabled(w http.ResponseWriter, r *http.Request) {
	d, ok := h.requireOwnedDaemon(w, r)
	if !ok {
		return
	}

	workspaceID := chi.URLParam(r, "workspaceId")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	// The owner must be a member of the workspace.
	if _, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found"); !ok {
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	assignment, err := h.Queries.UpsertDaemonWorkspace(r.Context(), db.UpsertDaemonWorkspaceParams{
		DaemonID:    d.ID,
		WorkspaceID: parseUUID(workspaceID),
		Enabled:     req.Enabled,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update assignment")
		return
	}

	// When disabling, mark the daemon's runtime rows for this workspace
	// offline so no scheduler picks them.
	if !req.Enabled {
		if err := h.Queries.SetRuntimesOfflineByDaemonAndWorkspace(r.Context(), db.SetRuntimesOfflineByDaemonAndWorkspaceParams{
			DaemonRef:   d.ID,
			WorkspaceID: parseUUID(workspaceID),
		}); err != nil {
			slog.Warn("failed to mark daemon runtimes offline", "daemon_id", uuidToString(d.ID), "workspace_id", workspaceID, "error", err)
		}
	}

	h.publish(protocol.EventDaemonRegister, workspaceID, "system", "", map[string]any{
		"action":    "assignment_changed",
		"daemon_id": uuidToString(d.ID),
		"enabled":   req.Enabled,
	})

	slog.Info("daemon workspace assignment updated",
		"daemon_id", uuidToString(d.ID),
		"workspace_id", workspaceID,
		"enabled", req.Enabled,
	)

	writeJSON(w, http.StatusOK, DaemonWorkspaceAssignmentResponse{
		WorkspaceID: uuidToString(assignment.WorkspaceID),
		Enabled:     assignment.Enabled,
	})
}
