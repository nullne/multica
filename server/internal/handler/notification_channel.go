package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	db "github.com/nullne/multica/server/pkg/db/generated"
)

type NotificationChannelResponse struct {
	ChannelType string `json:"channel_type"`
	ChannelID   string `json:"channel_id"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func notificationChannelToResponse(c db.UserNotificationChannel) NotificationChannelResponse {
	return NotificationChannelResponse{
		ChannelType: c.ChannelType,
		ChannelID:   c.ChannelID,
		Enabled:     c.Enabled,
		CreatedAt:   timestampToString(c.CreatedAt),
		UpdatedAt:   timestampToString(c.UpdatedAt),
	}
}

// ListNotificationChannels returns all notification channels for the current user.
func (h *Handler) ListNotificationChannels(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	channels, err := h.Queries.ListUserNotificationChannels(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}

	resp := make([]NotificationChannelResponse, 0, len(channels))
	for _, c := range channels {
		resp = append(resp, notificationChannelToResponse(c))
	}
	writeJSON(w, http.StatusOK, resp)
}

type UpsertTelegramChannelRequest struct {
	ChatID  string `json:"chat_id"`
	Enabled *bool  `json:"enabled"`
}

// UpsertTelegramChannel creates or updates the user's Telegram notification channel.
func (h *Handler) UpsertTelegramChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req UpsertTelegramChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	chatID := strings.TrimSpace(req.ChatID)
	if chatID == "" {
		writeError(w, http.StatusBadRequest, "chat_id is required")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	channel, err := h.Queries.UpsertUserNotificationChannel(r.Context(), db.UpsertUserNotificationChannelParams{
		UserID:      parseUUID(userID),
		ChannelType: "telegram",
		ChannelID:   chatID,
		Enabled:     enabled,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save channel")
		return
	}

	writeJSON(w, http.StatusOK, notificationChannelToResponse(channel))
}

// DeleteTelegramChannel removes the user's Telegram notification channel.
func (h *Handler) DeleteTelegramChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	if err := h.Queries.DeleteUserNotificationChannel(r.Context(), db.DeleteUserNotificationChannelParams{
		UserID:      parseUUID(userID),
		ChannelType: "telegram",
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete channel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
