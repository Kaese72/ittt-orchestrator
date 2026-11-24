package restmodels

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRuleRelatedIDs(t *testing.T) {
	var rule Rule
	if err := json.Unmarshal([]byte(`{
		"name": "evening", "enabled": true,
		"condition-tree": {
			"condition": {"type": "time-range", "from": "18:00:00", "to": "23:00:00", "timezone": "UTC"},
			"and": {
				"condition": {"type": "device-id-attribute-boolean-eq", "id": 7, "attribute": "active", "boolean": false},
				"or": {"condition": {"type": "device-id-attribute-number-lt", "id": 3, "attribute": "brightness", "value": 10}}
			}
		},
		"actions": [
			{"type": "device-capability", "id": 7, "capability": "activate"},
			{"type": "device-capability", "id": 12, "capability": "activate"},
			{"type": "group-capability", "id": 5, "capability": "activate"},
			{"type": "group-capability", "id": 2, "capability": "activate"},
			{"type": "group-capability", "id": 5, "capability": "deactivate"}
		]
	}`), &rule); err != nil {
		t.Fatal(err)
	}
	devices, groups := rule.RelatedIDs()
	if want := []int{3, 7, 12}; !reflect.DeepEqual(devices, want) {
		t.Errorf("devices = %v, want %v (sorted, de-duplicated, conditions and actions)", devices, want)
	}
	if want := []int{2, 5}; !reflect.DeepEqual(groups, want) {
		t.Errorf("groups = %v, want %v", groups, want)
	}
}

func TestRuleRelatedIDsEmpty(t *testing.T) {
	devices, groups := Rule{Name: "nothing"}.RelatedIDs()
	// Empty lists, not nil, so they encode as [] rather than null.
	if devices == nil || groups == nil || len(devices) != 0 || len(groups) != 0 {
		t.Fatalf("want empty non-nil lists, got %#v %#v", devices, groups)
	}
	// A zero ID is not a reference.
	devices, _ = Rule{Actions: []Action{{Type: ActionTypeDevice, ID: 0}}}.RelatedIDs()
	if len(devices) != 0 {
		t.Fatalf("zero ID counted as a reference: %v", devices)
	}
}
