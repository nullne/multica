package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

type LabelResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
}

func labelToResponse(l db.IssueLabel) LabelResponse {
	return LabelResponse{
		ID:          uuidToString(l.ID),
		WorkspaceID: uuidToString(l.WorkspaceID),
		Name:        l.Name,
		Color:       l.Color,
	}
}

func (h *Handler) ListLabels(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	labels, err := h.Queries.ListLabels(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list labels")
		return
	}

	resp := make([]LabelResponse, len(labels))
	for i, l := range labels {
		resp[i] = labelToResponse(l)
	}
	writeJSON(w, http.StatusOK, resp)
}

type CreateLabelRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (h *Handler) CreateLabel(w http.ResponseWriter, r *http.Request) {
	var req CreateLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Color == "" {
		writeError(w, http.StatusBadRequest, "color is required")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := resolveWorkspaceID(r)

	label, err := h.Queries.CreateLabel(r.Context(), db.CreateLabelParams{
		WorkspaceID: parseUUID(workspaceID),
		Name:        req.Name,
		Color:       req.Color,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create label")
		return
	}

	resp := labelToResponse(label)
	h.publish(protocol.EventLabelCreated, workspaceID, "member", userID, map[string]any{"label": resp})
	writeJSON(w, http.StatusCreated, resp)
}

type UpdateLabelRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (h *Handler) UpdateLabel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req UpdateLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Color == "" {
		writeError(w, http.StatusBadRequest, "color is required")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := resolveWorkspaceID(r)

	label, err := h.Queries.UpdateLabel(r.Context(), db.UpdateLabelParams{
		ID:          parseUUID(id),
		WorkspaceID: parseUUID(workspaceID),
		Name:        req.Name,
		Color:       req.Color,
	})
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "label not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update label")
		return
	}

	resp := labelToResponse(label)
	h.publish(protocol.EventLabelUpdated, workspaceID, "member", userID, map[string]any{"label": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteLabel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := resolveWorkspaceID(r)

	err := h.Queries.DeleteLabel(r.Context(), db.DeleteLabelParams{
		ID:          parseUUID(id),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete label")
		return
	}

	h.publish(protocol.EventLabelDeleted, workspaceID, "member", userID, map[string]any{"label_id": id})
	w.WriteHeader(http.StatusNoContent)
}

// SetIssueLabels replaces all labels on an issue with the provided set.
type SetIssueLabelsRequest struct {
	LabelIDs []string `json:"label_ids"`
}

func (h *Handler) SetIssueLabels(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")

	var req SetIssueLabelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	issue, ok := h.loadIssueForUser(w, r, issueID)
	if !ok {
		return
	}

	userID := requestUserID(r)
	workspaceID := uuidToString(issue.WorkspaceID)

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set labels")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	if err := qtx.RemoveAllLabelsFromIssue(r.Context(), issue.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set labels")
		return
	}

	for _, labelID := range req.LabelIDs {
		if err := qtx.AddLabelToIssue(r.Context(), db.AddLabelToIssueParams{
			IssueID: issue.ID,
			LabelID: parseUUID(labelID),
		}); err != nil {
			slog.Warn("add label to issue failed", "issue_id", issueID, "label_id", labelID, "error", err)
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set labels")
		return
	}

	labels, err := h.Queries.ListLabelsForIssue(r.Context(), issue.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch labels")
		return
	}

	resp := make([]LabelResponse, len(labels))
	for i, l := range labels {
		resp[i] = labelToResponse(l)
	}

	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	prefix := h.getIssuePrefix(r.Context(), issue.WorkspaceID)
	issueResp := issueToResponse(issue, prefix)
	issueResp.Labels = resp
	h.hydrateLinks(r.Context(), issue.ID, &issueResp)
	h.publish(protocol.EventIssueUpdated, workspaceID, actorType, actorID, map[string]any{
		"issue":          issueResp,
		"labels_changed": true,
	})

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListIssueLabels(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")

	issue, ok := h.loadIssueForUser(w, r, issueID)
	if !ok {
		return
	}

	labels, err := h.Queries.ListLabelsForIssue(r.Context(), issue.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list labels")
		return
	}

	resp := make([]LabelResponse, len(labels))
	for i, l := range labels {
		resp[i] = labelToResponse(l)
	}
	writeJSON(w, http.StatusOK, resp)
}
