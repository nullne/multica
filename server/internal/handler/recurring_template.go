package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/cron"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// RecurringTemplateResponse is the JSON shape returned to clients.
type RecurringTemplateResponse struct {
	ID                  string  `json:"id"`
	WorkspaceID         string  `json:"workspace_id"`
	Title               string  `json:"title"`
	Description         *string `json:"description"`
	Priority            string  `json:"priority"`
	AssigneeType        *string `json:"assignee_type"`
	AssigneeID          *string `json:"assignee_id"`
	DueDateOffsetHours  *int32  `json:"due_date_offset_hours"`
	DispatchProvider    *string `json:"dispatch_provider"`
	DispatchDaemonID    *string `json:"dispatch_daemon_id"`
	DispatchDaemonLabel *string `json:"dispatch_daemon_label"`
	Schedule            string  `json:"schedule"`
	Timezone            string  `json:"timezone"`
	Enabled             bool    `json:"enabled"`
	LastTriggeredAt     *string `json:"last_triggered_at"`
	NextRunAt           *string `json:"next_run_at"`
	CreatedByID         string  `json:"created_by_id"`
	CreatedByType       string  `json:"created_by_type"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func recurringTemplateToResponse(t db.RecurringIssueTemplate) RecurringTemplateResponse {
	var dueDateOffset *int32
	if t.DueDateOffsetHours.Valid {
		dueDateOffset = &t.DueDateOffsetHours.Int32
	}
	return RecurringTemplateResponse{
		ID:                  uuidToString(t.ID),
		WorkspaceID:         uuidToString(t.WorkspaceID),
		Title:               t.Title,
		Description:         textToPtr(t.Description),
		Priority:            t.Priority,
		AssigneeType:        textToPtr(t.AssigneeType),
		AssigneeID:          uuidToPtr(t.AssigneeID),
		DueDateOffsetHours:  dueDateOffset,
		DispatchProvider:    textToPtr(t.DispatchProvider),
		DispatchDaemonID:    uuidToPtr(t.DispatchDaemonID),
		DispatchDaemonLabel: textToPtr(t.DispatchDaemonLabel),
		Schedule:            t.Schedule,
		Timezone:            t.Timezone,
		Enabled:             t.Enabled,
		LastTriggeredAt:     timestampToPtr(t.LastTriggeredAt),
		NextRunAt:           timestampToPtr(t.NextRunAt),
		CreatedByID:         uuidToString(t.CreatedByID),
		CreatedByType:       t.CreatedByType,
		CreatedAt:           timestampToString(t.CreatedAt),
		UpdatedAt:           timestampToString(t.UpdatedAt),
	}
}

type CreateRecurringTemplateRequest struct {
	Title               string  `json:"title"`
	Description         *string `json:"description"`
	Priority            *string `json:"priority"`
	AssigneeType        *string `json:"assignee_type"`
	AssigneeID          *string `json:"assignee_id"`
	DueDateOffsetHours  *int32  `json:"due_date_offset_hours"`
	DispatchProvider    *string `json:"dispatch_provider"`
	DispatchDaemonID    *string `json:"dispatch_daemon_id"`
	DispatchDaemonLabel *string `json:"dispatch_daemon_label"`
	Schedule            string  `json:"schedule"`
	Timezone            string  `json:"timezone"`
	Enabled             *bool   `json:"enabled"`
}

func (h *Handler) CreateRecurringTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	var req CreateRecurringTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Schedule == "" {
		writeError(w, http.StatusBadRequest, "schedule is required")
		return
	}

	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid timezone: "+tz)
		return
	}

	sched, err := cron.Parse(req.Schedule)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule: "+err.Error())
		return
	}

	priority := "medium"
	if req.Priority != nil && *req.Priority != "" {
		priority = *req.Priority
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	nextRunAt := pgtype.Timestamptz{}
	if enabled {
		if next := sched.Next(time.Now().In(loc)); !next.IsZero() {
			nextRunAt = pgtype.Timestamptz{Time: next, Valid: true}
		}
	}

	var dueDateOffset pgtype.Int4
	if req.DueDateOffsetHours != nil {
		dueDateOffset = pgtype.Int4{Int32: *req.DueDateOffsetHours, Valid: true}
	}

	t, err := h.Queries.CreateRecurringTemplate(r.Context(), db.CreateRecurringTemplateParams{
		WorkspaceID:         parseUUID(workspaceID),
		Title:               req.Title,
		Description:         ptrToText(req.Description),
		Priority:            priority,
		AssigneeType:        ptrToText(req.AssigneeType),
		AssigneeID:          parseOptionalUUID(req.AssigneeID),
		DueDateOffsetHours:  dueDateOffset,
		DispatchProvider:    ptrToText(req.DispatchProvider),
		DispatchDaemonID:    parseOptionalUUID(req.DispatchDaemonID),
		DispatchDaemonLabel: ptrToText(req.DispatchDaemonLabel),
		Schedule:            req.Schedule,
		Timezone:            tz,
		Enabled:             enabled,
		NextRunAt:           nextRunAt,
		CreatedByID:         member.UserID,
		CreatedByType:       "member",
	})
	if err != nil {
		slog.Warn("create recurring template failed", "error", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "failed to create recurring template")
		return
	}

	writeJSON(w, http.StatusCreated, recurringTemplateToResponse(t))
}

func (h *Handler) ListRecurringTemplates(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	if _, ok := requireUserID(w, r); !ok {
		return
	}

	templates, err := h.Queries.ListRecurringTemplates(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list recurring templates")
		return
	}

	resp := make([]RecurringTemplateResponse, len(templates))
	for i, t := range templates {
		resp[i] = recurringTemplateToResponse(t)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetRecurringTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	id := chi.URLParam(r, "id")
	if _, ok := requireUserID(w, r); !ok {
		return
	}

	t, err := h.Queries.GetRecurringTemplateInWorkspace(r.Context(), parseUUID(id), parseUUID(workspaceID))
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "recurring template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get recurring template")
		return
	}

	writeJSON(w, http.StatusOK, recurringTemplateToResponse(t))
}

type UpdateRecurringTemplateRequest struct {
	Title               *string `json:"title"`
	Description         *string `json:"description"`
	Priority            *string `json:"priority"`
	AssigneeType        *string `json:"assignee_type"`
	AssigneeID          *string `json:"assignee_id"`
	DueDateOffsetHours  *int32  `json:"due_date_offset_hours"`
	DispatchProvider    *string `json:"dispatch_provider"`
	DispatchDaemonID    *string `json:"dispatch_daemon_id"`
	DispatchDaemonLabel *string `json:"dispatch_daemon_label"`
	Schedule            *string `json:"schedule"`
	Timezone            *string `json:"timezone"`
	Enabled             *bool   `json:"enabled"`
}

func (h *Handler) UpdateRecurringTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	id := chi.URLParam(r, "id")

	existing, err := h.Queries.GetRecurringTemplateInWorkspace(r.Context(), parseUUID(id), parseUUID(workspaceID))
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "recurring template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get recurring template")
		return
	}

	// Read body as raw bytes so we can detect which fields were explicitly
	// sent in the request (including explicit JSON null), which lets PATCH
	// callers clear nullable fields like assignee_id back to null.
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req UpdateRecurringTemplateRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var rawFields map[string]json.RawMessage
	_ = json.Unmarshal(bodyBytes, &rawFields)

	// Start with existing values.
	params := db.UpdateRecurringTemplateParams{
		ID:                  existing.ID,
		WorkspaceID:         existing.WorkspaceID,
		Title:               existing.Title,
		Description:         existing.Description,
		Priority:            existing.Priority,
		AssigneeType:        existing.AssigneeType,
		AssigneeID:          existing.AssigneeID,
		DueDateOffsetHours:  existing.DueDateOffsetHours,
		DispatchProvider:    existing.DispatchProvider,
		DispatchDaemonID:    existing.DispatchDaemonID,
		DispatchDaemonLabel: existing.DispatchDaemonLabel,
		Schedule:            existing.Schedule,
		Timezone:            existing.Timezone,
		Enabled:             existing.Enabled,
		NextRunAt:           existing.NextRunAt,
	}

	if req.Title != nil {
		params.Title = *req.Title
	}
	if req.Priority != nil {
		params.Priority = *req.Priority
	}
	if req.Enabled != nil {
		params.Enabled = *req.Enabled
	}
	// Nullable fields — only override when explicitly present in JSON
	// so that explicit null clears the field while omission preserves it.
	if _, ok := rawFields["description"]; ok {
		params.Description = ptrToText(req.Description)
	}
	if _, ok := rawFields["assignee_type"]; ok {
		params.AssigneeType = ptrToText(req.AssigneeType)
	}
	if _, ok := rawFields["assignee_id"]; ok {
		params.AssigneeID = parseOptionalUUID(req.AssigneeID)
	}
	if _, ok := rawFields["due_date_offset_hours"]; ok {
		if req.DueDateOffsetHours != nil {
			params.DueDateOffsetHours = pgtype.Int4{Int32: *req.DueDateOffsetHours, Valid: true}
		} else {
			params.DueDateOffsetHours = pgtype.Int4{Valid: false}
		}
	}
	if _, ok := rawFields["dispatch_provider"]; ok {
		params.DispatchProvider = ptrToText(req.DispatchProvider)
	}
	if _, ok := rawFields["dispatch_daemon_id"]; ok {
		params.DispatchDaemonID = parseOptionalUUID(req.DispatchDaemonID)
	}
	if _, ok := rawFields["dispatch_daemon_label"]; ok {
		params.DispatchDaemonLabel = ptrToText(req.DispatchDaemonLabel)
	}

	// Recompute next_run_at if schedule, timezone, or enabled changed.
	scheduleChanged := req.Schedule != nil && *req.Schedule != existing.Schedule
	timezoneChanged := req.Timezone != nil && *req.Timezone != existing.Timezone
	enabledChanged := req.Enabled != nil && *req.Enabled != existing.Enabled

	if req.Schedule != nil {
		params.Schedule = *req.Schedule
	}
	if req.Timezone != nil {
		params.Timezone = *req.Timezone
	}

	if scheduleChanged || timezoneChanged || enabledChanged || !existing.NextRunAt.Valid {
		if !params.Enabled {
			params.NextRunAt = pgtype.Timestamptz{}
		} else {
			loc, err := time.LoadLocation(params.Timezone)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid timezone: "+params.Timezone)
				return
			}
			sched, err := cron.Parse(params.Schedule)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid schedule: "+err.Error())
				return
			}
			if next := sched.Next(time.Now().In(loc)); !next.IsZero() {
				params.NextRunAt = pgtype.Timestamptz{Time: next, Valid: true}
			}
		}
	}

	updated, err := h.Queries.UpdateRecurringTemplate(r.Context(), params)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "recurring template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update recurring template")
		return
	}

	writeJSON(w, http.StatusOK, recurringTemplateToResponse(updated))
}

func (h *Handler) DeleteRecurringTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	id := chi.URLParam(r, "id")
	if _, ok := requireUserID(w, r); !ok {
		return
	}

	if err := h.Queries.DeleteRecurringTemplate(r.Context(), parseUUID(id), parseUUID(workspaceID)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete recurring template")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
