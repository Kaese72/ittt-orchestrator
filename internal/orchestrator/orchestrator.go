package orchestrator

import (
	"fmt"
	"time"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/eventmodels"
	"github.com/Kaese72/ittt-orchestrator/internal/devicestore"
	"github.com/Kaese72/ittt-orchestrator/internal/persistence"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
)

// Orchestrator evaluates rules against incoming device state changes and triggers
// actions when conditions are met.
type Orchestrator struct {
	db       persistence.PersistenceDB
	dsClient *devicestore.Client
}

func New(db persistence.PersistenceDB, dsClient *devicestore.Client) *Orchestrator {
	return &Orchestrator{db: db, dsClient: dsClient}
}

// EvaluateAndTrigger evaluates the condition tree of the given rule, triggers
// its actions if the result is true, and returns the full EvalResult so the
// caller can act on NextOccurrence.
func (o *Orchestrator) EvaluateAndTrigger(ruleID int) (restmodels.EvalResult, error) {
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
	evalTime := time.Now()
	if rule.NextOccurrence != nil {
		evalTime = *rule.NextOccurrence
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

// EvaluateConditionTree evaluates the condition tree of the given rule against
// the current time and live device state, returning the result and reason.
func (o *Orchestrator) EvaluateConditionTree(ruleID int) (restmodels.EvalResult, error) {
	rule, err := o.db.GetRule(ruleID)
	if err != nil {
		return restmodels.EvalResult{}, err
	}
	if rule.ConditionTree == nil {
		return restmodels.EvalResult{}, fmt.Errorf("rule %d has no condition tree", ruleID)
	}
	ctx := &evalContext{
		dsClient:    o.dsClient,
		deviceCache: make(map[int][]devicestore.Attribute),
		now:         time.Now(),
	}
	return rule.ConditionTree.Evaluate(ctx), nil

}

// HandleDeviceUpdate is called for every device attribute update received from
// the event bus. It finds rules that reference the updated device, evaluates
// their condition trees, and triggers actions for those that evaluate to true.
func (o *Orchestrator) HandleDeviceUpdate(update eventmodels.DeviceAttributeUpdate) {
	rules, err := o.db.GetRules()
	if err != nil {
		log.Error(fmt.Sprintf("failed to load rules: %s", err.Error()), map[string]interface{}{})
		return
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.ConditionTree == nil {
			log.Error(fmt.Sprintf("skipping rule %d: no condition tree", rule.ID), map[string]interface{}{})
			continue
		}
		if !referencesDevice(*rule.ConditionTree, update.DeviceID) {
			continue
		}
		ctx := &evalContext{
			dsClient:    o.dsClient,
			deviceCache: make(map[int][]devicestore.Attribute),
			now:         time.Now(),
		}
		result := rule.ConditionTree.Evaluate(ctx)
		if result.Result {
			for _, action := range rule.Actions {
				if err := triggerAction(o.dsClient, action); err != nil {
					log.Error(fmt.Sprintf("failed to trigger action %d for rule %d: %s", action.ActionID, rule.ID, err.Error()), map[string]interface{}{})
				}
			}
		}
	}
}

// referencesDevice reports whether the tree contains any condition that references deviceID.
func referencesDevice(tree restmodels.ConditionTree, deviceID int) bool {
	for _, id := range tree.DeviceReferences() {
		if id == deviceID {
			return true
		}
	}
	return false
}

// evalContext carries per-evaluation state: a device-store client, a cache
// of already-fetched device attributes, and the logical evaluation time.
type evalContext struct {
	dsClient    *devicestore.Client
	deviceCache map[int][]devicestore.Attribute
	now         time.Time
}

// Now implements restmodels.EvalContext.
func (c *evalContext) Now() time.Time {
	return c.now
}

// GetDeviceAttribute implements restmodels.EvalContext.
func (c *evalContext) GetDeviceAttribute(deviceID int, attribute string) (*devicestore.Attribute, error) {
	if _, ok := c.deviceCache[deviceID]; !ok {
		device, err := c.dsClient.GetDevice(deviceID)
		if err != nil {
			return nil, err
		}
		c.deviceCache[deviceID] = device.Attributes
	}
	for _, attr := range c.deviceCache[deviceID] {
		if attr.Name == attribute {
			return &attr, nil
		}
	}
	return nil, nil
}

func triggerAction(dsClient *devicestore.Client, action restmodels.Action) error {
	switch action.Type {
	case "device-capability":
		return dsClient.TriggerDeviceCapability(action.ID, action.Capability, action.Args)
	case "group-capability":
		return dsClient.TriggerGroupCapability(action.ID, action.Capability, action.Args)
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}
