package restmodels

import (
	"fmt"
	"math"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/danielgtaylor/huma/v2"
)

const defaultNumericEqMargin = 0.01

func fetchNumericAttribute(ctx EvalContext, id int, attribute string) (float64, EvalResult, bool) {
	attrValue, err := ctx.GetDeviceAttribute(id, attribute)
	if err != nil {
		log.Error(fmt.Sprintf("failed to fetch device %d attribute %q: %s", id, attribute, err.Error()), map[string]interface{}{})
		return 0, EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s: fetch error: %s", id, attribute, err.Error())}, false
	}
	if attrValue == nil {
		return 0, EvalResult{Result: false, Reason: fmt.Sprintf("device %d has no attribute %q", id, attribute)}, false
	}
	if attrValue.Numeric == nil {
		return 0, EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s has no numeric value", id, attribute)}, false
	}
	return float64(*attrValue.Numeric), EvalResult{}, true
}

// DeviceAttributeNumberEqCondition is true when |actual - value| < 0.01.
type DeviceAttributeNumberEqCondition struct {
	Type            string  `json:"type"`
	ID              int     `json:"id"`
	Attribute       string  `json:"attribute"`
	Value           float64 `json:"value"`
	CooldownSeconds *int64  `json:"cooldown-seconds,omitempty"`
}

func (c DeviceAttributeNumberEqCondition) DeviceReferences() []int    { return []int{c.ID} }
func (c DeviceAttributeNumberEqCondition) GetCooldownSeconds() *int64 { return c.CooldownSeconds }
func (c DeviceAttributeNumberEqCondition) Evaluate(ctx EvalContext) EvalResult {
	actual, fail, ok := fetchNumericAttribute(ctx, c.ID, c.Attribute)
	if !ok {
		return fail
	}
	if math.Abs(actual-c.Value) < defaultNumericEqMargin {
		return EvalResult{Result: true}
	}
	return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %v, not equal to %v (margin %.2f)", c.ID, c.Attribute, actual, c.Value, defaultNumericEqMargin)}
}

// DeviceAttributeNumberEqMarginCondition is true when |actual - value| <= margin.
type DeviceAttributeNumberEqMarginCondition struct {
	Type            string  `json:"type"`
	ID              int     `json:"id"`
	Attribute       string  `json:"attribute"`
	Value           float64 `json:"value"`
	Margin          float64 `json:"margin"`
	CooldownSeconds *int64  `json:"cooldown-seconds,omitempty"`
}

func (c DeviceAttributeNumberEqMarginCondition) DeviceReferences() []int    { return []int{c.ID} }
func (c DeviceAttributeNumberEqMarginCondition) GetCooldownSeconds() *int64 { return c.CooldownSeconds }
func (c DeviceAttributeNumberEqMarginCondition) Evaluate(ctx EvalContext) EvalResult {
	actual, fail, ok := fetchNumericAttribute(ctx, c.ID, c.Attribute)
	if !ok {
		return fail
	}
	if math.Abs(actual-c.Value) <= c.Margin {
		return EvalResult{Result: true}
	}
	return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %v, not equal to %v (margin %.4f)", c.ID, c.Attribute, actual, c.Value, c.Margin)}
}

func (c DeviceAttributeNumberEqMarginCondition) Resolve(_ huma.Context, _ *huma.PathBuffer) []error {
	if c.Margin < 0 {
		return []error{fmt.Errorf("margin must be >= 0")}
	}
	return nil
}

// DeviceAttributeNumberLtCondition is true when actual < value.
type DeviceAttributeNumberLtCondition struct {
	Type            string  `json:"type"`
	ID              int     `json:"id"`
	Attribute       string  `json:"attribute"`
	Value           float64 `json:"value"`
	CooldownSeconds *int64  `json:"cooldown-seconds,omitempty"`
}

func (c DeviceAttributeNumberLtCondition) DeviceReferences() []int    { return []int{c.ID} }
func (c DeviceAttributeNumberLtCondition) GetCooldownSeconds() *int64 { return c.CooldownSeconds }
func (c DeviceAttributeNumberLtCondition) Evaluate(ctx EvalContext) EvalResult {
	actual, fail, ok := fetchNumericAttribute(ctx, c.ID, c.Attribute)
	if !ok {
		return fail
	}
	if actual < c.Value {
		return EvalResult{Result: true}
	}
	return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %v, not less than %v", c.ID, c.Attribute, actual, c.Value)}
}

// DeviceAttributeNumberGtCondition is true when actual > value.
type DeviceAttributeNumberGtCondition struct {
	Type            string  `json:"type"`
	ID              int     `json:"id"`
	Attribute       string  `json:"attribute"`
	Value           float64 `json:"value"`
	CooldownSeconds *int64  `json:"cooldown-seconds,omitempty"`
}

func (c DeviceAttributeNumberGtCondition) DeviceReferences() []int    { return []int{c.ID} }
func (c DeviceAttributeNumberGtCondition) GetCooldownSeconds() *int64 { return c.CooldownSeconds }
func (c DeviceAttributeNumberGtCondition) Evaluate(ctx EvalContext) EvalResult {
	actual, fail, ok := fetchNumericAttribute(ctx, c.ID, c.Attribute)
	if !ok {
		return fail
	}
	if actual > c.Value {
		return EvalResult{Result: true}
	}
	return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %v, not greater than %v", c.ID, c.Attribute, actual, c.Value)}
}

// DeviceAttributeNumberLteCondition is true when actual <= value.
type DeviceAttributeNumberLteCondition struct {
	Type            string  `json:"type"`
	ID              int     `json:"id"`
	Attribute       string  `json:"attribute"`
	Value           float64 `json:"value"`
	CooldownSeconds *int64  `json:"cooldown-seconds,omitempty"`
}

func (c DeviceAttributeNumberLteCondition) DeviceReferences() []int    { return []int{c.ID} }
func (c DeviceAttributeNumberLteCondition) GetCooldownSeconds() *int64 { return c.CooldownSeconds }
func (c DeviceAttributeNumberLteCondition) Evaluate(ctx EvalContext) EvalResult {
	actual, fail, ok := fetchNumericAttribute(ctx, c.ID, c.Attribute)
	if !ok {
		return fail
	}
	if actual <= c.Value {
		return EvalResult{Result: true}
	}
	return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %v, not less than or equal to %v", c.ID, c.Attribute, actual, c.Value)}
}

// DeviceAttributeNumberGteCondition is true when actual >= value.
type DeviceAttributeNumberGteCondition struct {
	Type            string  `json:"type"`
	ID              int     `json:"id"`
	Attribute       string  `json:"attribute"`
	Value           float64 `json:"value"`
	CooldownSeconds *int64  `json:"cooldown-seconds,omitempty"`
}

func (c DeviceAttributeNumberGteCondition) DeviceReferences() []int    { return []int{c.ID} }
func (c DeviceAttributeNumberGteCondition) GetCooldownSeconds() *int64 { return c.CooldownSeconds }
func (c DeviceAttributeNumberGteCondition) Evaluate(ctx EvalContext) EvalResult {
	actual, fail, ok := fetchNumericAttribute(ctx, c.ID, c.Attribute)
	if !ok {
		return fail
	}
	if actual >= c.Value {
		return EvalResult{Result: true}
	}
	return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %v, not greater than or equal to %v", c.ID, c.Attribute, actual, c.Value)}
}
