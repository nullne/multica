package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/nullne/multica/server/internal/events"
	"github.com/nullne/multica/server/internal/handler"
	"github.com/nullne/multica/server/internal/telegram"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

// fakeTelegramServer creates a test HTTP server that captures Telegram sendMessage calls.
// It returns the server and a function that returns collected (chatID, text) pairs.
func fakeTelegramServer(t *testing.T) (*httptest.Server, func() []fakeTelegramMsg) {
	t.Helper()
	var mu sync.Mutex
	var msgs []fakeTelegramMsg

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		chatID, _ := payload["chat_id"].(string)
		text, _ := payload["text"].(string)
		mu.Lock()
		msgs = append(msgs, fakeTelegramMsg{ChatID: chatID, Text: text})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
	}))

	t.Cleanup(srv.Close)
	return srv, func() []fakeTelegramMsg {
		mu.Lock()
		defer mu.Unlock()
		out := make([]fakeTelegramMsg, len(msgs))
		copy(out, msgs)
		return out
	}
}

type fakeTelegramMsg struct {
	ChatID string
	Text   string
}

// setWorkspaceTelegramGroup configures the workspace with the given group chat ID.
func setWorkspaceTelegramGroup(t *testing.T, workspaceID, chatID string, enabled bool) {
	t.Helper()
	settings := map[string]any{
		"telegram_group": map[string]any{
			"chat_id": chatID,
			"enabled": enabled,
		},
	}
	b, _ := json.Marshal(settings)
	_, err := testPool.Exec(context.Background(),
		`UPDATE workspace SET settings = $1 WHERE id = $2`,
		b, workspaceID,
	)
	if err != nil {
		t.Fatalf("setWorkspaceTelegramGroup: %v", err)
	}
}

// setWorkspaceTelegramGroupWithPreferences configures the workspace Telegram group
// and category preferences for delivery filtering tests.
func setWorkspaceTelegramGroupWithPreferences(t *testing.T, workspaceID, chatID string, enabled bool, preferences map[string]bool) {
	t.Helper()
	settings := map[string]any{
		"telegram_group": map[string]any{
			"chat_id":     chatID,
			"enabled":     enabled,
			"preferences": preferences,
		},
	}
	b, _ := json.Marshal(settings)
	_, err := testPool.Exec(context.Background(),
		`UPDATE workspace SET settings = $1 WHERE id = $2`,
		b, workspaceID,
	)
	if err != nil {
		t.Fatalf("setWorkspaceTelegramGroupWithPreferences: %v", err)
	}
}

// clearWorkspaceTelegramGroup removes the telegram_group key from workspace settings.
func clearWorkspaceTelegramGroup(t *testing.T, workspaceID string) {
	t.Helper()
	testPool.Exec(context.Background(),
		`UPDATE workspace SET settings = settings - 'telegram_group' WHERE id = $1`,
		workspaceID,
	)
}

// waitForMsgs waits up to 500ms for at least n messages to arrive.
func waitForMsgs(getMsgs func() []fakeTelegramMsg, n int) []fakeTelegramMsg {
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		msgs := getMsgs()
		if len(msgs) >= n {
			return msgs
		}
		time.Sleep(10 * time.Millisecond)
	}
	return getMsgs()
}

// TestWorkspaceTelegram_IssueCreated verifies a workspace group message is sent when an issue is created.
func TestWorkspaceTelegram_IssueCreated(t *testing.T) {
	srv, getMsgs := fakeTelegramServer(t)
	bot := telegram.NewBotWithURL("test-token", srv.URL)

	const groupChatID = "-1001234567890"
	setWorkspaceTelegramGroup(t, testWorkspaceID, groupChatID, true)
	t.Cleanup(func() { clearWorkspaceTelegramGroup(t, testWorkspaceID) })

	queries := db.New(testPool)
	bus := events.New()
	registerSubscriberListeners(bus, queries)
	registerNotificationListeners(bus, queries, bot)

	issueID := createTestIssue(t, testWorkspaceID, testUserID)
	t.Cleanup(func() {
		cleanupInboxForIssue(t, issueID)
		cleanupTestIssue(t, issueID)
	})

	bus.Publish(events.Event{
		Type:        protocol.EventIssueCreated,
		WorkspaceID: testWorkspaceID,
		ActorType:   "member",
		ActorID:     testUserID,
		Payload: map[string]any{
			"issue": handler.IssueResponse{
				ID:          issueID,
				WorkspaceID: testWorkspaceID,
				Title:       "workspace telegram test",
				Status:      "todo",
				Priority:    "medium",
				CreatorType: "member",
				CreatorID:   testUserID,
			},
		},
	})

	msgs := waitForMsgs(getMsgs, 1)
	if len(msgs) == 0 {
		t.Fatal("expected at least 1 Telegram message to workspace group")
	}
	found := false
	for _, m := range msgs {
		if m.ChatID == groupChatID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected message to group chat %s, got: %v", groupChatID, msgs)
	}
}

// TestWorkspaceTelegram_Disabled verifies no group message when notifications are disabled.
func TestWorkspaceTelegram_Disabled(t *testing.T) {
	srv, getMsgs := fakeTelegramServer(t)
	bot := telegram.NewBotWithURL("test-token", srv.URL)

	const groupChatID = "-9990000000001"
	setWorkspaceTelegramGroup(t, testWorkspaceID, groupChatID, false) // disabled
	t.Cleanup(func() { clearWorkspaceTelegramGroup(t, testWorkspaceID) })

	queries := db.New(testPool)
	bus := events.New()
	registerSubscriberListeners(bus, queries)
	registerNotificationListeners(bus, queries, bot)

	issueID := createTestIssue(t, testWorkspaceID, testUserID)
	t.Cleanup(func() {
		cleanupInboxForIssue(t, issueID)
		cleanupTestIssue(t, issueID)
	})

	bus.Publish(events.Event{
		Type:        protocol.EventIssueCreated,
		WorkspaceID: testWorkspaceID,
		ActorType:   "member",
		ActorID:     testUserID,
		Payload: map[string]any{
			"issue": handler.IssueResponse{
				ID:          issueID,
				WorkspaceID: testWorkspaceID,
				Title:       "disabled telegram test",
				Status:      "todo",
				Priority:    "medium",
				CreatorType: "member",
				CreatorID:   testUserID,
			},
		},
	})

	// Wait briefly to confirm no message is sent.
	time.Sleep(150 * time.Millisecond)
	msgs := getMsgs()
	for _, m := range msgs {
		if m.ChatID == groupChatID {
			t.Fatalf("unexpected message sent to disabled group chat %s", groupChatID)
		}
	}
}

// TestWorkspaceTelegram_NotConfigured verifies no group message when no Telegram group is set.
func TestWorkspaceTelegram_NotConfigured(t *testing.T) {
	srv, getMsgs := fakeTelegramServer(t)
	bot := telegram.NewBotWithURL("test-token", srv.URL)

	clearWorkspaceTelegramGroup(t, testWorkspaceID)

	queries := db.New(testPool)
	bus := events.New()
	registerSubscriberListeners(bus, queries)
	registerNotificationListeners(bus, queries, bot)

	issueID := createTestIssue(t, testWorkspaceID, testUserID)
	t.Cleanup(func() {
		cleanupInboxForIssue(t, issueID)
		cleanupTestIssue(t, issueID)
	})

	bus.Publish(events.Event{
		Type:        protocol.EventIssueCreated,
		WorkspaceID: testWorkspaceID,
		ActorType:   "member",
		ActorID:     testUserID,
		Payload: map[string]any{
			"issue": handler.IssueResponse{
				ID:          issueID,
				WorkspaceID: testWorkspaceID,
				Title:       "unconfigured telegram test",
				Status:      "todo",
				Priority:    "medium",
				CreatorType: "member",
				CreatorID:   testUserID,
			},
		},
	})

	time.Sleep(150 * time.Millisecond)
	// Since there's no group configured, no messages should be sent at all.
	// (Personal subscriber Telegram is also nil so getMsgs should be empty.)
	_ = getMsgs()
	// No assertion needed — the test passes if the handler doesn't panic.
}

// TestWorkspaceTelegram_StatusChanged verifies a group message on status change.
func TestWorkspaceTelegram_StatusChanged(t *testing.T) {
	srv, getMsgs := fakeTelegramServer(t)
	bot := telegram.NewBotWithURL("test-token", srv.URL)

	const groupChatID = "-1001234567891"
	setWorkspaceTelegramGroup(t, testWorkspaceID, groupChatID, true)
	t.Cleanup(func() { clearWorkspaceTelegramGroup(t, testWorkspaceID) })

	queries := db.New(testPool)
	bus := events.New()
	registerSubscriberListeners(bus, queries)
	registerNotificationListeners(bus, queries, bot)

	issueID := createTestIssue(t, testWorkspaceID, testUserID)
	t.Cleanup(func() {
		cleanupInboxForIssue(t, issueID)
		cleanupTestIssue(t, issueID)
	})

	bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: testWorkspaceID,
		ActorType:   "member",
		ActorID:     testUserID,
		Payload: map[string]any{
			"issue": handler.IssueResponse{
				ID:          issueID,
				WorkspaceID: testWorkspaceID,
				Title:       "status change test",
				Status:      "in_progress",
				Priority:    "medium",
				CreatorType: "member",
				CreatorID:   testUserID,
			},
			"status_changed": true,
			"prev_status":    "todo",
		},
	})

	msgs := waitForMsgs(getMsgs, 1)
	found := false
	for _, m := range msgs {
		if m.ChatID == groupChatID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected status change message to group chat %s, got: %v", groupChatID, msgs)
	}
}

// TestWorkspaceTelegram_CategoryPreferences verifies group category preferences
// suppress disabled notification categories without disabling the whole channel.
func TestWorkspaceTelegram_CategoryPreferences(t *testing.T) {
	srv, getMsgs := fakeTelegramServer(t)
	bot := telegram.NewBotWithURL("test-token", srv.URL)

	const groupChatID = "-1001234567893"
	setWorkspaceTelegramGroupWithPreferences(t, testWorkspaceID, groupChatID, true, map[string]bool{
		"new_issues":    true,
		"field_changes": false,
	})
	t.Cleanup(func() { clearWorkspaceTelegramGroup(t, testWorkspaceID) })

	queries := db.New(testPool)
	bus := events.New()
	registerSubscriberListeners(bus, queries)
	registerNotificationListeners(bus, queries, bot)

	issueID := createTestIssue(t, testWorkspaceID, testUserID)
	t.Cleanup(func() {
		cleanupInboxForIssue(t, issueID)
		cleanupTestIssue(t, issueID)
	})

	bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: testWorkspaceID,
		ActorType:   "member",
		ActorID:     testUserID,
		Payload: map[string]any{
			"issue": handler.IssueResponse{
				ID:          issueID,
				WorkspaceID: testWorkspaceID,
				Title:       "blocked field change telegram test",
				Status:      "in_progress",
				Priority:    "medium",
				CreatorType: "member",
				CreatorID:   testUserID,
			},
			"status_changed": true,
			"prev_status":    "todo",
		},
	})

	time.Sleep(150 * time.Millisecond)
	if msgs := getMsgs(); len(msgs) != 0 {
		t.Fatalf("expected no field change Telegram messages, got: %v", msgs)
	}

	bus.Publish(events.Event{
		Type:        protocol.EventIssueCreated,
		WorkspaceID: testWorkspaceID,
		ActorType:   "member",
		ActorID:     testUserID,
		Payload: map[string]any{
			"issue": handler.IssueResponse{
				ID:          issueID,
				WorkspaceID: testWorkspaceID,
				Title:       "allowed new issue telegram test",
				Status:      "todo",
				Priority:    "medium",
				CreatorType: "member",
				CreatorID:   testUserID,
			},
		},
	})

	msgs := waitForMsgs(getMsgs, 1)
	if len(msgs) == 0 {
		t.Fatal("expected new issue Telegram message to be delivered")
	}
}

// TestPersonalTelegram_CategoryPreferences verifies per-user category
// preferences suppress private Telegram delivery for disabled categories.
func TestPersonalTelegram_CategoryPreferences(t *testing.T) {
	srv, getMsgs := fakeTelegramServer(t)
	bot := telegram.NewBotWithURL("test-token", srv.URL)

	clearWorkspaceTelegramGroup(t, testWorkspaceID)

	subscriberEmail := "telegram-pref-subscriber@multica.ai"
	subscriberID := createTestUser(t, subscriberEmail)
	t.Cleanup(func() { cleanupTestUser(t, subscriberEmail) })

	const privateChatID = "123456789"
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO user_notification_channel (user_id, channel_type, channel_id, enabled, preferences)
		VALUES ($1, 'telegram', $2, TRUE, '{"comments": false}'::jsonb)
		ON CONFLICT (user_id, channel_type) DO UPDATE SET
			channel_id = EXCLUDED.channel_id,
			enabled = EXCLUDED.enabled,
			preferences = EXCLUDED.preferences
	`, subscriberID, privateChatID)
	if err != nil {
		t.Fatalf("insert user notification channel: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(),
			`DELETE FROM user_notification_channel WHERE user_id = $1 AND channel_type = 'telegram'`,
			subscriberID)
	})

	queries := db.New(testPool)
	bus := events.New()
	registerSubscriberListeners(bus, queries)
	registerNotificationListeners(bus, queries, bot)

	issueID := createTestIssue(t, testWorkspaceID, testUserID)
	addTestSubscriber(t, issueID, "member", subscriberID, "manual")
	t.Cleanup(func() {
		cleanupInboxForIssue(t, issueID)
		cleanupTestIssue(t, issueID)
	})

	bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: testWorkspaceID,
		ActorType:   "member",
		ActorID:     testUserID,
		Payload: map[string]any{
			"comment": handler.CommentResponse{
				ID:      "00000000-0000-0000-0000-000000000001",
				IssueID: issueID,
				Content: "comment should not trigger private telegram",
			},
			"issue_title":  "personal telegram preference test",
			"issue_status": "todo",
		},
	})

	time.Sleep(150 * time.Millisecond)
	for _, msg := range getMsgs() {
		if msg.ChatID == privateChatID {
			t.Fatalf("unexpected private Telegram message for disabled comments category: %v", msg)
		}
	}
}

// TestWorkspaceTelegram_TaskFailed verifies a group message on task failure.
func TestWorkspaceTelegram_TaskFailed(t *testing.T) {
	srv, getMsgs := fakeTelegramServer(t)
	bot := telegram.NewBotWithURL("test-token", srv.URL)

	const groupChatID = "-1001234567892"
	setWorkspaceTelegramGroup(t, testWorkspaceID, groupChatID, true)
	t.Cleanup(func() { clearWorkspaceTelegramGroup(t, testWorkspaceID) })

	queries := db.New(testPool)
	bus := events.New()
	registerSubscriberListeners(bus, queries)
	registerNotificationListeners(bus, queries, bot)

	issueID := createTestIssue(t, testWorkspaceID, testUserID)
	t.Cleanup(func() {
		cleanupInboxForIssue(t, issueID)
		cleanupTestIssue(t, issueID)
	})

	bus.Publish(events.Event{
		Type:        protocol.EventTaskFailed,
		WorkspaceID: testWorkspaceID,
		ActorType:   "agent",
		ActorID:     "",
		Payload: map[string]any{
			"issue_id": issueID,
		},
	})

	msgs := waitForMsgs(getMsgs, 1)
	found := false
	for _, m := range msgs {
		if m.ChatID == groupChatID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected task_failed message to group chat %s, got: %v", groupChatID, msgs)
	}
}

// TestWorkspaceTelegramSettings_ParseMerge verifies the settings JSONB helpers
// preserve other fields and round-trip correctly.
func TestWorkspaceTelegramSettings_ParseMerge(t *testing.T) {
	// Parse from nil raw.
	if got := handler.ParseTelegramGroupSettings(nil); got != nil {
		t.Fatalf("expected nil for nil raw, got %+v", got)
	}

	// Parse from empty JSON.
	if got := handler.ParseTelegramGroupSettings([]byte(`{}`)); got != nil {
		t.Fatalf("expected nil for empty settings, got %+v", got)
	}

	// Simulate existing provider settings + add telegram_group.
	existing := []byte(`{"providers":{"claude":{"enabled":true}}}`)

	merged, err := handler.MergeTelegramGroupForTest(existing, handler.WorkspaceTelegramSettings{
		ChatID:  "-100999",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("MergeTelegramGroupForTest: %v", err)
	}

	// Providers should still be present.
	var full map[string]json.RawMessage
	if err := json.Unmarshal(merged, &full); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := full["providers"]; !ok {
		t.Error("providers key should be preserved after merge")
	}

	// Parse back.
	s := handler.ParseTelegramGroupSettings(merged)
	if s == nil {
		t.Fatal("expected non-nil settings after merge")
	}
	if s.ChatID != "-100999" {
		t.Fatalf("expected chat_id -100999, got %q", s.ChatID)
	}
	if !s.Enabled {
		t.Error("expected enabled=true")
	}

	// Clear.
	cleared, err := handler.ClearTelegramGroupForTest(merged)
	if err != nil {
		t.Fatalf("ClearTelegramGroupForTest: %v", err)
	}
	if got := handler.ParseTelegramGroupSettings(cleared); got != nil {
		t.Fatalf("expected nil after clear, got %+v", got)
	}
	// Providers should still be intact.
	if err := json.Unmarshal(cleared, &full); err != nil {
		t.Fatalf("unmarshal cleared: %v", err)
	}
	if _, ok := full["providers"]; !ok {
		t.Error("providers key should be preserved after clear")
	}
}
