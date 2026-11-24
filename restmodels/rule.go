package restmodels

import (
	"sort"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// The `doc` tags and TransformSchema descriptions in this package are part of
// the API: huma puts them in the generated JSON Schema, which is published in
// the OpenAPI spec and, self-contained, at GET /ittt-orchestrator/v0/rules/schema
// -- the rule format AI agents (the chatbot, MCP clients) read before writing
// a rule. Describe behavior there, not only structure, and keep it in sync
// with the code and README.condition-types.md. Keep it short: it is sent to
// a model every time the schema is fetched.

// Action describes a capability trigger that fires when a rule's conditions are true.
type Action struct {
	// ActionID is assigned by the server and ignored on input. Actions are replaced
	// wholesale whenever the owning rule is created or updated, so IDs are not stable
	// across saves.
	ActionID   int            `json:"action-id,omitempty" doc:"Assigned by the server; ignored on input, and not stable across saves of the rule."`
	Type       string         `json:"type" enum:"device-capability,group-capability" doc:"Whether id is a device ID (device-capability) or a group ID (group-capability)."`
	ID         int            `json:"id" doc:"The ID of the device or group to trigger the capability on."`
	Capability string         `json:"capability" doc:"The capability to trigger, as listed by device-store for that device or group."`
	Args       map[string]any `json:"args,omitempty" doc:"Capability arguments, in the shape device-store lists for that capability."`
}

func (Action) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Description = "A device or group capability triggered every time the rule fires."
	return s
}

// Rule is an ITTT automation rule.
type Rule struct {
	ID             int            `json:"id,omitempty" doc:"Assigned by the server; ignored on input."`
	Name           string         `json:"name"`
	Enabled        bool           `json:"enabled" doc:"A disabled rule is never evaluated, so it never fires."`
	ConditionTree  *ConditionTree `json:"condition-tree,omitempty" doc:"Decides when the rule fires."`
	Actions        []Action       `json:"actions,omitempty" doc:"Triggered, in order, every time the rule fires."`
	NextOccurrence *time.Time     `json:"next-occurence,omitempty" doc:"System-managed: when the rule is next evaluated on a schedule, because a time condition starts or stops matching then. Ignored on input."`
	// CooldownUntil is system-managed. It is set when a condition-triggered cooldown is in
	// progress and cleared once the timer fires and the rule has been re-evaluated.
	CooldownUntil *time.Time `json:"cooldown-until,omitempty" doc:"System-managed: set while a condition cooldown postpones evaluation, to when it will be evaluated. Ignored on input."`
	// RelatedDevices/RelatedGroups are derived from the condition tree and
	// actions (RelatedIDs) and stored whenever the rule is written, so rules
	// can be filtered by them. Always present in responses, never required
	// on input -- clients round-trip GET output into PUT unchanged.
	RelatedDevices []int `json:"related-devices" required:"false" readOnly:"true" doc:"System-managed: IDs of every device this rule refers to, in its conditions or as an action target. Ignored on input."`
	RelatedGroups  []int `json:"related-groups" required:"false" readOnly:"true" doc:"System-managed: IDs of every group this rule triggers as an action target. Ignored on input."`
}

const (
	ActionTypeDevice = "device-capability"
	ActionTypeGroup  = "group-capability"
)

// RelatedIDs returns the sorted, de-duplicated IDs of the devices (condition
// references and device-capability actions) and groups (group-capability
// actions) the rule refers to. A group's member devices are not included:
// membership lives in device-store and changes without the rule being saved.
// Non-positive IDs are not real references and are skipped (persistence
// stores a zero condition device ID as NULL, too).
func (r Rule) RelatedIDs() (devices []int, groups []int) {
	deviceSet := map[int]bool{}
	groupSet := map[int]bool{}
	if r.ConditionTree != nil {
		for _, id := range r.ConditionTree.DeviceReferences() {
			deviceSet[id] = true
		}
	}
	for _, action := range r.Actions {
		switch action.Type {
		case ActionTypeDevice:
			deviceSet[action.ID] = true
		case ActionTypeGroup:
			groupSet[action.ID] = true
		}
	}
	return sortedPositive(deviceSet), sortedPositive(groupSet)
}

func sortedPositive(set map[int]bool) []int {
	ids := []int{}
	for id := range set {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

func (Rule) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Description = "An automation rule. Whenever it is evaluated and its condition tree is true, all its actions are triggered -- on every such evaluation, not only when it changes from false to true. " +
		"An enabled rule is evaluated right after it is created or replaced, whenever a device referenced by one of its conditions reports an update (postponed by that condition's cooldown-seconds, if set), and at its next-occurence. " +
		"Rules are always written whole: creating or replacing a rule stores exactly the given name, enabled flag, condition tree and actions, and anything left out is removed."
	return s
}
