package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotificationChannelAPI(t *testing.T) {
	ctx := context.Background()
	// Clean up any pre-existing test data.
	testPool.Exec(ctx, `DELETE FROM user_notification_channel WHERE user_id = $1`, testUserID)
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM user_notification_channel WHERE user_id = $1`, testUserID)
	})

	t.Run("ListEmpty", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := newRequest("GET", "/api/me/notification-channels", nil)
		testHandler.ListNotificationChannels(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var channels []NotificationChannelResponse
		json.NewDecoder(w.Body).Decode(&channels)
		if len(channels) != 0 {
			t.Fatalf("expected empty list, got %d channels", len(channels))
		}
	})

	t.Run("UpsertTelegram", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := newRequest("PUT", "/api/me/notification-channels/telegram", map[string]any{
			"chat_id": "12345678",
			"preferences": map[string]bool{
				"comments":      false,
				"field_changes": true,
			},
		})
		testHandler.UpsertTelegramChannel(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var ch NotificationChannelResponse
		json.NewDecoder(w.Body).Decode(&ch)
		if ch.ChannelType != "telegram" {
			t.Fatalf("expected telegram channel, got %q", ch.ChannelType)
		}
		if ch.ChannelID != "12345678" {
			t.Fatalf("expected channel_id 12345678, got %q", ch.ChannelID)
		}
		if !ch.Enabled {
			t.Fatal("expected channel to be enabled by default")
		}
		if ch.Preferences["comments"] {
			t.Fatal("expected comments preference to be disabled")
		}
		if !ch.Preferences["field_changes"] {
			t.Fatal("expected field_changes preference to be enabled")
		}
		if !ch.Preferences["assignments_mentions"] {
			t.Fatal("expected omitted assignment preference to default enabled")
		}
	})

	t.Run("ListAfterUpsert", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := newRequest("GET", "/api/me/notification-channels", nil)
		testHandler.ListNotificationChannels(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var channels []NotificationChannelResponse
		json.NewDecoder(w.Body).Decode(&channels)
		if len(channels) != 1 {
			t.Fatalf("expected 1 channel, got %d", len(channels))
		}
		if channels[0].ChannelType != "telegram" {
			t.Fatalf("expected telegram, got %q", channels[0].ChannelType)
		}
		if channels[0].Preferences["comments"] {
			t.Fatal("expected stored comments preference to remain disabled")
		}
	})

	t.Run("UpdateTelegramDisable", func(t *testing.T) {
		enabled := false
		w := httptest.NewRecorder()
		req := newRequest("PUT", "/api/me/notification-channels/telegram", map[string]any{
			"chat_id": "12345678",
			"enabled": enabled,
		})
		testHandler.UpsertTelegramChannel(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var ch NotificationChannelResponse
		json.NewDecoder(w.Body).Decode(&ch)
		if ch.Enabled {
			t.Fatal("expected channel to be disabled")
		}
	})

	t.Run("DeleteTelegram", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := newRequest("DELETE", "/api/me/notification-channels/telegram", nil)
		testHandler.DeleteTelegramChannel(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}

		// Verify it's gone
		w = httptest.NewRecorder()
		req = newRequest("GET", "/api/me/notification-channels", nil)
		testHandler.ListNotificationChannels(w, req)
		var channels []NotificationChannelResponse
		json.NewDecoder(w.Body).Decode(&channels)
		if len(channels) != 0 {
			t.Fatalf("expected empty list after delete, got %d channels", len(channels))
		}
	})

	t.Run("UpsertMissingChatID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := newRequest("PUT", "/api/me/notification-channels/telegram", map[string]any{
			"chat_id": "",
		})
		testHandler.UpsertTelegramChannel(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}
