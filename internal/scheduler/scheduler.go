package scheduler

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/eventmodels"
	"github.com/Kaese72/ittt-orchestrator/internal/orchestrator"
	"github.com/Kaese72/ittt-orchestrator/internal/persistence"
)

// Scheduler maintains per-rule timers and fires rule evaluation at each rule's
// next occurrence. It is the sole owner of scheduling state and is updated
// exclusively through RabbitMQ rule events.
type Scheduler struct {
	db   persistence.PersistenceDB
	orch *orchestrator.Orchestrator
	mu   sync.Mutex
	timers map[int]*time.Timer
}

func New(db persistence.PersistenceDB, orch *orchestrator.Orchestrator) *Scheduler {
	return &Scheduler{
		db:     db,
		orch:   orch,
		timers: make(map[int]*time.Timer),
	}
}

// Start seeds the initial schedule from the next_occurence stored in the database.
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
// created, updated, or deleted.
func (s *Scheduler) HandleRuleEvent(event eventmodels.RuleEvent) {
	switch event.Event {
	case "upsert":
		rule, err := s.db.GetRule(event.RuleID)
		if err != nil {
			log.Error(fmt.Sprintf("scheduler: failed to load rule %d: %s", event.RuleID, err.Error()), map[string]interface{}{})
			return
		}
		if !rule.Enabled || rule.NextOccurrence == nil {
			s.cancel(event.RuleID)
			return
		}
		s.scheduleAt(event.RuleID, *rule.NextOccurrence)
	case "deleted":
		s.cancel(event.RuleID)
	default:
		log.Error(fmt.Sprintf("scheduler: unknown rule event %q for rule %d", event.Event, event.RuleID), map[string]interface{}{})
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
	s.timers[ruleID] = time.AfterFunc(delay, func() { s.fire(ruleID) })
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

// fire is called when a rule's timer expires. It evaluates the rule, triggers
// actions if the conditions are met, and reschedules for the next occurrence.
func (s *Scheduler) fire(ruleID int) {
	result, err := s.orch.EvaluateAndTrigger(ruleID)
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
