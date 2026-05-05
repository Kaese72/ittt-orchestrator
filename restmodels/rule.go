package restmodels

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// ConditionTree is a node in the logical expression tree.
// Each node holds a condition check and optional AND/OR child nodes.
type ConditionTree struct {
	Condition ConditionUnion `json:"condition"`
	And       *ConditionTree `json:"and,omitempty"`
	Or        *ConditionTree `json:"or,omitempty"`
}

// Condition is the sealed interface for all condition types.
type Condition interface {
	isCondition()
}

// TimeRangeCondition checks whether the current time falls within a daily window.
type TimeRangeCondition struct {
	Type     string `json:"type"`
	From     string `json:"from"     format:"time" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$" patternDescription:"HH:MM:SS" example:"06:00:00"`
	To       string `json:"to"       format:"time" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$" patternDescription:"HH:MM:SS" example:"22:00:00"`
	Timezone string `json:"timezone" doc:"IANA timezone identifier" example:"Europe/Stockholm"`
}

func (TimeRangeCondition) isCondition() {}

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

// DeviceAttributeBooleanEqCondition checks that a device attribute equals a boolean value.
type DeviceAttributeBooleanEqCondition struct {
	Type      string `json:"type"`
	ID        int    `json:"id"`
	Attribute string `json:"attribute"`
	Boolean   *bool  `json:"boolean"`
}

func (DeviceAttributeBooleanEqCondition) isCondition() {}

// ConditionUnion is a JSON discriminated union of Condition types, dispatching on the "type" field.
type ConditionUnion struct {
	value Condition
}

func NewConditionUnion(c Condition) ConditionUnion {
	return ConditionUnion{value: c}
}

func (u ConditionUnion) Value() Condition {
	return u.value
}

func (u ConditionUnion) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.value)
}

func (u *ConditionUnion) UnmarshalJSON(data []byte) error {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	switch probe.Type {
	case "time-range":
		var c TimeRangeCondition
		if err := json.Unmarshal(data, &c); err != nil {
			return err
		}
		u.value = c
	case "device-id-attribute-boolean-eq":
		var c DeviceAttributeBooleanEqCondition
		if err := json.Unmarshal(data, &c); err != nil {
			return err
		}
		u.value = c
	default:
		return fmt.Errorf("unknown condition type: %q", probe.Type)
	}
	return nil
}

// Schema implements huma.SchemaProvider, emitting a oneOf schema with a type discriminator.
func (ConditionUnion) Schema(r huma.Registry) *huma.Schema {
	trRef := r.Schema(reflect.TypeOf(TimeRangeCondition{}), true, "TimeRangeCondition")
	daRef := r.Schema(reflect.TypeOf(DeviceAttributeBooleanEqCondition{}), true, "DeviceAttributeBooleanEqCondition")
	// Set titles on the registered component schemas so UI tools display the
	// type names instead of the fallback "object" label in the oneOf selector.
	if s := r.SchemaFromRef(trRef.Ref); s != nil {
		s.Title = "TimeRangeCondition"
	}
	if s := r.SchemaFromRef(daRef.Ref); s != nil {
		s.Title = "DeviceAttributeBooleanEqCondition"
	}
	return &huma.Schema{
		OneOf: []*huma.Schema{trRef, daRef},
		Discriminator: &huma.Discriminator{
			PropertyName: "type",
			Mapping: map[string]string{
				"time-range":                     trRef.Ref,
				"device-id-attribute-boolean-eq": daRef.Ref,
			},
		},
	}
}

// Resolve implements huma.ResolverWithPath, delegating to the inner condition if it supports validation.
func (u ConditionUnion) Resolve(ctx huma.Context, prefix *huma.PathBuffer) []error {
	if r, ok := u.value.(huma.ResolverWithPath); ok {
		return r.Resolve(ctx, prefix)
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
