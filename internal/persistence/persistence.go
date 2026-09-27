package persistence

import (
	"time"

	"github.com/Kaese72/huemie-lib/query"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
)

// PersistenceDB is the interface all persistence implementations must satisfy
type PersistenceDB interface {
	// GetRules returns the page of rules matching filters and ordered by
	// sorts (or the implementation's default order if sorts is empty), along
	// with the total number of rules matching filters (ignoring pagination).
	GetRules(filters []query.Filter, sorts []query.Sort, pagination query.Pagination) ([]restmodels.Rule, int, error)
	GetRule(id int) (restmodels.Rule, error)
	CreateRule(rule restmodels.Rule) (restmodels.Rule, error)
	UpdateRule(id int, rule restmodels.Rule) (restmodels.Rule, error)
	DeleteRule(id int) error
	UpdateNextOccurrence(ruleID int, t *time.Time) error
	UpdateCooldownUntil(ruleID int, t *time.Time) error

	GetActions(ruleID int) ([]restmodels.Action, error)
	GetAction(ruleID, actionID int) (restmodels.Action, error)
	CreateAction(ruleID int, action restmodels.Action) (restmodels.Action, error)
	UpdateAction(ruleID, actionID int, action restmodels.Action) (restmodels.Action, error)
	DeleteAction(ruleID, actionID int) error
}
