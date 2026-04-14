package util

import (
	"testing"
)

func TestParseMentions_SingleMember(t *testing.T) {
	content := `Hey [@Alice](mention://member/a1b2c3d4-e5f6-7890-abcd-ef1234567890) can you check this?`
	mentions := ParseMentions(content)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(mentions))
	}
	if mentions[0].Type != "member" {
		t.Errorf("type = %q, want %q", mentions[0].Type, "member")
	}
	if mentions[0].ID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("id = %q, want %q", mentions[0].ID, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	}
}

func TestParseMentions_AgentMention(t *testing.T) {
	content := `Assigning [@Bot](mention://agent/11111111-2222-3333-4444-555555555555)`
	mentions := ParseMentions(content)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(mentions))
	}
	if mentions[0].Type != "agent" {
		t.Errorf("type = %q, want %q", mentions[0].Type, "agent")
	}
}

func TestParseMentions_IssueMentionNoAt(t *testing.T) {
	// Issue mentions use [MUL-123](mention://issue/id) without @
	content := `See [MUL-42](mention://issue/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee) for details`
	mentions := ParseMentions(content)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(mentions))
	}
	if mentions[0].Type != "issue" {
		t.Errorf("type = %q, want %q", mentions[0].Type, "issue")
	}
}

func TestParseMentions_AllMention(t *testing.T) {
	content := `[@all](mention://all/all)`
	mentions := ParseMentions(content)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(mentions))
	}
	if mentions[0].Type != "all" {
		t.Errorf("type = %q, want %q", mentions[0].Type, "all")
	}
	if mentions[0].ID != "all" {
		t.Errorf("id = %q, want %q", mentions[0].ID, "all")
	}
}

func TestParseMentions_MultipleMentions(t *testing.T) {
	content := `[@Alice](mention://member/11111111-1111-1111-1111-111111111111) and [@Bob](mention://member/22222222-2222-2222-2222-222222222222) should review`
	mentions := ParseMentions(content)
	if len(mentions) != 2 {
		t.Fatalf("expected 2 mentions, got %d", len(mentions))
	}
}

func TestParseMentions_Deduplication(t *testing.T) {
	content := `[@Alice](mention://member/11111111-1111-1111-1111-111111111111) and again [@Alice](mention://member/11111111-1111-1111-1111-111111111111)`
	mentions := ParseMentions(content)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 mention (deduped), got %d", len(mentions))
	}
}

func TestParseMentions_NoMentions(t *testing.T) {
	content := `Just a plain comment with no mentions`
	mentions := ParseMentions(content)
	if len(mentions) != 0 {
		t.Fatalf("expected 0 mentions, got %d", len(mentions))
	}
}

func TestParseMentions_EmptyContent(t *testing.T) {
	mentions := ParseMentions("")
	if len(mentions) != 0 {
		t.Fatalf("expected 0 mentions, got %d", len(mentions))
	}
}

func TestIsMentionAll(t *testing.T) {
	allMention := Mention{Type: "all", ID: "all"}
	if !allMention.IsMentionAll() {
		t.Error("expected IsMentionAll to return true for type=all")
	}

	memberMention := Mention{Type: "member", ID: "some-id"}
	if memberMention.IsMentionAll() {
		t.Error("expected IsMentionAll to return false for type=member")
	}
}

func TestHasMentionAll(t *testing.T) {
	mentions := []Mention{
		{Type: "member", ID: "11111111-1111-1111-1111-111111111111"},
		{Type: "all", ID: "all"},
	}
	if !HasMentionAll(mentions) {
		t.Error("expected HasMentionAll to return true")
	}
}

func TestHasMentionAll_NoAll(t *testing.T) {
	mentions := []Mention{
		{Type: "member", ID: "11111111-1111-1111-1111-111111111111"},
	}
	if HasMentionAll(mentions) {
		t.Error("expected HasMentionAll to return false")
	}
}

func TestHasMentionAll_Empty(t *testing.T) {
	if HasMentionAll(nil) {
		t.Error("expected HasMentionAll to return false for nil slice")
	}
}
