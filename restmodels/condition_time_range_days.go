package restmodels

import (
	"fmt"
	"time"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/danielgtaylor/huma/v2"
)

var dayNameToWeekday = map[string]time.Weekday{
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
	"sunday":    time.Sunday,
}

// TimeRangeDaysCondition is like TimeRangeCondition but also restricts which days of the week
// are active. The anchor day is determined by when "from" falls, not by midnight — so a
// midnight-crossing range (e.g. from 22:00 to 02:00) with days=["friday"] covers
// Friday 22:00 through Saturday 02:00.
type TimeRangeDaysCondition struct {
	Type     string   `json:"type"`
	From     string   `json:"from"     format:"time" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$" patternDescription:"HH:MM:SS" example:"22:00:00"`
	To       string   `json:"to"       format:"time" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$" patternDescription:"HH:MM:SS" example:"02:00:00"`
	Timezone string   `json:"timezone" doc:"IANA timezone identifier" example:"Europe/Stockholm"`
	Days     []string `json:"days"     minItems:"1" maxItems:"6" enum:"monday,tuesday,wednesday,thursday,friday,saturday,sunday" doc:"Days on which the time range is active, determined by the day 'from' falls on."`
}

func (c TimeRangeDaysCondition) DeviceReferences() []int { return nil }

func (c TimeRangeDaysCondition) Evaluate(ctx EvalContext) EvalResult {
	from, err := time.Parse("15:04:05", c.From)
	if err != nil {
		log.Error(fmt.Sprintf("invalid from time in time-range-days condition: %s", err.Error()), map[string]interface{}{})
		return EvalResult{Result: false, Reason: fmt.Sprintf("invalid from time format %q", c.From)}
	}
	to, err := time.Parse("15:04:05", c.To)
	if err != nil {
		log.Error(fmt.Sprintf("invalid to time in time-range-days condition: %s", err.Error()), map[string]interface{}{})
		return EvalResult{Result: false, Reason: fmt.Sprintf("invalid to time format %q", c.To)}
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return EvalResult{Result: false, Reason: fmt.Sprintf("time-range-days condition has invalid timezone %q: %s", c.Timezone, err.Error())}
	}

	daySet := make(map[time.Weekday]bool, len(c.Days))
	for _, d := range c.Days {
		wd, ok := dayNameToWeekday[d]
		if !ok {
			return EvalResult{Result: false, Reason: fmt.Sprintf("unrecognised day name %q", d)}
		}
		daySet[wd] = true
	}

	now := ctx.Now().In(loc)
	fromToday := time.Date(now.Year(), now.Month(), now.Day(), from.Hour(), from.Minute(), from.Second(), 0, loc)
	toToday := time.Date(now.Year(), now.Month(), now.Day(), to.Hour(), to.Minute(), to.Second(), 0, loc)

	var inRange bool
	var anchorWeekday time.Weekday
	var windowEnd time.Time

	if !fromToday.After(toToday) {
		// Normal range e.g. 06:00–22:00
		inRange = !now.Before(fromToday) && now.Before(toToday)
		if inRange {
			anchorWeekday = fromToday.Weekday()
			windowEnd = toToday
		}
	} else {
		// Midnight-crossing range e.g. 22:00–02:00
		if !now.Before(fromToday) {
			// Evening segment: window started today, ends tomorrow
			inRange = true
			anchorWeekday = fromToday.Weekday()
			windowEnd = time.Date(now.Year(), now.Month(), now.Day()+1, to.Hour(), to.Minute(), to.Second(), 0, loc)
		} else if now.Before(toToday) {
			// Morning segment: window started yesterday, ends today
			inRange = true
			yesterday := time.Date(now.Year(), now.Month(), now.Day()-1, from.Hour(), from.Minute(), from.Second(), 0, loc)
			anchorWeekday = yesterday.Weekday()
			windowEnd = toToday
		}
	}

	if inRange && daySet[anchorWeekday] {
		return EvalResult{Result: true, NextOccurrence: &windowEnd}
	}

	nextOcc := nextFromOnMatchingDay(now, loc, from.Hour(), from.Minute(), from.Second(), daySet)
	var reason string
	if inRange {
		reason = fmt.Sprintf("current time %s is in range %s–%s but %s is not an active day", now.Format("15:04:05"), c.From, c.To, anchorWeekday)
	} else {
		reason = fmt.Sprintf("current time %s (%s) is outside range %s–%s", now.Format("15:04:05"), now.Weekday(), c.From, c.To)
	}
	return EvalResult{Result: false, Reason: reason, NextOccurrence: &nextOcc}
}

// nextFromOnMatchingDay returns the next time that the given clock time (h:m:s) falls on
// a day in daySet, strictly after now.
func nextFromOnMatchingDay(now time.Time, loc *time.Location, h, m, s int, daySet map[time.Weekday]bool) time.Time {
	candidate := time.Date(now.Year(), now.Month(), now.Day(), h, m, s, 0, loc)
	if !candidate.After(now) {
		candidate = time.Date(now.Year(), now.Month(), now.Day()+1, h, m, s, 0, loc)
	}
	for i := 0; i < 7; i++ {
		if daySet[candidate.Weekday()] {
			return candidate
		}
		candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day()+1, h, m, s, 0, loc)
	}
	return candidate
}

func (c TimeRangeDaysCondition) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	if c.Timezone == "" {
		return []error{&huma.ErrorDetail{
			Message:  "required for time-range-days conditions",
			Location: prefix.String() + "/timezone",
			Value:    c.Timezone,
		}}
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return []error{&huma.ErrorDetail{
			Message:  fmt.Sprintf("unrecognised timezone: %s", err),
			Location: prefix.String() + "/timezone",
			Value:    c.Timezone,
		}}
	}
	return nil
}
