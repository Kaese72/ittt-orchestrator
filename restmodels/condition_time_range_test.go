package restmodels_test

import (
	"testing"
	"time"

	"github.com/Kaese72/ittt-orchestrator/restmodels"
)

// assertNextOccurrence checks that result.NextOccurrence matches want.
func assertNextOccurrence(t *testing.T, result restmodels.EvalResult, want *time.Time) {
	t.Helper()
	if want == nil {
		if result.NextOccurrence != nil {
			t.Errorf("expected NextOccurrence to be nil, got %s", result.NextOccurrence.UTC().Format(time.RFC3339))
		}
		return
	}
	if result.NextOccurrence == nil {
		t.Errorf("expected NextOccurrence %s, got nil", want.UTC().Format(time.RFC3339))
		return
	}
	if !result.NextOccurrence.Equal(*want) {
		t.Errorf("expected NextOccurrence %s, got %s",
			want.UTC().Format(time.RFC3339),
			result.NextOccurrence.UTC().Format(time.RFC3339))
	}
}

// TestTimeRangeCondition_InvalidFrom verifies that an unparseable "from" timestamp
// causes Evaluate to return false with a non-empty reason instead of panicking or
// silently succeeding. NextOccurrence must be nil because evaluation failed before
// any boundary could be computed.
func TestTimeRangeCondition_InvalidFrom(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "not-a-time", To: "22:00:00", Timezone: "UTC"}
	result := cond.Evaluate(stubEvalContext{now: time.Now()})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason")
	}
	assertNextOccurrence(t, result, nil)
}

// TestTimeRangeCondition_InvalidTo verifies that an unparseable "to" timestamp
// causes Evaluate to return false with a non-empty reason instead of panicking or
// silently succeeding. NextOccurrence must be nil because evaluation failed before
// any boundary could be computed.
func TestTimeRangeCondition_InvalidTo(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "06:00:00", To: "not-a-time", Timezone: "UTC"}
	result := cond.Evaluate(stubEvalContext{now: time.Now()})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason")
	}
	assertNextOccurrence(t, result, nil)
}

// TestTimeRangeCondition_WithinRangeSameDay verifies that a time falling between
// "from" and "to" on the same day evaluates to true. NextOccurrence must be the
// "to" boundary on the same day (22:00 UTC on 2024-01-15) because that is when
// the rule next exits the active window.
func TestTimeRangeCondition_WithinRangeSameDay(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "06:00:00", To: "22:00:00", Timezone: "UTC"}
	// 12:00 UTC is squarely within 06:00–22:00.
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)})
	if !result.Result {
		t.Errorf("expected true result, got false (reason: %s)", result.Reason)
	}
	// Exits the window at 22:00 UTC today.
	want := time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeCondition_BeforeRangeSameDay verifies that a time before the "from"
// boundary of a same-day range evaluates to false. NextOccurrence must be the "from"
// boundary on the same day (10:00 UTC on 2024-01-15) because that is when the window
// next opens.
func TestTimeRangeCondition_BeforeRangeSameDay(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "10:00:00", To: "20:00:00", Timezone: "UTC"}
	// 09:00 UTC is before the 10:00 start.
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	// Window opens at 10:00 UTC today.
	want := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeCondition_AfterRangeSameDay verifies that a time past the "to"
// boundary of a same-day range evaluates to false. NextOccurrence must be the "from"
// boundary on the following day (10:00 UTC on 2024-01-16) because today's window has
// already closed.
func TestTimeRangeCondition_AfterRangeSameDay(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "10:00:00", To: "20:00:00", Timezone: "UTC"}
	// 21:00 UTC is after the 20:00 end.
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 21, 0, 0, 0, time.UTC)})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	// Window next opens at 10:00 UTC tomorrow.
	want := time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeCondition_WithinRangeCrossMidnight verifies that a time in the
// evening portion of a midnight-crossing range (e.g. 22:00–06:00) evaluates to true.
// NextOccurrence must be the "to" boundary on the following day (06:00 UTC on
// 2024-01-16) because the current time is in the evening segment and the window
// exits after midnight.
func TestTimeRangeCondition_WithinRangeCrossMidnight(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "22:00:00", To: "06:00:00", Timezone: "UTC"}
	// 23:00 UTC is after the 22:00 start and before midnight, so within range.
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 23, 0, 0, 0, time.UTC)})
	if !result.Result {
		t.Errorf("expected true result, got false (reason: %s)", result.Reason)
	}
	// Evening segment: exits at 06:00 UTC the next day.
	want := time.Date(2024, 1, 16, 6, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeCondition_BeforeRangeCrossMidnight verifies that a time just before
// the "from" boundary of a midnight-crossing range (i.e. in the gap) evaluates to false.
// NextOccurrence must be the "from" boundary on the same day (22:00 UTC on 2024-01-15)
// because that is when the window next opens.
func TestTimeRangeCondition_BeforeRangeCrossMidnight(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "22:00:00", To: "06:00:00", Timezone: "UTC"}
	// 21:00 UTC is before the 22:00 start and after the 06:00 end, so outside range.
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 21, 0, 0, 0, time.UTC)})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	// Window opens at 22:00 UTC today.
	want := time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeCondition_AfterRangeCrossMidnight verifies that a time just past the
// "to" boundary of a midnight-crossing range (i.e. in the gap) evaluates to false.
// NextOccurrence must be the "from" boundary on the same day (22:00 UTC on 2024-01-15)
// because that is when the window next opens.
func TestTimeRangeCondition_AfterRangeCrossMidnight(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "22:00:00", To: "06:00:00", Timezone: "UTC"}
	// 07:00 UTC is after the 06:00 end and before the 22:00 start, so outside range.
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 7, 0, 0, 0, time.UTC)})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	// Window opens at 22:00 UTC today.
	want := time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeCondition_WithinRangeWithTimezoneDaylightSavingJumpFromBeforeToWithin
// verifies that DST is applied correctly when the UTC time would fall before the range
// under the standard (non-DST) offset, but lands within the range once the active DST
// offset is applied.
//
// Scenario: range 10:00–20:00, timezone America/New_York.
// Evaluation time: 2024-07-15 14:30 UTC (summer, EDT active → UTC-4).
//   - With DST (EDT, UTC-4): 14:30 − 4 h = 10:30 → within range.
//   - Without DST (EST, UTC-5): 14:30 − 5 h = 09:30 → before range.
//
// NextOccurrence must be 20:00 EDT = 2024-07-16 00:00 UTC, the moment the window closes.
func TestTimeRangeCondition_WithinRangeWithTimezoneDaylightSavingJumpFromBeforeToWithin(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "10:00:00", To: "20:00:00", Timezone: "America/New_York"}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 7, 15, 14, 30, 0, 0, time.UTC)})
	if !result.Result {
		t.Errorf("expected true result with DST applied, got false (reason: %s)", result.Reason)
	}
	// 20:00 EDT on 2024-07-15 = 00:00 UTC on 2024-07-16.
	want := time.Date(2024, 7, 16, 0, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeCondition_WithinRangeWithTimezoneDaylightSavingJumpFromAfterToWithin
// verifies that DST is applied correctly when the UTC time would fall after the range
// under the DST offset, but lands within the range once the standard (non-DST) offset
// is applied.
//
// Scenario: range 10:00–18:00, timezone America/New_York.
// Evaluation time: 2024-01-15 22:30 UTC (winter, EST active → UTC-5).
//   - With standard time (EST, UTC-5): 22:30 − 5 h = 17:30 → within range.
//   - Without standard time (EDT, UTC-4): 22:30 − 4 h = 18:30 → after range.
//
// NextOccurrence must be 18:00 EST = 2024-01-15 23:00 UTC, the moment the window closes.
func TestTimeRangeCondition_WithinRangeWithTimezoneDaylightSavingJumpFromAfterToWithin(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "10:00:00", To: "18:00:00", Timezone: "America/New_York"}
	result := cond.Evaluate(stubEvalContext{now: time.Date(2024, 1, 15, 22, 30, 0, 0, time.UTC)})
	if !result.Result {
		t.Errorf("expected true result with standard time applied, got false (reason: %s)", result.Reason)
	}
	// 18:00 EST on 2024-01-15 = 23:00 UTC on 2024-01-15.
	want := time.Date(2024, 1, 15, 23, 0, 0, 0, time.UTC)
	assertNextOccurrence(t, result, &want)
}

// TestTimeRangeCondition_NonExistentTimezone verifies that an unrecognised timezone
// string causes Evaluate to return false with a non-empty reason rather than panicking.
// NextOccurrence must be nil because evaluation failed before any boundary could be computed.
func TestTimeRangeCondition_NonExistentTimezone(t *testing.T) {
	cond := restmodels.TimeRangeCondition{Type: "time-range", From: "06:00:00", To: "22:00:00", Timezone: "Not/A/Real/Timezone"}
	result := cond.Evaluate(stubEvalContext{now: time.Now()})
	if result.Result {
		t.Errorf("expected false result, got true (reason: %s)", result.Reason)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason")
	}
	assertNextOccurrence(t, result, nil)
}
