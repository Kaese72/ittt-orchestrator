package restmodels

import (
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// ConditionTree is a node in the logical expression tree.
// Each node holds a condition check and optional AND/OR child nodes.
type ConditionTree struct {
	Condition Condition      `json:"condition"`
	And       *ConditionTree `json:"and,omitempty"`
	Or        *ConditionTree `json:"or,omitempty"`
}

// Condition is a single boolean check within a ConditionTree node.
type Condition struct {
	Type string `json:"type"`
	// Used by time-range
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	// Used by device-id-attribute-boolean-eq
	ID        int    `json:"id,omitempty"`
	Attribute string `json:"attribute,omitempty"`
	Boolean   *bool  `json:"boolean,omitempty"`
}

// Resolve implements huma.Resolver. Huma calls this for every Condition in the
// request body tree, so timezone validation is enforced automatically on all
// create and update endpoints.
func (c Condition) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	if c.Type != "time-range" {
		return nil
	}
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
}
