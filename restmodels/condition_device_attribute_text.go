package restmodels

import (
	"fmt"
	"strings"

	log "github.com/Kaese72/huemie-lib/logging"
)

func fetchTextAttribute(ctx EvalContext, id int, attribute string) (string, EvalResult, bool) {
	attrValue, err := ctx.GetDeviceAttribute(id, attribute)
	if err != nil {
		log.Error(fmt.Sprintf("failed to fetch device %d attribute %q: %s", id, attribute, err.Error()), map[string]interface{}{})
		return "", EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s: fetch error: %s", id, attribute, err.Error())}, false
	}
	if attrValue == nil {
		return "", EvalResult{Result: false, Reason: fmt.Sprintf("device %d has no attribute %q", id, attribute)}, false
	}
	if attrValue.Text == nil {
		return "", EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s has no text value", id, attribute)}, false
	}
	return *attrValue.Text, EvalResult{}, true
}

// DeviceAttributeTextEqCondition is true when the attribute text exactly matches value (case-sensitive).
type DeviceAttributeTextEqCondition struct {
	Type            string `json:"type"`
	ID              int    `json:"id" doc:"The ID of the device whose attribute is checked."`
	Attribute       string `json:"attribute" doc:"The attribute name, as listed by device-store for that device. Its current value is compared."`
	Value           string `json:"value" doc:"The text the attribute is compared with."`
	CooldownSeconds *int64 `json:"cooldown-seconds,omitempty" doc:"Debounce: when this device reports an update, the rule is evaluated this many seconds after its latest update instead of immediately, so it only fires if the condition still holds by then."`
}

func (c DeviceAttributeTextEqCondition) DeviceReferences() []int    { return []int{c.ID} }
func (c DeviceAttributeTextEqCondition) GetCooldownSeconds() *int64 { return c.CooldownSeconds }
func (c DeviceAttributeTextEqCondition) Evaluate(ctx EvalContext) EvalResult {
	actual, fail, ok := fetchTextAttribute(ctx, c.ID, c.Attribute)
	if !ok {
		return fail
	}
	if actual == c.Value {
		return EvalResult{Result: true}
	}
	return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %q, expected %q", c.ID, c.Attribute, actual, c.Value)}
}

// DeviceAttributeTextSubstringCondition is true when the attribute text contains value.
type DeviceAttributeTextSubstringCondition struct {
	Type            string `json:"type"`
	ID              int    `json:"id" doc:"The ID of the device whose attribute is checked."`
	Attribute       string `json:"attribute" doc:"The attribute name, as listed by device-store for that device. Its current value is compared."`
	Value           string `json:"value" doc:"The text the attribute is compared with."`
	CooldownSeconds *int64 `json:"cooldown-seconds,omitempty" doc:"Debounce: when this device reports an update, the rule is evaluated this many seconds after its latest update instead of immediately, so it only fires if the condition still holds by then."`
}

func (c DeviceAttributeTextSubstringCondition) DeviceReferences() []int    { return []int{c.ID} }
func (c DeviceAttributeTextSubstringCondition) GetCooldownSeconds() *int64 { return c.CooldownSeconds }
func (c DeviceAttributeTextSubstringCondition) Evaluate(ctx EvalContext) EvalResult {
	actual, fail, ok := fetchTextAttribute(ctx, c.ID, c.Attribute)
	if !ok {
		return fail
	}
	if strings.Contains(actual, c.Value) {
		return EvalResult{Result: true}
	}
	return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %q, does not contain %q", c.ID, c.Attribute, actual, c.Value)}
}
