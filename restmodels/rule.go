package restmodels

import "time"

// Action describes a capability trigger that fires when a rule's conditions are true.
type Action struct {
	// ActionID is the resource identifier used when managing actions individually.
	ActionID   int            `json:"action-id,omitempty"`
	Type       string         `json:"type"`
	ID         int            `json:"id"`
	Capability string         `json:"capability"`
	Args       map[string]any `json:"args,omitempty"`
}

// Rule is an ITTT automation rule.
type Rule struct {
	ID             int            `json:"id,omitempty"`
	Name           string         `json:"name"`
	Enabled        bool           `json:"enabled"`
	ConditionTree  *ConditionTree `json:"condition-tree,omitempty"`
	Actions        []Action       `json:"actions,omitempty"`
	NextOccurrence *time.Time     `json:"next-occurence,omitempty"`
	// CooldownUntil is system-managed. It is set when a condition-triggered cooldown is in
	// progress and cleared once the timer fires and the rule has been re-evaluated.
	CooldownUntil *time.Time `json:"cooldown-until,omitempty"`
}
