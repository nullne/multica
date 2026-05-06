package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/nullne/multica/server/internal/logger"
	"github.com/nullne/multica/server/pkg/protocol"
)

// WorkspaceTelegramSettings holds the workspace-level Telegram group chat configuration.
type WorkspaceTelegramSettings struct {
	ChatID  string `json:"chat_id"`
	Enabled bool   `json:"enabled"`
}

// ParseTelegramGroupSettings extracts the telegram_group section from the raw
// workspace settings JSONB. Returns nil when not configured.
func ParseTelegramGroupSettings(raw []byte) *WorkspaceTelegramSettings {
	if raw == nil {
		return nil
	}
	var full map[string]json.RawMessage
	if err := json.Unmarshal(raw, &full); err != nil {
		return nil
	}
	tgRaw, ok := full["telegram_group"]
	if !ok {
		return nil
	}
	var s WorkspaceTelegramSettings
	if err := json.Unmarshal(tgRaw, &s); err != nil {
		return nil
	}
	return &s
}

// MergeTelegramGroupForTest merges Telegram group settings into workspace settings JSONB.
// Exported for use in cross-package tests only.
func MergeTelegramGroupForTest(raw []byte, s WorkspaceTelegramSettings) ([]byte, error) {
	return mergeTelegramGroupIntoWorkspace(raw, s)
}

// ClearTelegramGroupForTest removes telegram_group from workspace settings JSONB.
// Exported for use in cross-package tests only.
func ClearTelegramGroupForTest(raw []byte) ([]byte, error) {
	return clearTelegramGroupFromWorkspace(raw)
}

// mergeTelegramGroupIntoWorkspace merges the Telegram group settings into the
// raw workspace settings JSONB, preserving all other fields.
func mergeTelegramGroupIntoWorkspace(raw []byte, s WorkspaceTelegramSettings) ([]byte, error) {
	var full map[string]json.RawMessage
	if raw != nil {
		if err := json.Unmarshal(raw, &full); err != nil {
			full = make(map[string]json.RawMessage)
		}
	}
	if full == nil {
		full = make(map[string]json.RawMessage)
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	full["telegram_group"] = b
	return json.Marshal(full)
}

// clearTelegramGroupFromWorkspace removes the telegram_group key from workspace
// settings JSONB, preserving all other fields.
func clearTelegramGroupFromWorkspace(raw []byte) ([]byte, error) {
	var full map[string]json.RawMessage
	if raw != nil {
		if err := json.Unmarshal(raw, &full); err != nil {
			full = make(map[string]json.RawMessage)
		}
	}
	if full == nil {
		full = make(map[string]json.RawMessage)
	}
	delete(full, "telegram_group")
	return json.Marshal(full)
}

// GetWorkspaceTelegramNotifications returns the workspace Telegram group notification settings.
func (h *Handler) GetWorkspaceTelegramNotifications(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")

	ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	s := ParseTelegramGroupSettings(ws.Settings)
	if s == nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true,
		"chat_id":    s.ChatID,
		"enabled":    s.Enabled,
	})
}

type UpsertWorkspaceTelegramRequest struct {
	ChatID  string `json:"chat_id"`
	Enabled *bool  `json:"enabled"`
}

// UpsertWorkspaceTelegramNotifications saves or updates the workspace Telegram group chat ID.
func (h *Handler) UpsertWorkspaceTelegramNotifications(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")

	var req UpsertWorkspaceTelegramRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	chatID := strings.TrimSpace(req.ChatID)
	if chatID == "" {
		writeError(w, http.StatusBadRequest, "chat_id is required")
		return
	}

	ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	merged, err := mergeTelegramGroupIntoWorkspace(ws.Settings, WorkspaceTelegramSettings{
		ChatID:  chatID,
		Enabled: enabled,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode settings")
		return
	}

	ws, err = h.Queries.UpdateWorkspace(r.Context(), updateSettingsOnly(ws.ID, merged))
	if err != nil {
		slog.Warn("upsert workspace telegram failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", id)...)
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	userID := requestUserID(r)
	h.publish(protocol.EventWorkspaceUpdated, id, "member", userID, map[string]any{"workspace": workspaceToResponse(ws)})

	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true,
		"chat_id":    chatID,
		"enabled":    enabled,
	})
}

// DeleteWorkspaceTelegramNotifications removes the workspace Telegram group configuration.
func (h *Handler) DeleteWorkspaceTelegramNotifications(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")

	ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	cleared, err := clearTelegramGroupFromWorkspace(ws.Settings)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode settings")
		return
	}

	ws, err = h.Queries.UpdateWorkspace(r.Context(), updateSettingsOnly(ws.ID, cleared))
	if err != nil {
		slog.Warn("delete workspace telegram failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", id)...)
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	userID := requestUserID(r)
	h.publish(protocol.EventWorkspaceUpdated, id, "member", userID, map[string]any{"workspace": workspaceToResponse(ws)})

	w.WriteHeader(http.StatusNoContent)
}
