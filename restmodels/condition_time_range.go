package restmodels

import (
	"fmt"
	"time"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/danielgtaylor/huma/v2"
)

// TimeRangeCondition checks whether the current time falls within a daily window.
type TimeRangeCondition struct {
	Type     string `json:"type"`
	From     string `json:"from"     format:"time" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$" patternDescription:"HH:MM:SS" example:"06:00:00"`
	To       string `json:"to"       format:"time" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$" patternDescription:"HH:MM:SS" example:"22:00:00"`
	Timezone string `json:"timezone" doc:"IANA timezone identifier" example:"Europe/Stockholm"`
}

func (c TimeRangeCondition) DeviceReferences() []int { return nil }

func (c TimeRangeCondition) Evaluate(ctx EvalContext) EvalResult {
	from, err := time.Parse("15:04:05", c.From)
	if err != nil {
		log.Error(fmt.Sprintf("invalid from time in time-range condition: %s", err.Error()), map[string]interface{}{})
		return EvalResult{Result: false, Reason: fmt.Sprintf("invalid from time format %q", c.From)}
	}
	to, err := time.Parse("15:04:05", c.To)
	if err != nil {
		log.Error(fmt.Sprintf("invalid to time in time-range condition: %s", err.Error()), map[string]interface{}{})
		return EvalResult{Result: false, Reason: fmt.Sprintf("invalid to time format %q", c.To)}
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return EvalResult{Result: false, Reason: fmt.Sprintf("time-range condition has invalid timezone %q: %s", c.Timezone, err.Error())}
	}
	now := ctx.Now().In(loc)
	fromToday := time.Date(now.Year(), now.Month(), now.Day(), from.Hour(), from.Minute(), from.Second(), 0, loc)
	toToday := time.Date(now.Year(), now.Month(), now.Day(), to.Hour(), to.Minute(), to.Second(), 0, loc)
	tomorrow := 24 * time.Hour

	var inRange bool
	var nextOcc time.Time

	if !fromToday.After(toToday) {
		// Normal range e.g. 06:00–22:00
		inRange = !now.Before(fromToday) && now.Before(toToday)
		if inRange {
			nextOcc = toToday
		} else if now.Before(fromToday) {
			nextOcc = fromToday
		} else {
			nextOcc = fromToday.Add(tomorrow)
		}
	} else {
		// Midnight-wrapping range e.g. 22:00–06:00
		inRange = !now.Before(fromToday) || now.Before(toToday)
		if inRange {
			if !now.Before(fromToday) {
				nextOcc = toToday.Add(tomorrow)
			} else {
				nextOcc = toToday
			}
		} else {
			nextOcc = fromToday
		}
	}

	if !inRange {
		return EvalResult{
			Result:         false,
			Reason:         fmt.Sprintf("current time %s is outside range %s–%s", now.Format("15:04:05"), c.From, c.To),
			NextOccurrence: &nextOcc,
		}
	}
	return EvalResult{Result: true, NextOccurrence: &nextOcc}
}

func (c TimeRangeCondition) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	if c.Timezone == "" {
		return []error{&huma.ErrorDetail{
			Message:  "required for time-range conditions",
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
