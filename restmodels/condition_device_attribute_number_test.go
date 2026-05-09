package restmodels_test

import (
	"testing"

	"github.com/Kaese72/ittt-orchestrator/internal/devicestore"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
)

func numericCtx(deviceID int, attribute string, value float32) stubEvalContext {
	v := value
	return stubEvalContext{
		store: map[int]map[string]devicestore.Attribute{
			deviceID: {
				attribute: {Name: attribute, Numeric: &v},
			},
		},
	}
}

func TestDeviceAttributeNumberEqCondition_MissingDevice(t *testing.T) {
	ctx := stubEvalContext{store: map[int]map[string]devicestore.Attribute{}}
	c := restmodels.DeviceAttributeNumberEqCondition{Type: "device-id-attribute-number-eq", ID: 1, Attribute: "brightness", Value: 50}
	if c.Evaluate(ctx).Result {
		t.Fatal("expected false when device missing")
	}
}

func TestDeviceAttributeNumberEqCondition_NotNumericAttribute(t *testing.T) {
	boolVal := true
	ctx := stubEvalContext{
		store: map[int]map[string]devicestore.Attribute{
			1: {"brightness": {Name: "brightness", Boolean: &boolVal}},
		},
	}
	c := restmodels.DeviceAttributeNumberEqCondition{Type: "device-id-attribute-number-eq", ID: 1, Attribute: "brightness", Value: 50}
	if c.Evaluate(ctx).Result {
		t.Fatal("expected false when attribute has no numeric value")
	}
}

func TestDeviceAttributeNumberEqCondition_WithinMargin(t *testing.T) {
	ctx := numericCtx(1, "brightness", 50.005)
	c := restmodels.DeviceAttributeNumberEqCondition{Type: "device-id-attribute-number-eq", ID: 1, Attribute: "brightness", Value: 50}
	if !c.Evaluate(ctx).Result {
		t.Fatal("expected true: value within default margin 0.01")
	}
}

func TestDeviceAttributeNumberEqCondition_OutsideMargin(t *testing.T) {
	ctx := numericCtx(1, "brightness", 50.02)
	c := restmodels.DeviceAttributeNumberEqCondition{Type: "device-id-attribute-number-eq", ID: 1, Attribute: "brightness", Value: 50}
	if c.Evaluate(ctx).Result {
		t.Fatal("expected false: value outside default margin 0.01")
	}
}

func TestDeviceAttributeNumberEqMarginCondition_WithinMargin(t *testing.T) {
	ctx := numericCtx(1, "brightness", 55)
	c := restmodels.DeviceAttributeNumberEqMarginCondition{Type: "device-id-attribute-number-eq-margin", ID: 1, Attribute: "brightness", Value: 50, Margin: 5}
	if !c.Evaluate(ctx).Result {
		t.Fatal("expected true: value within custom margin 5")
	}
}

func TestDeviceAttributeNumberEqMarginCondition_OutsideMargin(t *testing.T) {
	ctx := numericCtx(1, "brightness", 56)
	c := restmodels.DeviceAttributeNumberEqMarginCondition{Type: "device-id-attribute-number-eq-margin", ID: 1, Attribute: "brightness", Value: 50, Margin: 5}
	if c.Evaluate(ctx).Result {
		t.Fatal("expected false: value outside custom margin 5")
	}
}

func TestDeviceAttributeNumberLtCondition(t *testing.T) {
	tests := []struct {
		actual float32
		want   bool
	}{
		{49, true},
		{50, false},
		{51, false},
	}
	for _, tt := range tests {
		ctx := numericCtx(1, "brightness", tt.actual)
		c := restmodels.DeviceAttributeNumberLtCondition{Type: "device-id-attribute-number-lt", ID: 1, Attribute: "brightness", Value: 50}
		if c.Evaluate(ctx).Result != tt.want {
			t.Errorf("lt: actual=%v expected result=%v", tt.actual, tt.want)
		}
	}
}

func TestDeviceAttributeNumberGtCondition(t *testing.T) {
	tests := []struct {
		actual float32
		want   bool
	}{
		{51, true},
		{50, false},
		{49, false},
	}
	for _, tt := range tests {
		ctx := numericCtx(1, "brightness", tt.actual)
		c := restmodels.DeviceAttributeNumberGtCondition{Type: "device-id-attribute-number-gt", ID: 1, Attribute: "brightness", Value: 50}
		if c.Evaluate(ctx).Result != tt.want {
			t.Errorf("gt: actual=%v expected result=%v", tt.actual, tt.want)
		}
	}
}

func TestDeviceAttributeNumberLteCondition(t *testing.T) {
	tests := []struct {
		actual float32
		want   bool
	}{
		{49, true},
		{50, true},
		{51, false},
	}
	for _, tt := range tests {
		ctx := numericCtx(1, "brightness", tt.actual)
		c := restmodels.DeviceAttributeNumberLteCondition{Type: "device-id-attribute-number-lte", ID: 1, Attribute: "brightness", Value: 50}
		if c.Evaluate(ctx).Result != tt.want {
			t.Errorf("lte: actual=%v expected result=%v", tt.actual, tt.want)
		}
	}
}

func TestDeviceAttributeNumberGteCondition(t *testing.T) {
	tests := []struct {
		actual float32
		want   bool
	}{
		{51, true},
		{50, true},
		{49, false},
	}
	for _, tt := range tests {
		ctx := numericCtx(1, "brightness", tt.actual)
		c := restmodels.DeviceAttributeNumberGteCondition{Type: "device-id-attribute-number-gte", ID: 1, Attribute: "brightness", Value: 50}
		if c.Evaluate(ctx).Result != tt.want {
			t.Errorf("gte: actual=%v expected result=%v", tt.actual, tt.want)
		}
	}
}

func TestDeviceAttributeNumberConditions_NoNextOccurrence(t *testing.T) {
	ctx := numericCtx(1, "brightness", 50)
	conditions := []restmodels.Condition{
		restmodels.DeviceAttributeNumberEqCondition{Type: "device-id-attribute-number-eq", ID: 1, Attribute: "brightness", Value: 50},
		restmodels.DeviceAttributeNumberEqMarginCondition{Type: "device-id-attribute-number-eq-margin", ID: 1, Attribute: "brightness", Value: 50, Margin: 1},
		restmodels.DeviceAttributeNumberLtCondition{Type: "device-id-attribute-number-lt", ID: 1, Attribute: "brightness", Value: 100},
		restmodels.DeviceAttributeNumberGtCondition{Type: "device-id-attribute-number-gt", ID: 1, Attribute: "brightness", Value: 0},
		restmodels.DeviceAttributeNumberLteCondition{Type: "device-id-attribute-number-lte", ID: 1, Attribute: "brightness", Value: 50},
		restmodels.DeviceAttributeNumberGteCondition{Type: "device-id-attribute-number-gte", ID: 1, Attribute: "brightness", Value: 50},
	}
	for _, cond := range conditions {
		if result := cond.Evaluate(ctx); result.NextOccurrence != nil {
			t.Errorf("%T must not emit a next occurrence", cond)
		}
	}
}
