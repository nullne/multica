package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nullne/multica/server/internal/events"
	"github.com/nullne/multica/server/internal/handler"
	"github.com/nullne/multica/server/internal/realtime"
	"github.com/nullne/multica/server/pkg/protocol"
)

// registerListeners wires up event bus listeners for WS broadcasting.
// Personal events (inbox, invites) are sent only to the target user via
// SendToUser. All other events are broadcast to the workspace room.
func registerListeners(bus *events.Bus, hub *realtime.Hub) {
	// Personal events should NOT be broadcast to the whole workspace.
	personalEvents := map[string]bool{
		protocol.EventInboxNew:           true,
		protocol.EventInboxRead:          true,
		protocol.EventInboxArchived:      true,
		protocol.EventInboxBatchRead:     true,
		protocol.EventInboxBatchArchived: true,
	}

	// Helper: marshal event and send to a specific user within the event's workspace.
	sendToRecipient := func(hub *realtime.Hub, e events.Event, recipientID string) {
		if recipientID == "" {
			return
		}
		data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID})
		if err != nil {
			return
		}
		if e.WorkspaceID != "" {
			hub.SendToUserInWorkspace(recipientID, e.WorkspaceID, data)
		} else {
			hub.SendToUser(recipientID, data)
		}
	}

	// inbox:new — extract recipient from nested item
	bus.Subscribe(protocol.EventInboxNew, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		item, ok := payload["item"].(map[string]any)
		if !ok {
			return
		}
		recipientID, _ := item["recipient_id"].(string)
		sendToRecipient(hub, e, recipientID)
	})

	// inbox:read, inbox:archived, inbox:batch-read, inbox:batch-archived
	// — extract recipient from top-level payload
	for _, eventType := range []string{
		protocol.EventInboxRead, protocol.EventInboxArchived,
		protocol.EventInboxBatchRead, protocol.EventInboxBatchArchived,
	} {
		bus.Subscribe(eventType, func(e events.Event) {
			payload, ok := e.Payload.(map[string]any)
			if !ok {
				return
			}
			recipientID, _ := payload["recipient_id"].(string)
			sendToRecipient(hub, e, recipientID)
		})
	}

	// member:added — also send to the invited user so they discover the new workspace.
	// Pass excludeWorkspace so clients already in the target room (reached via
	// BroadcastToWorkspace in SubscribeAll) don't receive the event twice.
	bus.Subscribe(protocol.EventMemberAdded, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		var userID string
		switch m := payload["member"].(type) {
		case handler.MemberWithUserResponse:
			userID = m.UserID
		case map[string]any:
			userID, _ = m["user_id"].(string)
		default:
			slog.Warn("member:added: unexpected member payload type", "type", fmt.Sprintf("%T", payload["member"]))
		}
		if userID == "" {
			return
		}
		data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID})
		if err != nil {
			return
		}
		hub.SendToUser(userID, data, e.WorkspaceID)
	})

	// SubscribeAll handles workspace-broadcast for non-personal events.
	bus.SubscribeAll(func(e events.Event) {
		// Skip personal events — they are handled by type-specific listeners above.
		if personalEvents[e.Type] {
			return
		}

		msg := map[string]any{
			"type":     e.Type,
			"payload":  e.Payload,
			"actor_id": e.ActorID,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			slog.Error("failed to marshal event", "event_type", e.Type, "error", err)
			return
		}
		if e.WorkspaceID != "" {
			hub.BroadcastToWorkspace(e.WorkspaceID, data)
		} else if strings.HasPrefix(e.Type, "daemon:") {
			hub.Broadcast(data)
		}
		// Otherwise drop — no global broadcast for non-daemon events without a workspace.
	})
}
