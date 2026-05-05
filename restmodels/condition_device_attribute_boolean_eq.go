package restmodels

import (
	"fmt"

	log "github.com/Kaese72/huemie-lib/logging"
)

// DeviceAttributeBooleanEqCondition checks that a device attribute equals a boolean value.
type DeviceAttributeBooleanEqCondition struct {
	Type      string `json:"type"`
	ID        int    `json:"id"`
	Attribute string `json:"attribute"`
	Boolean   bool   `json:"boolean"`
}

func (c DeviceAttributeBooleanEqCondition) DeviceReferences() []int { return []int{c.ID} }

func (c DeviceAttributeBooleanEqCondition) Evaluate(ctx EvalContext) EvalResult {
	attrValue, err := ctx.GetDeviceAttribute(c.ID, c.Attribute)
	if err != nil {
		log.Error(fmt.Sprintf("failed to fetch device %d attribute %q: %s", c.ID, c.Attribute, err.Error()), map[string]interface{}{})
		return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s: fetch error: %s", c.ID, c.Attribute, err.Error())}
	}
	if attrValue == nil {
		return EvalResult{Result: false, Reason: fmt.Sprintf("device %d has no attribute %q", c.ID, c.Attribute)}
	}
	if attrValue.Boolean == nil {
		return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s has no boolean value", c.ID, c.Attribute)}
	}
	if *attrValue.Boolean != c.Boolean {
		return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %v, expected %v", c.ID, c.Attribute, *attrValue.Boolean, c.Boolean)}
	}
	return EvalResult{Result: true}
}
