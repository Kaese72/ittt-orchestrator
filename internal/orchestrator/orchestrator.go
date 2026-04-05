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
		if !referencesDevice(rule.ConditionTree, update.DeviceID) {
			continue
		}
		ctx := &evalContext{
			dsClient:    o.dsClient,
			deviceCache: make(map[int][]devicestore.Attribute),
		}
		if evaluateTree(rule.ConditionTree, ctx) {
			for _, action := range rule.Actions {
				if err := triggerAction(o.dsClient, action); err != nil {
					log.Error(fmt.Sprintf("failed to trigger action %d for rule %d: %s", action.ActionID, rule.ID, err.Error()), map[string]interface{}{})
				}
			}
		}
	}
}

// referencesDevice reports whether any device-id-attribute-boolean-eq condition
// in the tree references the given device ID.
func referencesDevice(tree *restmodels.ConditionTree, deviceID int) bool {
	if tree == nil {
		return false
	}
	if tree.Condition.Type == "device-id-attribute-boolean-eq" && tree.Condition.ID == deviceID {
		return true
	}
	return referencesDevice(tree.And, deviceID) || referencesDevice(tree.Or, deviceID)
}

// evalContext carries per-evaluation state: a device-store client and a cache
// of already-fetched device attributes.
type evalContext struct {
	dsClient    *devicestore.Client
	deviceCache map[int][]devicestore.Attribute
}

func (c *evalContext) getDeviceBooleanAttribute(deviceID int, attribute string) (*bool, error) {
	if _, ok := c.deviceCache[deviceID]; !ok {
		device, err := c.dsClient.GetDevice(deviceID)
		if err != nil {
			return nil, err
		}
		c.deviceCache[deviceID] = device.Attributes
	}
	for _, attr := range c.deviceCache[deviceID] {
		if attr.Name == attribute {
			return attr.Boolean, nil
		}
	}
	return nil, nil
}

// evaluateTree evaluates a condition tree recursively.
//
// Each node evaluates its own condition. If an AND child is present its result
// is ANDed with the node result. If an OR child is present the node result
// (after applying AND) is ORed with the OR subtree.
func evaluateTree(tree *restmodels.ConditionTree, ctx *evalContext) bool {
	if tree == nil {
		return true
	}
	nodeResult := evaluateCondition(tree.Condition, ctx)
	if tree.And != nil {
		nodeResult = nodeResult && evaluateTree(tree.And, ctx)
	}
	if tree.Or != nil {
		return nodeResult || evaluateTree(tree.Or, ctx)
	}
	return nodeResult
}

func evaluateCondition(cond restmodels.Condition, ctx *evalContext) bool {
	switch cond.Type {
	case "time-gte":
		t, err := time.Parse("15:04:05", cond.Time)
		if err != nil {
			log.Error(fmt.Sprintf("invalid time in time-gte condition: %s", err.Error()), map[string]interface{}{})
			return false
		}
		now := time.Now().UTC()
		threshold := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		return !now.Before(threshold)

	case "time-lt":
		t, err := time.Parse("15:04:05", cond.Time)
		if err != nil {
			log.Error(fmt.Sprintf("invalid time in time-lt condition: %s", err.Error()), map[string]interface{}{})
			return false
		}
		now := time.Now().UTC()
		threshold := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		return now.Before(threshold)

	case "device-id-attribute-boolean-eq":
		if cond.Boolean == nil {
			return false
		}
		attrValue, err := ctx.getDeviceBooleanAttribute(cond.ID, cond.Attribute)
		if err != nil {
			log.Error(fmt.Sprintf("failed to fetch device %d attribute %q: %s", cond.ID, cond.Attribute, err.Error()), map[string]interface{}{})
			return false
		}
		if attrValue == nil {
			return false
		}
		return *attrValue == *cond.Boolean

	default:
		log.Error(fmt.Sprintf("unknown condition type: %s", cond.Type), map[string]interface{}{})
		return false
	}
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
