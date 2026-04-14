package util

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestParseUUID_Valid(t *testing.T) {
	input := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	u := ParseUUID(input)
	if !u.Valid {
		t.Fatal("expected valid UUID")
	}
}

func TestParseUUID_Invalid(t *testing.T) {
	u := ParseUUID("not-a-uuid")
	if u.Valid {
		t.Error("expected invalid UUID for bad input")
	}
}

func TestParseUUID_Empty(t *testing.T) {
	u := ParseUUID("")
	if u.Valid {
		t.Error("expected invalid UUID for empty string")
	}
}

func TestUUIDToString_Valid(t *testing.T) {
	input := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	u := ParseUUID(input)
	result := UUIDToString(u)
	if result != input {
		t.Errorf("UUIDToString = %q, want %q", result, input)
	}
}

func TestUUIDToString_Invalid(t *testing.T) {
	u := pgtype.UUID{Valid: false}
	result := UUIDToString(u)
	if result != "" {
		t.Errorf("UUIDToString = %q, want empty string", result)
	}
}

func TestUUIDRoundTrip(t *testing.T) {
	ids := []string{
		"00000000-0000-0000-0000-000000000000",
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
	}
	for _, id := range ids {
		u := ParseUUID(id)
		got := UUIDToString(u)
		if got != id {
			t.Errorf("round-trip failed: input=%q, output=%q", id, got)
		}
	}
}

func TestTextToPtr_Valid(t *testing.T) {
	text := pgtype.Text{String: "hello", Valid: true}
	ptr := TextToPtr(text)
	if ptr == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *ptr != "hello" {
		t.Errorf("got %q, want %q", *ptr, "hello")
	}
}

func TestTextToPtr_Invalid(t *testing.T) {
	text := pgtype.Text{Valid: false}
	ptr := TextToPtr(text)
	if ptr != nil {
		t.Errorf("expected nil for invalid text, got %q", *ptr)
	}
}

func TestPtrToText_NonNil(t *testing.T) {
	s := "hello"
	text := PtrToText(&s)
	if !text.Valid {
		t.Fatal("expected valid text")
	}
	if text.String != "hello" {
		t.Errorf("got %q, want %q", text.String, "hello")
	}
}

func TestPtrToText_Nil(t *testing.T) {
	text := PtrToText(nil)
	if text.Valid {
		t.Error("expected invalid text for nil pointer")
	}
}

func TestStrToText_NonEmpty(t *testing.T) {
	text := StrToText("hello")
	if !text.Valid {
		t.Fatal("expected valid text")
	}
	if text.String != "hello" {
		t.Errorf("got %q, want %q", text.String, "hello")
	}
}

func TestStrToText_Empty(t *testing.T) {
	text := StrToText("")
	if text.Valid {
		t.Error("expected invalid text for empty string")
	}
}

func TestTimestampToString_Valid(t *testing.T) {
	ts := pgtype.Timestamptz{
		Time:  time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Valid: true,
	}
	result := TimestampToString(ts)
	expected := "2026-01-15T10:30:00Z"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestTimestampToString_Invalid(t *testing.T) {
	ts := pgtype.Timestamptz{Valid: false}
	result := TimestampToString(ts)
	if result != "" {
		t.Errorf("got %q, want empty string", result)
	}
}

func TestTimestampToPtr_Valid(t *testing.T) {
	ts := pgtype.Timestamptz{
		Time:  time.Date(2026, 3, 20, 15, 0, 0, 0, time.UTC),
		Valid: true,
	}
	ptr := TimestampToPtr(ts)
	if ptr == nil {
		t.Fatal("expected non-nil pointer")
	}
	expected := "2026-03-20T15:00:00Z"
	if *ptr != expected {
		t.Errorf("got %q, want %q", *ptr, expected)
	}
}

func TestTimestampToPtr_Invalid(t *testing.T) {
	ts := pgtype.Timestamptz{Valid: false}
	ptr := TimestampToPtr(ts)
	if ptr != nil {
		t.Error("expected nil for invalid timestamp")
	}
}

func TestUUIDToPtr_Valid(t *testing.T) {
	u := ParseUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	ptr := UUIDToPtr(u)
	if ptr == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *ptr != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("got %q, want %q", *ptr, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	}
}

func TestUUIDToPtr_Invalid(t *testing.T) {
	u := pgtype.UUID{Valid: false}
	ptr := UUIDToPtr(u)
	if ptr != nil {
		t.Error("expected nil for invalid UUID")
	}
}
