package cron

import (
	"testing"
	"time"
)

func TestParse_InvalidExpressions(t *testing.T) {
	cases := []string{
		"",
		"* * * *",       // too few fields
		"* * * * * *",   // too many fields
		"60 * * * *",    // minute out of range
		"* 24 * * *",    // hour out of range
		"* * 0 * *",     // dom out of range (0 not valid)
		"* * * 13 *",    // month out of range
		"* * * * 8",     // dow out of range
		"* * * * */0",   // step zero
		"abc * * * *",   // non-numeric
	}
	for _, expr := range cases {
		_, err := Parse(expr)
		if err == nil {
			t.Errorf("Parse(%q) expected error, got nil", expr)
		}
	}
}

func TestParse_ValidExpressions(t *testing.T) {
	cases := []string{
		"* * * * *",
		"0 9 * * 1",
		"*/5 * * * *",
		"0 0 1 1 *",
		"0 12 * * 1-5",
		"30 6,12,18 * * *",
		"0 0 * * 0,6",
		"0 9 1-15 * *",
		"0 0 * * 7", // Sunday via 7
	}
	for _, expr := range cases {
		_, err := Parse(expr)
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", expr, err)
		}
	}
}

func mustParse(t *testing.T, expr string) *Schedule {
	t.Helper()
	s, err := Parse(expr)
	if err != nil {
		t.Fatalf("Parse(%q) failed: %v", expr, err)
	}
	return s
}

func TestNext_EveryMinute(t *testing.T) {
	s := mustParse(t, "* * * * *")
	now := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	next := s.Next(now)
	want := time.Date(2024, 1, 15, 10, 31, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}
}

func TestNext_EveryHourAtMinuteZero(t *testing.T) {
	s := mustParse(t, "0 * * * *")
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	next := s.Next(now)
	want := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}
}

func TestNext_DailyAt9AM(t *testing.T) {
	s := mustParse(t, "0 9 * * *")
	// Before 9 AM same day
	now := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	next := s.Next(now)
	want := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}

	// After 9 AM — should be next day
	now2 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	next2 := s.Next(now2)
	want2 := time.Date(2024, 1, 16, 9, 0, 0, 0, time.UTC)
	if !next2.Equal(want2) {
		t.Errorf("Next() = %v, want %v", next2, want2)
	}
}

func TestNext_WeekdayOnly(t *testing.T) {
	// Every weekday (Mon–Fri) at midnight
	s := mustParse(t, "0 0 * * 1-5")
	// 2024-01-13 is Saturday
	now := time.Date(2024, 1, 13, 12, 0, 0, 0, time.UTC)
	next := s.Next(now)
	// Next Monday is 2024-01-15
	want := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}
}

func TestNext_Sunday7EquivalentTo0(t *testing.T) {
	s0 := mustParse(t, "0 0 * * 0")
	s7 := mustParse(t, "0 0 * * 7")
	// 2024-01-14 is Sunday
	now := time.Date(2024, 1, 13, 12, 0, 0, 0, time.UTC) // Saturday
	if !s0.Next(now).Equal(s7.Next(now)) {
		t.Errorf("dow=0 and dow=7 should produce the same next time, got %v vs %v", s0.Next(now), s7.Next(now))
	}
}

func TestNext_MonthlyFirstOfMonth(t *testing.T) {
	s := mustParse(t, "0 9 1 * *")
	now := time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC)
	next := s.Next(now)
	want := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}
}

func TestNext_TimezoneAware(t *testing.T) {
	nyc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone data unavailable:", err)
	}
	s := mustParse(t, "0 9 * * *")
	// 08:00 NYC time
	now := time.Date(2024, 6, 15, 8, 0, 0, 0, nyc)
	next := s.Next(now)
	want := time.Date(2024, 6, 15, 9, 0, 0, 0, nyc)
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}
}

func TestNext_EveryFiveMinutes(t *testing.T) {
	s := mustParse(t, "*/5 * * * *")
	now := time.Date(2024, 1, 15, 10, 7, 0, 0, time.UTC)
	next := s.Next(now)
	want := time.Date(2024, 1, 15, 10, 10, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}
}

func TestNext_YearBoundary(t *testing.T) {
	s := mustParse(t, "0 0 1 1 *")
	now := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	next := s.Next(now)
	want := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}
}

func TestNext_AtCurrentMinuteBoundary(t *testing.T) {
	// Exactly at the top of a minute that matches — should return NEXT occurrence.
	s := mustParse(t, "0 10 * * *")
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	next := s.Next(now)
	want := time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}
}

func TestNext_BothDOMAndDOWSpecified(t *testing.T) {
	// "0 0 15 * 1" — OR: fires on 15th of any month OR every Monday
	s := mustParse(t, "0 0 15 * 1")
	// 2024-01-08 is Monday (not the 15th), should match
	now := time.Date(2024, 1, 7, 12, 0, 0, 0, time.UTC) // Sunday
	next := s.Next(now)
	want := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC) // Monday
	if !next.Equal(want) {
		t.Errorf("Next() = %v, want %v", next, want)
	}
}
