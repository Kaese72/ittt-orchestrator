package restmodels

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/Kaese72/ittt-orchestrator/internal/devicestore"
	"github.com/danielgtaylor/huma/v2"
)

// EvalContext provides runtime data needed by conditions during evaluation.
type EvalContext interface {
	// Now returns the time at which this evaluation is considered to occur.
	// For scheduled rules this is the rule's next-occurrence timestamp; for
	// device-update or direct-trigger evaluations it is the wall-clock time.
	// We are doing it this way to reduce reliance on the orchestrator's
	// system clock and allow testing of time-based conditions.
	Now() time.Time
	GetDeviceAttribute(deviceID int, attribute string) (*devicestore.Attribute, error)
}

// EvalResult is the outcome of evaluating a condition or condition tree.
type EvalResult struct {
	Result         bool
	Reason         string
	NextOccurrence *time.Time
}

// Condition is the interface for all condition types.
type Condition interface {
	Evaluate(ctx EvalContext) EvalResult
	// DeviceReferences returns the IDs of any devices this condition checks,
	// or nil if it references no devices.
	DeviceReferences() []int
}

// ConditionTree is a node in the logical expression tree.
// Each node holds a condition check and optional AND/OR child nodes.
type ConditionTree struct {
	Condition ConditionUnion `json:"condition"`
	And       *ConditionTree `json:"and,omitempty"`
	Or        *ConditionTree `json:"or,omitempty"`
}

// Evaluate walks the tree recursively, returning the combined result, a
// human-readable reason when false, and the earliest next-occurrence hint.
func (t ConditionTree) Evaluate(ctx EvalContext) EvalResult {
	node := t.Condition.Value().Evaluate(ctx)
	nextOcc := node.NextOccurrence

	combined := node
	if t.And != nil {
		andResult := t.And.Evaluate(ctx)
		nextOcc = minNextOcc(nextOcc, andResult.NextOccurrence)
		if !andResult.Result {
			combined = EvalResult{Result: false, Reason: andResult.Reason}
			if !node.Result {
				combined.Reason = node.Reason
			}
		}
	}

	if t.Or != nil {
		orResult := t.Or.Evaluate(ctx)
		nextOcc = minNextOcc(nextOcc, orResult.NextOccurrence)
		if combined.Result || orResult.Result {
			return EvalResult{Result: true, NextOccurrence: nextOcc}
		}
		return EvalResult{
			Result:         false,
			Reason:         fmt.Sprintf("(%s) OR (%s)", combined.Reason, orResult.Reason),
			NextOccurrence: nextOcc,
		}
	}

	combined.NextOccurrence = nextOcc
	return combined
}

func minNextOcc(a, b *time.Time) *time.Time {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Before(*b) {
		return a
	}
	return b
}

// DeviceReferences returns all device IDs referenced anywhere in the tree, or nil if none.
func (t *ConditionTree) DeviceReferences() []int {
	var refs []int
	refs = append(refs, t.Condition.Value().DeviceReferences()...)
	if t.And != nil {
		refs = append(refs, t.And.DeviceReferences()...)
	}
	if t.Or != nil {
		refs = append(refs, t.Or.DeviceReferences()...)
	}
	if len(refs) == 0 {
		return nil
	}
	return refs
}

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
	case "time-range-days":
		var c TimeRangeDaysCondition
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
	trdRef := r.Schema(reflect.TypeOf(TimeRangeDaysCondition{}), true, "TimeRangeDaysCondition")
	daRef := r.Schema(reflect.TypeOf(DeviceAttributeBooleanEqCondition{}), true, "DeviceAttributeBooleanEqCondition")
	// Set titles on the registered component schemas so UI tools display the
	// type names instead of the fallback "object" label in the oneOf selector.
	if s := r.SchemaFromRef(trRef.Ref); s != nil {
		s.Title = "TimeRangeCondition"
	}
	if s := r.SchemaFromRef(trdRef.Ref); s != nil {
		s.Title = "TimeRangeDaysCondition"
	}
	if s := r.SchemaFromRef(daRef.Ref); s != nil {
		s.Title = "DeviceAttributeBooleanEqCondition"
	}
	return &huma.Schema{
		OneOf: []*huma.Schema{trRef, trdRef, daRef},
		Discriminator: &huma.Discriminator{
			PropertyName: "type",
			Mapping: map[string]string{
				"time-range":                     trRef.Ref,
				"time-range-days":                trdRef.Ref,
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
