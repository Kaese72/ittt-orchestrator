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
func (o *Orchestrator) EvaluateAndTrigger(ruleID int) (EvalResult, error) {
	rule, err := o.db.GetRule(ruleID)
	if err != nil {
		return EvalResult{}, err
	}
	if !rule.Enabled {
		return EvalResult{}, nil
	}
	ctx := &evalContext{dsClient: o.dsClient, deviceCache: make(map[int][]devicestore.Attribute)}
	result := evaluateTree(rule.ConditionTree, ctx)
	if result.Result {
		for _, action := range rule.Actions {
			if err := triggerAction(o.dsClient, action); err != nil {
				log.Error(fmt.Sprintf("failed to trigger action %d for rule %d: %s", action.ActionID, ruleID, err.Error()), map[string]interface{}{})
			}
		}
	}
	return result, nil
}

// EvalResult holds the outcome of a condition tree evaluation.
type EvalResult struct {
	Result         bool
	Reason         string     // non-empty when Result is false
	NextOccurrence *time.Time // when the rule should next be re-evaluated; nil if not applicable
}

// EvaluateConditionTree evaluates the condition tree of the given rule against
// the current time and live device state, returning the result and reason.
func (o *Orchestrator) EvaluateConditionTree(ruleID int) (EvalResult, error) {
	rule, err := o.db.GetRule(ruleID)
	if err != nil {
		return EvalResult{}, err
	}
	ctx := &evalContext{
		dsClient:    o.dsClient,
		deviceCache: make(map[int][]devicestore.Attribute),
	}
	return evaluateTree(rule.ConditionTree, ctx), nil
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
		result := evaluateTree(rule.ConditionTree, ctx)
		if result.Result {
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

// minNextOcc returns the earlier of two optional timestamps.
func minNextOcc(a, b *time.Time) *time.Time {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Before(*b) {
		return a
	}
	return b
}

// evaluateTree evaluates a condition tree recursively, returning a result,
// a human-readable reason when the result is false, and the next occurrence
// timestamp (the earliest non-nil value emitted by any node in the tree).
func evaluateTree(tree *restmodels.ConditionTree, ctx *evalContext) EvalResult {
	if tree == nil {
		return EvalResult{Result: true}
	}

	node := evaluateCondition(tree.Condition, ctx)
	nextOcc := node.NextOccurrence

	combined := node
	if tree.And != nil {
		andResult := evaluateTree(tree.And, ctx)
		nextOcc = minNextOcc(nextOcc, andResult.NextOccurrence)
		if !andResult.Result {
			combined = EvalResult{Result: false, Reason: andResult.Reason}
			if !node.Result {
				combined.Reason = node.Reason
			}
		}
	}

	if tree.Or != nil {
		orResult := evaluateTree(tree.Or, ctx)
		nextOcc = minNextOcc(nextOcc, orResult.NextOccurrence)
		if combined.Result || orResult.Result {
			return EvalResult{Result: true, NextOccurrence: nextOcc}
		}
		return EvalResult{
			Result:         false,
			Reason:         fmt.Sprintf("(%s) OR (%s)", combined.Reason, orResult.Reason),
			NextOccurrence: nextOcc,
		}
	}

	combined.NextOccurrence = nextOcc
	return combined
}

func evaluateCondition(cond restmodels.Condition, ctx *evalContext) EvalResult {
	switch cond.Type {
	case "time-range":
		from, err := time.Parse("15:04:05", cond.From)
		if err != nil {
			log.Error(fmt.Sprintf("invalid from time in time-range condition: %s", err.Error()), map[string]interface{}{})
			return EvalResult{Result: false, Reason: fmt.Sprintf("invalid from time format %q", cond.From)}
		}
		to, err := time.Parse("15:04:05", cond.To)
		if err != nil {
			log.Error(fmt.Sprintf("invalid to time in time-range condition: %s", err.Error()), map[string]interface{}{})
			return EvalResult{Result: false, Reason: fmt.Sprintf("invalid to time format %q", cond.To)}
		}
		loc := time.UTC
		if cond.Timezone != "" {
			if l, err := time.LoadLocation(cond.Timezone); err != nil {
				log.Error(fmt.Sprintf("unknown timezone %q in time-range condition, falling back to UTC: %s", cond.Timezone, err.Error()), map[string]interface{}{})
			} else {
				loc = l
			}
		}
		now := time.Now().In(loc)
		fromToday := time.Date(now.Year(), now.Month(), now.Day(), from.Hour(), from.Minute(), from.Second(), 0, loc)
		toToday := time.Date(now.Year(), now.Month(), now.Day(), to.Hour(), to.Minute(), to.Second(), 0, loc)
		tomorrow := 24 * time.Hour

		var inRange bool
		var nextOcc time.Time

		if !fromToday.After(toToday) {
			// Normal range e.g. 06:00–22:00
			inRange = !now.Before(fromToday) && now.Before(toToday)
			if inRange {
				nextOcc = toToday // exit at to today
			} else if now.Before(fromToday) {
				nextOcc = fromToday // enter at from today
			} else {
				nextOcc = fromToday.Add(tomorrow) // enter at from tomorrow
			}
		} else {
			// Midnight-wrapping range e.g. 22:00–06:00
			inRange = !now.Before(fromToday) || now.Before(toToday)
			if inRange {
				if !now.Before(fromToday) {
					nextOcc = toToday.Add(tomorrow) // in evening part, exit at to tomorrow
				} else {
					nextOcc = toToday // in morning part, exit at to today
				}
			} else {
				nextOcc = fromToday // outside range, enter at from today
			}
		}

		if !inRange {
			return EvalResult{
				Result:         false,
				Reason:         fmt.Sprintf("current time %s is outside range %s–%s", now.Format("15:04:05"), cond.From, cond.To),
				NextOccurrence: &nextOcc,
			}
		}
		return EvalResult{Result: true, NextOccurrence: &nextOcc}

	case "device-id-attribute-boolean-eq":
		if cond.Boolean == nil {
			return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s: no expected value configured", cond.ID, cond.Attribute)}
		}
		attrValue, err := ctx.getDeviceBooleanAttribute(cond.ID, cond.Attribute)
		if err != nil {
			log.Error(fmt.Sprintf("failed to fetch device %d attribute %q: %s", cond.ID, cond.Attribute, err.Error()), map[string]interface{}{})
			return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s: fetch error: %s", cond.ID, cond.Attribute, err.Error())}
		}
		if attrValue == nil {
			return EvalResult{Result: false, Reason: fmt.Sprintf("device %d has no attribute %q", cond.ID, cond.Attribute)}
		}
		if *attrValue != *cond.Boolean {
			return EvalResult{Result: false, Reason: fmt.Sprintf("device %d.%s is %v, expected %v", cond.ID, cond.Attribute, *attrValue, *cond.Boolean)}
		}
		return EvalResult{Result: true}

	default:
		log.Error(fmt.Sprintf("unknown condition type: %s", cond.Type), map[string]interface{}{})
		return EvalResult{Result: false, Reason: fmt.Sprintf("unknown condition type %q", cond.Type)}
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
