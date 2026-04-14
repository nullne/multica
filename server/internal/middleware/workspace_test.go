package middleware

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

func TestMemberFromContext_Present(t *testing.T) {
	member := db.Member{
		ID:   pgtype.UUID{Valid: true},
		Role: "admin",
	}
	ctx := context.WithValue(context.Background(), ctxKeyMember, member)

	got, ok := MemberFromContext(ctx)
	if !ok {
		t.Fatal("expected member to be present in context")
	}
	if got.Role != "admin" {
		t.Errorf("role = %q, want %q", got.Role, "admin")
	}
}

func TestMemberFromContext_Absent(t *testing.T) {
	_, ok := MemberFromContext(context.Background())
	if ok {
		t.Error("expected member to be absent from empty context")
	}
}

func TestWorkspaceIDFromContext_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKeyWorkspaceID, "ws-123")
	got := WorkspaceIDFromContext(ctx)
	if got != "ws-123" {
		t.Errorf("workspace_id = %q, want %q", got, "ws-123")
	}
}

func TestWorkspaceIDFromContext_Absent(t *testing.T) {
	got := WorkspaceIDFromContext(context.Background())
	if got != "" {
		t.Errorf("workspace_id = %q, want empty string", got)
	}
}

func TestSetMemberContext(t *testing.T) {
	member := db.Member{
		Role: "member",
	}
	ctx := SetMemberContext(context.Background(), "ws-456", member)

	gotWS := WorkspaceIDFromContext(ctx)
	if gotWS != "ws-456" {
		t.Errorf("workspace_id = %q, want %q", gotWS, "ws-456")
	}

	gotMember, ok := MemberFromContext(ctx)
	if !ok {
		t.Fatal("expected member to be present")
	}
	if gotMember.Role != "member" {
		t.Errorf("role = %q, want %q", gotMember.Role, "member")
	}
}

func TestSetMemberContext_OverwritesPrevious(t *testing.T) {
	member1 := db.Member{Role: "admin"}
	ctx := SetMemberContext(context.Background(), "ws-1", member1)

	member2 := db.Member{Role: "viewer"}
	ctx = SetMemberContext(ctx, "ws-2", member2)

	gotWS := WorkspaceIDFromContext(ctx)
	if gotWS != "ws-2" {
		t.Errorf("workspace_id = %q, want %q", gotWS, "ws-2")
	}

	gotMember, _ := MemberFromContext(ctx)
	if gotMember.Role != "viewer" {
		t.Errorf("role = %q, want %q", gotMember.Role, "viewer")
	}
}
