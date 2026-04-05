package persistence

import "github.com/Kaese72/ittt-orchestrator/restmodels"

// PersistenceDB is the interface all persistence implementations must satisfy
type PersistenceDB interface {
	GetRules() ([]restmodels.Rule, error)
	GetRule(id int) (restmodels.Rule, error)
	CreateRule(rule restmodels.Rule) (restmodels.Rule, error)
	UpdateRule(id int, rule restmodels.Rule) (restmodels.Rule, error)
	DeleteRule(id int) error

	GetActions(ruleID int) ([]restmodels.Action, error)
	GetAction(ruleID, actionID int) (restmodels.Action, error)
	CreateAction(ruleID int, action restmodels.Action) (restmodels.Action, error)
	UpdateAction(ruleID, actionID int, action restmodels.Action) (restmodels.Action, error)
	DeleteAction(ruleID, actionID int) error
}
