// Package cron parses 5-field cron expressions and computes next-run times.
// Format: "minute hour dom month dow" (e.g. "0 9 * * 1" = every Monday at 09:00).
//
// Field ranges:
//   - minute:  0–59
//   - hour:    0–23
//   - dom:     1–31
//   - month:   1–12
//   - dow:     0–7  (0 and 7 both = Sunday)
//
// Supported syntax per field: *, n, n-m, */n, n-m/n, comma-separated combinations.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule holds a parsed cron expression.
type Schedule struct {
	minute  [60]bool
	hour    [24]bool
	dom     [32]bool // index 1–31
	month   [13]bool // index 1–12
	dow     [8]bool  // index 0–7; 0 and 7 both map to Sunday
	domStar bool
	dowStar bool
}

// Parse parses a 5-field cron expression.
func Parse(expr string) (*Schedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron: expected 5 fields, got %d in %q", len(fields), expr)
	}

	var s Schedule
	var err error

	if err = fillField(s.minute[:], fields[0], 0, 59); err != nil {
		return nil, fmt.Errorf("cron minute field %q: %w", fields[0], err)
	}
	if err = fillField(s.hour[:], fields[1], 0, 23); err != nil {
		return nil, fmt.Errorf("cron hour field %q: %w", fields[1], err)
	}
	if err = fillField(s.dom[:], fields[2], 1, 31); err != nil {
		return nil, fmt.Errorf("cron dom field %q: %w", fields[2], err)
	}
	s.domStar = fields[2] == "*"

	if err = fillField(s.month[:], fields[3], 1, 12); err != nil {
		return nil, fmt.Errorf("cron month field %q: %w", fields[3], err)
	}
	if err = fillField(s.dow[:], fields[4], 0, 7); err != nil {
		return nil, fmt.Errorf("cron dow field %q: %w", fields[4], err)
	}
	s.dowStar = fields[4] == "*"
	// Normalize: 7 = Sunday = 0
	if s.dow[7] {
		s.dow[0] = true
	}

	return &s, nil
}

// Next returns the next activation time after from, in from's location.
// Returns zero time if no match is found within 5 years (invalid expression).
func (s *Schedule) Next(from time.Time) time.Time {
	// Advance past the current second; align to minute boundary.
	t := from.Add(time.Minute).Truncate(time.Minute)
	limit := t.AddDate(5, 0, 0)

	for t.Before(limit) {
		// --- month ---
		m := int(t.Month())
		if !s.month[m] {
			// Jump to the 1st of the next month at 00:00.
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}

		// --- day-of-month / day-of-week ---
		if !s.matchDay(t) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}

		// --- hour ---
		h := t.Hour()
		if !s.hour[h] {
			t = time.Date(t.Year(), t.Month(), t.Day(), h+1, 0, 0, 0, t.Location())
			continue
		}

		// --- minute ---
		min := t.Minute()
		if !s.minute[min] {
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	return time.Time{}
}

// matchDay applies POSIX cron rules:
//   - if both DOM and DOW are unrestricted (*), every day matches
//   - if only one is restricted, that one must match
//   - if both are restricted, either matching is sufficient (OR)
func (s *Schedule) matchDay(t time.Time) bool {
	if s.domStar && s.dowStar {
		return true
	}
	if s.domStar {
		return s.dow[int(t.Weekday())]
	}
	if s.dowStar {
		return s.dom[t.Day()]
	}
	return s.dom[t.Day()] || s.dow[int(t.Weekday())]
}

// fillField parses a cron field expression and marks the matching indices in bits.
// bits must be at least max+1 in length; index 0 is unused for 1-based fields.
func fillField(bits []bool, expr string, min, max int) error {
	for _, part := range strings.Split(expr, ",") {
		if err := fillPart(bits, part, min, max); err != nil {
			return err
		}
	}
	return nil
}

func fillPart(bits []bool, part string, min, max int) error {
	// Split on "/" for step value.
	step := 1
	if idx := strings.Index(part, "/"); idx >= 0 {
		var err error
		step, err = strconv.Atoi(part[idx+1:])
		if err != nil || step <= 0 {
			return fmt.Errorf("invalid step in %q", part)
		}
		part = part[:idx]
	}

	// Determine range.
	var lo, hi int
	switch {
	case part == "*":
		lo, hi = min, max
	case strings.Contains(part, "-"):
		idx := strings.Index(part, "-")
		var err error
		lo, err = strconv.Atoi(part[:idx])
		if err != nil {
			return fmt.Errorf("invalid range start in %q", part)
		}
		hi, err = strconv.Atoi(part[idx+1:])
		if err != nil {
			return fmt.Errorf("invalid range end in %q", part)
		}
	default:
		val, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("invalid value %q", part)
		}
		lo, hi = val, val
	}

	if lo < min || hi > max || lo > hi {
		return fmt.Errorf("value %d-%d out of range [%d, %d]", lo, hi, min, max)
	}
	if step == 0 {
		return fmt.Errorf("step must be > 0")
	}

	for v := lo; v <= hi; v += step {
		bits[v] = true
	}
	return nil
}
