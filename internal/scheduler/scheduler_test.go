package scheduler

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Kaese72/huemie-lib/query"
	"github.com/Kaese72/ittt-orchestrator/eventmodels"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
)

// fakeDB records the filters GetRules is called with and returns fixed
// rules. Only the methods HandleDeviceUpdate uses do anything.
type fakeDB struct {
	mu       sync.Mutex
	rules    []restmodels.Rule
	filters  [][]query.Filter
	cooldown map[int]*time.Time
}

func (f *fakeDB) GetRules(filters []query.Filter, _ []query.Sort, _ query.Pagination) ([]restmodels.Rule, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters = append(f.filters, filters)
	return f.rules, len(f.rules), nil
}
func (f *fakeDB) GetRule(int) (restmodels.Rule, error)                { return restmodels.Rule{}, nil }
func (f *fakeDB) CreateRule(restmodels.Rule) (restmodels.Rule, error) { return restmodels.Rule{}, nil }
func (f *fakeDB) UpdateRule(int, restmodels.Rule) (restmodels.Rule, error) {
	return restmodels.Rule{}, nil
}
func (f *fakeDB) DeleteRule(int) error                       { return nil }
func (f *fakeDB) UpdateNextOccurrence(int, *time.Time) error { return nil }
func (f *fakeDB) UpdateCooldownUntil(ruleID int, t *time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cooldown[ruleID] = t
	return nil
}

func TestHandleDeviceUpdateLoadsOnlyRelatedRules(t *testing.T) {
	hour := int64(3600)
	db := &fakeDB{
		cooldown: map[int]*time.Time{},
		rules: []restmodels.Rule{
			{
				// A condition on device 7, with a cooldown so it is scheduled
				// rather than evaluated (this test has no orchestrator).
				ID: 1, Enabled: true,
				ConditionTree: &restmodels.ConditionTree{Condition: restmodels.NewConditionUnion(
					restmodels.DeviceAttributeBooleanEqCondition{Type: "device-id-attribute-boolean-eq", ID: 7, Attribute: "active", CooldownSeconds: &hour},
				)},
			},
			{
				// Device 7 is only an action target: related, but its update
				// must not trigger the rule.
				ID: 2, Enabled: true,
				ConditionTree: &restmodels.ConditionTree{Condition: restmodels.NewConditionUnion(
					restmodels.DeviceAttributeBooleanEqCondition{Type: "device-id-attribute-boolean-eq", ID: 9, Attribute: "active", CooldownSeconds: &hour},
				)},
				Actions: []restmodels.Action{{Type: restmodels.ActionTypeDevice, ID: 7, Capability: "activate"}},
			},
		},
	}
	s := New(db, nil)
	defer func() {
		for id := range s.timers {
			s.cancel(id)
		}
	}()

	s.HandleDeviceUpdate(eventmodels.DeviceAttributeUpdate{DeviceID: 7})

	if len(db.filters) != 1 || !reflect.DeepEqual(db.filters[0], DeviceUpdateFilters(7)) {
		t.Fatalf("GetRules filters = %v, want %v", db.filters, DeviceUpdateFilters(7))
	}
	if db.cooldown[1] == nil {
		t.Error("rule 1 (condition on device 7) was not scheduled")
	}
	if _, ok := db.cooldown[2]; ok {
		t.Error("rule 2 (device 7 only an action target) was scheduled")
	}
}

func TestHandleDeviceUpdateIgnoresInvalidDeviceID(t *testing.T) {
	db := &fakeDB{cooldown: map[int]*time.Time{}}
	New(db, nil).HandleDeviceUpdate(eventmodels.DeviceAttributeUpdate{DeviceID: 0})
	if len(db.filters) != 0 {
		t.Fatalf("GetRules called for device 0: %v", db.filters)
	}
}
