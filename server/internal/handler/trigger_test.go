package handler

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// Helper to build a pgtype.UUID from a string.
func testUUID(s string) pgtype.UUID {
	return parseUUID(s)
}

// Helper to build a pgtype.Text.
func testText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

const (
	agentAssigneeID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	otherAgentID    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	memberID        = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	otherMemberID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
)

func issueWithAgentAssignee() db.Issue {
	return db.Issue{
		AssigneeType: testText("agent"),
		AssigneeID:   testUUID(agentAssigneeID),
	}
}

func issueNoAssignee() db.Issue {
	return db.Issue{}
}

// -------------------------------------------------------------------
// commentMentionsOthersButNotAssignee
// -------------------------------------------------------------------

func TestCommentMentionsOthersButNotAssignee(t *testing.T) {
	h := &Handler{} // nil handler — method doesn't use h

	issue := issueWithAgentAssignee()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "no mentions → allow trigger",
			content: "just a plain comment",
			want:    false,
		},
		{
			name:    "mentions assignee → allow trigger",
			content: fmt.Sprintf("[@Agent](mention://agent/%s) please fix", agentAssigneeID),
			want:    false,
		},
		{
			name:    "mentions other agent only → suppress",
			content: fmt.Sprintf("[@Other](mention://agent/%s) what do you think?", otherAgentID),
			want:    true,
		},
		{
			name:    "mentions other member only → suppress",
			content: fmt.Sprintf("[@Bob](mention://member/%s) take a look", memberID),
			want:    true,
		},
		{
			name:    "mentions both assignee and other → allow trigger",
			content: fmt.Sprintf("[@Agent](mention://agent/%s) and [@Other](mention://agent/%s)", agentAssigneeID, otherAgentID),
			want:    false,
		},
		{
			name:    "@all mention → suppress (broadcast, not directed at agent)",
			content: "[@All](mention://all/all) heads up everyone",
			want:    true,
		},
		{
			name:    "@all with assignee mention → suppress (@all takes precedence)",
			content: fmt.Sprintf("[@All](mention://all/all) [@Agent](mention://agent/%s) fyi", agentAssigneeID),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.commentMentionsOthersButNotAssignee(tt.content, issue)
			if got != tt.want {
				t.Errorf("commentMentionsOthersButNotAssignee() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommentMentionsOthersButNotAssignee_NoAssignee(t *testing.T) {
	h := &Handler{}
	issue := issueNoAssignee()

	// Any mention on an unassigned issue → suppress
	content := fmt.Sprintf("[@Agent](mention://agent/%s) help", otherAgentID)
	if got := h.commentMentionsOthersButNotAssignee(content, issue); !got {
		t.Errorf("expected true for mentions on unassigned issue, got false")
	}
}

// -------------------------------------------------------------------
// isReplyToMemberThread
// -------------------------------------------------------------------

func TestIsReplyToMemberThread(t *testing.T) {
	h := &Handler{}
	issue := issueWithAgentAssignee()

	memberParent := &db.Comment{AuthorType: "member", AuthorID: testUUID(memberID)}
	agentParent := &db.Comment{AuthorType: "agent", AuthorID: testUUID(agentAssigneeID)}

	tests := []struct {
		name    string
		parent  *db.Comment
		content string
		want    bool
	}{
		{
			name:    "top-level comment (nil parent) → allow",
			parent:  nil,
			content: "a comment",
			want:    false,
		},
		{
			name:    "reply to agent thread, no mentions → allow",
			parent:  agentParent,
			content: "sounds good",
			want:    false,
		},
		{
			name:    "reply to agent thread, mention other member → allow (handled by other check)",
			parent:  agentParent,
			content: fmt.Sprintf("[@Bob](mention://member/%s) thoughts?", memberID),
			want:    false, // isReplyToMemberThread only checks member threads
		},
		{
			name:    "reply to member thread, no mentions → suppress",
			parent:  memberParent,
			content: "I agree with you",
			want:    true,
		},
		{
			name:    "reply to member thread, mention other member → suppress",
			parent:  memberParent,
			content: fmt.Sprintf("[@Alice](mention://member/%s) what about this?", otherMemberID),
			want:    true,
		},
		{
			name:    "reply to member thread, mention assignee agent → allow",
			parent:  memberParent,
			content: fmt.Sprintf("[@Agent](mention://agent/%s) can you help?", agentAssigneeID),
			want:    false,
		},
		{
			name:    "reply to member thread, mention other agent (not assignee) → suppress",
			parent:  memberParent,
			content: fmt.Sprintf("[@Other](mention://agent/%s) take a look", otherAgentID),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.isReplyToMemberThread(tt.parent, tt.content, issue)
			if got != tt.want {
				t.Errorf("isReplyToMemberThread() = %v, want %v", got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------
// Combined trigger decision (simulates the full on_comment check)
// -------------------------------------------------------------------

func TestOnCommentTriggerDecision(t *testing.T) {
	h := &Handler{}
	issue := issueWithAgentAssignee()

	memberParent := &db.Comment{AuthorType: "member", AuthorID: testUUID(memberID)}
	agentParent := &db.Comment{AuthorType: "agent", AuthorID: testUUID(agentAssigneeID)}

	// Simulates the combined check from CreateComment:
	//   !commentMentionsOthersButNotAssignee && !isReplyToMemberThread
	shouldTrigger := func(parent *db.Comment, content string) bool {
		return !h.commentMentionsOthersButNotAssignee(content, issue) &&
			!h.isReplyToMemberThread(parent, content, issue)
	}

	tests := []struct {
		name    string
		parent  *db.Comment
		content string
		want    bool
	}{
		{"top-level, no mention", nil, "hello agent", true},
		{"top-level, mention assignee", nil, fmt.Sprintf("[@Agent](mention://agent/%s) fix this", agentAssigneeID), true},
		{"top-level, mention other only", nil, fmt.Sprintf("[@Other](mention://agent/%s) look", otherAgentID), false},
		{"reply agent thread, no mention", agentParent, "got it", true},
		{"reply agent thread, mention other member", agentParent, fmt.Sprintf("[@Bob](mention://member/%s) ?", memberID), false},
		{"reply agent thread, mention assignee", agentParent, fmt.Sprintf("[@Agent](mention://agent/%s) yes", agentAssigneeID), true},
		{"reply member thread, no mention", memberParent, "agreed", false},
		{"reply member thread, mention other member", memberParent, fmt.Sprintf("[@Bob](mention://member/%s) ok", memberID), false},
		{"reply member thread, mention assignee", memberParent, fmt.Sprintf("[@Agent](mention://agent/%s) help", agentAssigneeID), true},
		{"top-level, @all broadcast", nil, "[@All](mention://all/all) heads up team", false},
		{"reply agent thread, @all broadcast", agentParent, "[@All](mention://all/all) update for everyone", false},
		{"reply member thread, @all broadcast", memberParent, "[@All](mention://all/all) fyi", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldTrigger(tt.parent, tt.content)
			if got != tt.want {
				t.Errorf("shouldTrigger() = %v, want %v", got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------
// agentHasTriggerEnabled
// -------------------------------------------------------------------

func TestAgentHasTriggerEnabled(t *testing.T) {
	tests := []struct {
		name        string
		raw         []byte
		triggerType string
		want        bool
	}{
		{
			name:        "nil triggers → enabled (backwards compat)",
			raw:         nil,
			triggerType: "on_comment",
			want:        true,
		},
		{
			name:        "empty byte slice → enabled",
			raw:         []byte{},
			triggerType: "on_comment",
			want:        true,
		},
		{
			name:        "empty JSON array → enabled (backwards compat)",
			raw:         []byte("[]"),
			triggerType: "on_comment",
			want:        true,
		},
		{
			name:        "on_comment explicitly enabled",
			raw:         mustJSON([]agentTriggerSnapshot{{Type: "on_comment", Enabled: true}}),
			triggerType: "on_comment",
			want:        true,
		},
		{
			name:        "on_comment explicitly disabled",
			raw:         mustJSON([]agentTriggerSnapshot{{Type: "on_comment", Enabled: false}}),
			triggerType: "on_comment",
			want:        false,
		},
		{
			name:        "on_mention not configured but others are → enabled by default",
			raw:         mustJSON([]agentTriggerSnapshot{{Type: "on_comment", Enabled: true}}),
			triggerType: "on_mention",
			want:        true,
		},
		{
			name:        "invalid JSON → disabled (fail safe)",
			raw:         []byte("{bad json"),
			triggerType: "on_comment",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agentHasTriggerEnabled(tt.raw, tt.triggerType)
			if got != tt.want {
				t.Errorf("agentHasTriggerEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------
// defaultAgentTriggers
// -------------------------------------------------------------------

func TestDefaultAgentTriggers(t *testing.T) {
	raw := defaultAgentTriggers()

	var triggers []agentTriggerSnapshot
	if err := json.Unmarshal(raw, &triggers); err != nil {
		t.Fatalf("failed to unmarshal default triggers: %v", err)
	}

	if len(triggers) != 3 {
		t.Fatalf("expected 3 default triggers, got %d", len(triggers))
	}

	expected := map[string]bool{
		"on_assign":  true,
		"on_comment": true,
		"on_mention": true,
	}
	for _, tr := range triggers {
		want, ok := expected[tr.Type]
		if !ok {
			t.Errorf("unexpected trigger type: %s", tr.Type)
			continue
		}
		if tr.Enabled != want {
			t.Errorf("trigger %s: enabled = %v, want %v", tr.Type, tr.Enabled, want)
		}
		delete(expected, tr.Type)
	}
	for typ := range expected {
		t.Errorf("missing trigger type: %s", typ)
	}

	// Verify all triggers are enabled via agentHasTriggerEnabled
	for _, typ := range []string{"on_assign", "on_comment", "on_mention"} {
		if !agentHasTriggerEnabled(raw, typ) {
			t.Errorf("agentHasTriggerEnabled(default, %q) = false, want true", typ)
		}
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
