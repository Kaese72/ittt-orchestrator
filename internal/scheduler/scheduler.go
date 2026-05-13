package scheduler

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/eventmodels"
	"github.com/Kaese72/ittt-orchestrator/internal/orchestrator"
	"github.com/Kaese72/ittt-orchestrator/internal/persistence"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
)

// Scheduler maintains per-rule timers and fires rule evaluation at each rule's
// next occurrence. It is the sole owner of scheduling state and is updated
// through RabbitMQ rule events and device-update events.
type Scheduler struct {
	db     persistence.PersistenceDB
	orch   *orchestrator.Orchestrator
	mu     sync.Mutex
	timers map[int]*time.Timer
}

func New(db persistence.PersistenceDB, orch *orchestrator.Orchestrator) *Scheduler {
	return &Scheduler{
		db:     db,
		orch:   orch,
		timers: make(map[int]*time.Timer),
	}
}

// Start seeds the initial schedule from next_occurrence stored in the database.
// Call this once after the scheduler is wired up.
func (s *Scheduler) Start() {
	rules, err := s.db.GetRules()
	if err != nil {
		log.Error(fmt.Sprintf("scheduler start: failed to load rules: %s", err.Error()), map[string]interface{}{})
		return
	}
	for _, rule := range rules {
		if !rule.Enabled || rule.NextOccurrence == nil {
			continue
		}
		s.scheduleAt(rule.ID, *rule.NextOccurrence)
	}
}

// HandleRuleEvent is called by the RabbitMQ consumer whenever a rule is
// created, updated, or deleted. On upsert the rule is evaluated immediately
// so the new or changed rule takes effect without delay.
func (s *Scheduler) HandleRuleEvent(event eventmodels.RuleEvent) {
	switch event.Event {
	case "upsert":
		rule, err := s.db.GetRule(event.RuleID)
		if err != nil {
			log.Error(fmt.Sprintf("scheduler: failed to load rule %d: %s", event.RuleID, err.Error()), map[string]interface{}{})
			return
		}
		if !rule.Enabled {
			s.cancel(event.RuleID)
			return
		}
		s.scheduleAt(event.RuleID, time.Now())
	case "deleted":
		s.cancel(event.RuleID)
	default:
		log.Error(fmt.Sprintf("scheduler: unknown rule event %q for rule %d", event.Event, event.RuleID), map[string]interface{}{})
	}
}

// HandleDeviceUpdate is called for every device attribute update received from
// the event bus. It finds rules that reference the updated device. If the matching
// condition has a cooldown, the rule is scheduled to re-evaluate after the cooldown
// expires; otherwise it is evaluated immediately.
func (s *Scheduler) HandleDeviceUpdate(update eventmodels.DeviceAttributeUpdate) {
	rules, err := s.db.GetRules()
	if err != nil {
		log.Error(fmt.Sprintf("scheduler: failed to load rules for device update: %s", err.Error()), map[string]interface{}{})
		return
	}
	for _, rule := range rules {
		if !rule.Enabled || rule.ConditionTree == nil {
			continue
		}
		if !referencesDevice(*rule.ConditionTree, update.DeviceID) {
			continue
		}
		maxCooldown := rule.ConditionTree.MaxCooldownForDevice(update.DeviceID)
		if maxCooldown > 0 {
			cooldownUntil := time.Now().Add(time.Duration(maxCooldown) * time.Second)
			if rule.CooldownUntil != nil && rule.CooldownUntil.After(cooldownUntil) {
				cooldownUntil = *rule.CooldownUntil
			}
			if err := s.db.UpdateCooldownUntil(rule.ID, &cooldownUntil); err != nil {
				log.Error(fmt.Sprintf("scheduler: failed to set cooldown for rule %d: %s", rule.ID, err.Error()), map[string]interface{}{})
				continue
			}
			log.Info(fmt.Sprintf("scheduler: rule %d cooldown set, will re-evaluate at %s", rule.ID, cooldownUntil.UTC().Format(time.RFC3339)), map[string]interface{}{})
			s.scheduleAt(rule.ID, cooldownUntil)
		} else {
			s.evaluate(rule.ID, time.Now())
		}
	}
}

// scheduleAt sets (or resets) the timer for ruleID to fire at the given time.
func (s *Scheduler) scheduleAt(ruleID int, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[ruleID]; ok {
		t.Stop()
	}
	delay := time.Until(at)
	if delay < 0 {
		delay = 0
	}
	log.Info(
		fmt.Sprintf("scheduling rule %d in %s (at %s)", ruleID, delay.Round(time.Second), at.UTC().Format(time.RFC3339)),
		map[string]interface{}{},
	)
	s.timers[ruleID] = time.AfterFunc(delay, func() { s.evaluate(ruleID, at) })
}

// cancel stops and removes the timer for ruleID.
func (s *Scheduler) cancel(ruleID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[ruleID]; ok {
		t.Stop()
		delete(s.timers, ruleID)
	}
}

// evaluate assesses a rule at the given time, triggers actions if conditions are
// met, persists the new next_occurrence, and reschedules the timer if needed.
func (s *Scheduler) evaluate(ruleID int, evalTime time.Time) {
	result, err := s.orch.EvaluateAndTrigger(ruleID, evalTime)
	if err != nil {
		log.Error(fmt.Sprintf("scheduler: evaluation of rule %d failed: %s", ruleID, err.Error()), map[string]interface{}{})
		return
	}
	log.Info(fmt.Sprintf("scheduler: rule %d evaluated: result=%v", ruleID, result.Result), map[string]interface{}{})
	if err := s.db.UpdateNextOccurrence(ruleID, result.NextOccurrence); err != nil {
		log.Error(fmt.Sprintf("scheduler: failed to persist next occurrence for rule %d: %s", ruleID, err.Error()), map[string]interface{}{})
	}
	if result.NextOccurrence != nil {
		s.scheduleAt(ruleID, *result.NextOccurrence)
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
