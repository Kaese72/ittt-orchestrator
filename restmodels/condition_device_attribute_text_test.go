package restmodels_test

import (
	"testing"

	"github.com/Kaese72/ittt-orchestrator/internal/devicestore"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
)

func textCtx(deviceID int, attribute string, value string) stubEvalContext {
	v := value
	return stubEvalContext{
		store: map[int]map[string]devicestore.Attribute{
			deviceID: {
				attribute: {Name: attribute, Text: &v},
			},
		},
	}
}

func TestDeviceAttributeTextEqCondition_MissingDevice(t *testing.T) {
	ctx := stubEvalContext{store: map[int]map[string]devicestore.Attribute{}}
	c := restmodels.DeviceAttributeTextEqCondition{Type: "device-id-attribute-text-eq", ID: 1, Attribute: "mode", Value: "cool"}
	if c.Evaluate(ctx).Result {
		t.Fatal("expected false when device missing")
	}
}

func TestDeviceAttributeTextEqCondition_NotTextAttribute(t *testing.T) {
	boolVal := true
	ctx := stubEvalContext{
		store: map[int]map[string]devicestore.Attribute{
			1: {"mode": {Name: "mode", Boolean: &boolVal}},
		},
	}
	c := restmodels.DeviceAttributeTextEqCondition{Type: "device-id-attribute-text-eq", ID: 1, Attribute: "mode", Value: "cool"}
	if c.Evaluate(ctx).Result {
		t.Fatal("expected false when attribute has no text value")
	}
}

func TestDeviceAttributeTextEqCondition_Match(t *testing.T) {
	ctx := textCtx(1, "mode", "cool")
	c := restmodels.DeviceAttributeTextEqCondition{Type: "device-id-attribute-text-eq", ID: 1, Attribute: "mode", Value: "cool"}
	if !c.Evaluate(ctx).Result {
		t.Fatal("expected true: exact match")
	}
}

func TestDeviceAttributeTextEqCondition_NoMatch(t *testing.T) {
	ctx := textCtx(1, "mode", "heat")
	c := restmodels.DeviceAttributeTextEqCondition{Type: "device-id-attribute-text-eq", ID: 1, Attribute: "mode", Value: "cool"}
	if c.Evaluate(ctx).Result {
		t.Fatal("expected false: no exact match")
	}
}

func TestDeviceAttributeTextEqCondition_CaseSensitive(t *testing.T) {
	ctx := textCtx(1, "mode", "Cool")
	c := restmodels.DeviceAttributeTextEqCondition{Type: "device-id-attribute-text-eq", ID: 1, Attribute: "mode", Value: "cool"}
	if c.Evaluate(ctx).Result {
		t.Fatal("expected false: eq is case-sensitive")
	}
}

func TestDeviceAttributeTextSubstringCondition_Match(t *testing.T) {
	ctx := textCtx(1, "label", "living room light")
	c := restmodels.DeviceAttributeTextSubstringCondition{Type: "device-id-attribute-text-substring", ID: 1, Attribute: "label", Value: "room"}
	if !c.Evaluate(ctx).Result {
		t.Fatal("expected true: substring present")
	}
}

func TestDeviceAttributeTextSubstringCondition_NoMatch(t *testing.T) {
	ctx := textCtx(1, "label", "kitchen light")
	c := restmodels.DeviceAttributeTextSubstringCondition{Type: "device-id-attribute-text-substring", ID: 1, Attribute: "label", Value: "room"}
	if c.Evaluate(ctx).Result {
		t.Fatal("expected false: substring not present")
	}
}

func TestDeviceAttributeTextConditions_NoNextOccurrence(t *testing.T) {
	ctx := textCtx(1, "mode", "cool")
	conditions := []restmodels.Condition{
		restmodels.DeviceAttributeTextEqCondition{Type: "device-id-attribute-text-eq", ID: 1, Attribute: "mode", Value: "cool"},
		restmodels.DeviceAttributeTextSubstringCondition{Type: "device-id-attribute-text-substring", ID: 1, Attribute: "mode", Value: "oo"},
	}
	for _, cond := range conditions {
		if result := cond.Evaluate(ctx); result.NextOccurrence != nil {
			t.Errorf("%T must not emit a next occurrence", cond)
		}
	}
}
