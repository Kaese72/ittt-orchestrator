package restmodels

import "github.com/danielgtaylor/huma/v2"

// This file holds each condition type's schema description: what it checks
// and whether it schedules evaluations. huma calls TransformSchema when it
// generates a type's schema, so these end up in the OpenAPI spec and in the
// GET /ittt-orchestrator/v0/rules/schema document AI agents read before
// writing a rule (see rule.go). A new condition type needs an entry here as
// well as in ConditionUnion; TestConditionSchemasDescribed checks both.

// describeCondition sets a condition schema's description and pins its
// "type" property to the discriminator value, so the schema is also correct
// for a plain JSON Schema consumer that ignores the OpenAPI discriminator
// (without it, every numeric condition would match every numeric schema).
func describeCondition(s *huma.Schema, conditionType, description string) *huma.Schema {
	s.Description = description
	if t, ok := s.Properties["type"]; ok {
		t.Enum = []any{conditionType}
	}
	return s
}

const (
	deviceConditionNoSchedule = " False if the device, the attribute or a value of that kind is missing. Never schedules an evaluation; the rule is evaluated when the device reports an update."
	timeConditionSchedule     = " Schedules an evaluation for the next time it starts or stops matching."
)

func (TimeRangeCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "time-range",
		"True every day while the local time is in the window from (inclusive) to to (exclusive). From later than to wraps past midnight, e.g. 22:00:00 to 06:00:00."+timeConditionSchedule)
}

func (TimeRangeDaysCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "time-range-days",
		"Like time-range, but only on the given days. A window wrapping past midnight belongs to the day it starts on: from 22:00:00 to 02:00:00 on friday is Friday 22:00 to Saturday 02:00."+timeConditionSchedule)
}

func (DeviceAttributeBooleanEqCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "device-id-attribute-boolean-eq",
		"True when the device's boolean attribute equals boolean."+deviceConditionNoSchedule)
}

func (DeviceAttributeNumberEqCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "device-id-attribute-number-eq",
		"True when the device's numeric attribute equals value, within 0.01. Use device-id-attribute-number-eq-margin for a different tolerance."+deviceConditionNoSchedule)
}

func (DeviceAttributeNumberEqMarginCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "device-id-attribute-number-eq-margin",
		"True when the device's numeric attribute is within margin of value (|attribute - value| <= margin)."+deviceConditionNoSchedule)
}

func (DeviceAttributeNumberLtCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "device-id-attribute-number-lt",
		"True when the device's numeric attribute is less than value."+deviceConditionNoSchedule)
}

func (DeviceAttributeNumberGtCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "device-id-attribute-number-gt",
		"True when the device's numeric attribute is greater than value."+deviceConditionNoSchedule)
}

func (DeviceAttributeNumberLteCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "device-id-attribute-number-lte",
		"True when the device's numeric attribute is less than or equal to value."+deviceConditionNoSchedule)
}

func (DeviceAttributeNumberGteCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "device-id-attribute-number-gte",
		"True when the device's numeric attribute is greater than or equal to value."+deviceConditionNoSchedule)
}

func (DeviceAttributeTextEqCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "device-id-attribute-text-eq",
		"True when the device's text attribute equals value exactly (case-sensitive)."+deviceConditionNoSchedule)
}

func (DeviceAttributeTextSubstringCondition) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	return describeCondition(s, "device-id-attribute-text-substring",
		"True when the device's text attribute contains value (case-sensitive)."+deviceConditionNoSchedule)
}

func (ConditionTree) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Description = "A node of the condition tree. A node is true when its condition and its \"and\" subtree are both true, or when its \"or\" subtree is true. " +
		"So nest with \"and\" for \"all of these\", and chain with \"or\" for \"any of these\"."
	return s
}
