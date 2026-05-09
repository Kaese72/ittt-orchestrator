package restmodels_test

import (
	"testing"
	"time"

	"github.com/Kaese72/ittt-orchestrator/restmodels"
)

// TestTimeRangeDaysCondition_InvalidFrom verifies that an unparseable "from" timestamp
// causes Evaluate to return false with a non-empty reason. NextOccurrence must be nil.
func TestTimeRangeDaysCondition_InvalidFrom(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{Type: "time-range-days", From: "not-a-time", To: "22:00:00", Timezone: "UTC", Days: []string{"monday"}}
	result := cond.Evaluate(stubEvalContext{now: time.Now()})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason")
	}
	assertNextOccurrence(t, result, nil)
}

// TestTimeRangeDaysCondition_InvalidTo verifies that an unparseable "to" timestamp
// causes Evaluate to return false with a non-empty reason. NextOccurrence must be nil.
func TestTimeRangeDaysCondition_InvalidTo(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{Type: "time-range-days", From: "06:00:00", To: "not-a-time", Timezone: "UTC", Days: []string{"monday"}}
	result := cond.Evaluate(stubEvalContext{now: time.Now()})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason")
	}
	assertNextOccurrence(t, result, nil)
}

// TestTimeRangeDaysCondition_InvalidTimezone verifies that an unrecognised timezone
// causes Evaluate to return false with a non-empty reason. NextOccurrence must be nil.
func TestTimeRangeDaysCondition_InvalidTimezone(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{Type: "time-range-days", From: "06:00:00", To: "22:00:00", Timezone: "Not/Real", Days: []string{"monday"}}
	result := cond.Evaluate(stubEvalContext{now: time.Now()})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason")
	}
	assertNextOccurrence(t, result, nil)
}

// TestTimeRangeDaysCondition_WithinRangeOnMatchingDay verifies that a time falling
// inside the range on a whitelisted day evaluates to true. NextOccurrence must be the
// "to" boundary on the same day.
//
// 2024-01-15 is a Monday.
func TestTimeRangeDaysCondition_WithinRangeOnMatchingDay(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "06:00:00", To: "22:00:00", Timezone: "UTC",
		Days: []string{"monday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)})
	if !result.Result {
		t.Errorf("expected true result, got false (reason: %s)", result.Reason)
	}
	want := time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeDaysCondition_WithinRangeOnNonMatchingDay verifies that a time falling
// inside the range on a day NOT in the whitelist evaluates to false. NextOccurrence must
// be the next "from" on a matching day.
//
// 2024-01-16 is a Tuesday; days=["monday"] so next match is Monday 2024-01-22.
func TestTimeRangeDaysCondition_WithinRangeOnNonMatchingDay(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "06:00:00", To: "22:00:00", Timezone: "UTC",
		Days: []string{"monday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	want := time.Date(2024, 1, 22, 6, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeDaysCondition_BeforeRangeOnMatchingDay verifies that a time before
// the "from" boundary on a whitelisted day evaluates to false. NextOccurrence must be
// "from" on the same day.
//
// 2024-01-15 is a Monday.
func TestTimeRangeDaysCondition_BeforeRangeOnMatchingDay(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "10:00:00", To: "20:00:00", Timezone: "UTC",
		Days: []string{"monday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	want := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeDaysCondition_BeforeRangeOnNonMatchingDay verifies that a time before
// the "from" boundary on a non-matching day evaluates to false. NextOccurrence must be
// "from" on the next matching day.
//
// 2024-01-16 is a Tuesday; days=["monday"] so next match is Monday 2024-01-22.
func TestTimeRangeDaysCondition_BeforeRangeOnNonMatchingDay(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "10:00:00", To: "20:00:00", Timezone: "UTC",
		Days: []string{"monday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 16, 9, 0, 0, 0, time.UTC)})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	want := time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeDaysCondition_AfterRangeOnMatchingDay verifies that a time after the
// "to" boundary on a whitelisted day evaluates to false. NextOccurrence must be "from"
// on the next occurrence of a matching day (next week).
//
// 2024-01-15 is a Monday; after 20:00 the window has closed, so next match is 2024-01-22.
func TestTimeRangeDaysCondition_AfterRangeOnMatchingDay(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "10:00:00", To: "20:00:00", Timezone: "UTC",
		Days: []string{"monday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 21, 0, 0, 0, time.UTC)})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	want := time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeDaysCondition_CrossMidnightEveningSideMatchingDay verifies the evening
// segment of a midnight-crossing range on a whitelisted anchor day. NextOccurrence must
// be "to" the following morning.
//
// 2024-01-15 is a Monday. Range 22:00–02:00, days=["monday"]. Now=Monday 23:00.
// Window runs Monday 22:00 → Tuesday 02:00. Anchor is Monday → matches.
func TestTimeRangeDaysCondition_CrossMidnightEveningSideMatchingDay(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "22:00:00", To: "02:00:00", Timezone: "UTC",
		Days: []string{"monday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 23, 0, 0, 0, time.UTC)})
	if !result.Result {
		t.Errorf("expected true result, got false (reason: %s)", result.Reason)
	}
	want := time.Date(2024, 1, 16, 2, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeDaysCondition_CrossMidnightMorningSideMatchingDay verifies the morning
// segment of a midnight-crossing range when the anchor day (yesterday) is in the whitelist.
// NextOccurrence must be "to" today (the end of the current window).
//
// 2024-01-16 is a Tuesday. Range 22:00–02:00, days=["monday"]. Now=Tuesday 01:00.
// The active window started on Monday (anchor) at 22:00 and ends Tuesday 02:00. Monday matches.
func TestTimeRangeDaysCondition_CrossMidnightMorningSideMatchingDay(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "22:00:00", To: "02:00:00", Timezone: "UTC",
		Days: []string{"monday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 16, 1, 0, 0, 0, time.UTC)})
	if !result.Result {
		t.Errorf("expected true result, got false (reason: %s)", result.Reason)
	}
	want := time.Date(2024, 1, 16, 2, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeDaysCondition_CrossMidnightMorningSideNonMatchingDay verifies that the
// morning segment of a midnight-crossing range evaluates to false when the anchor day
// (yesterday) is NOT in the whitelist. NextOccurrence must be "from" on the next
// matching day.
//
// 2024-01-16 is a Tuesday. Range 22:00–02:00, days=["friday"]. Now=Tuesday 01:00.
// The window started on Monday (not in days=["friday"]) → false.
// Next Friday 22:00 is 2024-01-19.
func TestTimeRangeDaysCondition_CrossMidnightMorningSideNonMatchingDay(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "22:00:00", To: "02:00:00", Timezone: "UTC",
		Days: []string{"friday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 16, 1, 0, 0, 0, time.UTC)})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	want := time.Date(2024, 1, 19, 22, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeDaysCondition_MultiDayConsecutiveBlock verifies that when multiple
// consecutive days are enabled, NextOccurrence when true points past the entire block,
// not just to the next midnight.
//
// days=["monday","tuesday","wednesday"], now=Monday 12:00, range 06:00–22:00.
// The condition is true on Monday. The window ends Monday 22:00, but Tuesday and
// Wednesday are also active. However, each individual window ends at 22:00 on its own
// day — the condition re-evaluates and stays true. NextOccurrence is Monday 22:00
// (when this particular window closes; the scheduler re-evaluates from there).
//
// Note: consecutive-day merging for next_occurrence is a property of the condition tree
// scheduler, not of the individual condition. Each window emits its own "to" boundary.
// This test confirms the per-window behaviour.
func TestTimeRangeDaysCondition_MultiDayWindowEndsAtCurrentDayTo(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "06:00:00", To: "22:00:00", Timezone: "UTC",
		Days: []string{"monday", "tuesday", "wednesday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)})
	if !result.Result {
		t.Errorf("expected true result, got false (reason: %s)", result.Reason)
	}
	// NextOccurrence is the end of the current window (Monday 22:00), not Wednesday 22:00.
	want := time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeDaysCondition_WithDaylightSaving verifies that DST is applied correctly.
//
// Range 10:00–20:00, timezone America/New_York, days=["monday"].
// 2024-07-15 is a Monday. Evaluation time: 14:30 UTC (summer, EDT active → UTC-4).
// Local time: 10:30 → inside range. NextOccurrence must be 20:00 EDT = 00:00 UTC on 2024-07-16.
func TestTimeRangeDaysCondition_WithDaylightSaving(t *testing.T) {
	cond := restmodels.TimeRangeDaysCondition{
		Type: "time-range-days", From: "10:00:00", To: "20:00:00", Timezone: "America/New_York",
		Days: []string{"monday"},
	}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 7, 15, 14, 30, 0, 0, time.UTC)})
	if !result.Result {
		t.Errorf("expected true result with DST applied, got false (reason: %s)", result.Reason)
	}
	want := time.Date(2024, 7, 16, 0, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}
