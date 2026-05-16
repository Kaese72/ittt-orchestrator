package orchestrator

import (
	"fmt"
	"time"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/internal/devicestore"
	"github.com/Kaese72/ittt-orchestrator/internal/persistence"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
)

// DeviceStateReader is the read-only view of device-store used for condition evaluation.
// This is all the api mode ever needs from device-store.
type DeviceStateReader interface {
	GetDevice(id int) (devicestore.Device, error)
}

// DeviceStoreClient is the full device-store interface used by rule-state: condition
// evaluation plus capability triggering.
type DeviceStoreClient interface {
	DeviceStateReader
	TriggerDeviceCapability(id int, capability string, args map[string]any) error
	TriggerGroupCapability(id int, capability string, args map[string]any) error
}

// ConditionEvaluator performs read-only rule condition evaluation. It is the only
// orchestrator type used by the api mode.
type ConditionEvaluator struct {
	db       persistence.PersistenceDB
	dsClient DeviceStateReader
}

func NewConditionEvaluator(db persistence.PersistenceDB, dsClient DeviceStateReader) *ConditionEvaluator {
	return &ConditionEvaluator{db: db, dsClient: dsClient}
}

// EvaluateConditionTree evaluates the condition tree of the given rule against the
// current time and live device state, returning the result without triggering any actions.
func (e *ConditionEvaluator) EvaluateConditionTree(ruleID int) (restmodels.EvalResult, error) {
	rule, err := e.db.GetRule(ruleID)
	if err != nil {
		return restmodels.EvalResult{}, err
	}
	if rule.ConditionTree == nil {
		return restmodels.EvalResult{}, fmt.Errorf("rule %d has no condition tree", ruleID)
	}
	ctx := &evalContext{
		dsClient:    e.dsClient,
		deviceCache: make(map[int][]devicestore.Attribute),
		now:         time.Now(),
	}
	return rule.ConditionTree.Evaluate(ctx), nil
}

// EvaluateConditionTreeDirect evaluates the given condition tree at the given time (or now
// if at is nil), without looking up any rule from the database.
func (e *ConditionEvaluator) EvaluateConditionTreeDirect(tree *restmodels.ConditionTree, at *time.Time) (restmodels.EvalResult, error) {
	now := time.Now()
	if at != nil {
		now = *at
	}
	ctx := &evalContext{
		dsClient:    e.dsClient,
		deviceCache: make(map[int][]devicestore.Attribute),
		now:         now,
	}
	return tree.Evaluate(ctx), nil
}

// Orchestrator evaluates rules and triggers actions. It is only used by rule-state mode.
type Orchestrator struct {
	db       persistence.PersistenceDB
	dsClient DeviceStoreClient
}

func New(db persistence.PersistenceDB, dsClient DeviceStoreClient) *Orchestrator {
	return &Orchestrator{db: db, dsClient: dsClient}
}

// EvaluateAndTrigger evaluates the condition tree of the given rule at evalTime,
// triggers actions if conditions are true, and returns the EvalResult so the caller
// can act on NextOccurrence.
func (o *Orchestrator) EvaluateAndTrigger(ruleID int, evalTime time.Time) (restmodels.EvalResult, error) {
	rule, err := o.db.GetRule(ruleID)
	if err != nil {
		return restmodels.EvalResult{}, err
	}
	if !rule.Enabled {
		return restmodels.EvalResult{}, nil
	}
	if rule.ConditionTree == nil {
		return restmodels.EvalResult{}, fmt.Errorf("rule %d has no condition tree", ruleID)
	}

	// Clear an expired cooldown so the UI does not show stale data.
	if rule.CooldownUntil != nil && evalTime.After(*rule.CooldownUntil) {
		if err := o.db.UpdateCooldownUntil(ruleID, nil); err != nil {
			return restmodels.EvalResult{}, err
		}
	}

	ctx := &evalContext{dsClient: o.dsClient, deviceCache: make(map[int][]devicestore.Attribute), now: evalTime}
	result := rule.ConditionTree.Evaluate(ctx)

	if result.Result {
		for _, action := range rule.Actions {
			if err := triggerAction(o.dsClient, action); err != nil {
				log.Error(fmt.Sprintf("failed to trigger action %d for rule %d: %s", action.ActionID, ruleID, err.Error()), map[string]interface{}{})
			}
		}
	}

	return result, nil
}

// evalContext carries per-evaluation state: a device-store reader, a cache of
// already-fetched device attributes, and the logical evaluation time.
type evalContext struct {
	dsClient    DeviceStateReader
	deviceCache map[int][]devicestore.Attribute
	now         time.Time
}

func (c *evalContext) Now() time.Time {
	return c.now
}

func (c *evalContext) GetDeviceAttribute(deviceID int, attribute string) (*devicestore.Attribute, error) {
	if _, ok := c.deviceCache[deviceID]; !ok {
		device, err := c.dsClient.GetDevice(deviceID)
		if err != nil {
			return nil, err
		}
		c.deviceCache[deviceID] = device.Attributes // devicestore.Device is a value type
	}
	for _, attr := range c.deviceCache[deviceID] {
		if attr.Name == attribute {
			return &attr, nil
		}
	}
	return nil, nil
}

func triggerAction(dsClient DeviceStoreClient, action restmodels.Action) error {
	switch action.Type {
	case "device-capability":
		return dsClient.TriggerDeviceCapability(action.ID, action.Capability, action.Args)
	case "group-capability":
		return dsClient.TriggerGroupCapability(action.ID, action.Capability, action.Args)
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}
