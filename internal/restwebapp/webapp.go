package restwebapp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/eventmodels"
	"github.com/Kaese72/ittt-orchestrator/internal/events"
	"github.com/Kaese72/ittt-orchestrator/internal/orchestrator"
	"github.com/Kaese72/ittt-orchestrator/internal/persistence"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
	"github.com/danielgtaylor/huma/v2"
)

func internalError(err error) error {
	log.Error(err.Error(), map[string]interface{}{})
	return huma.Error500InternalServerError(err.Error())
}

type WebApp struct {
	db        persistence.PersistenceDB
	orch      *orchestrator.Orchestrator
	publisher *events.RuleEventPublisher
}

func NewWebApp(db persistence.PersistenceDB, orch *orchestrator.Orchestrator, publisher *events.RuleEventPublisher) WebApp {
	return WebApp{db: db, orch: orch, publisher: publisher}
}

// publishRuleEvent publishes a rule event and logs but does not fail on error.
func (w WebApp) publishRuleEvent(ruleID int, event string) {
	if err := w.publisher.Publish(eventmodels.RuleEvent{RuleID: ruleID, Event: event}); err != nil {
		log.Error(fmt.Sprintf("failed to publish rule event for rule %d: %s", ruleID, err.Error()), map[string]interface{}{})
	}
}

// GetRules returns all automation rules
func (w WebApp) GetRules(ctx context.Context, _ *struct{}) (*struct{ Body []restmodels.Rule }, error) {
	rules, err := w.db.GetRules()
	if err != nil {
		return nil, internalError(err)
	}
	return &struct{ Body []restmodels.Rule }{Body: rules}, nil
}

// GetRule returns a single rule by ID
func (w WebApp) GetRule(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
}) (*struct{ Body restmodels.Rule }, error) {
	rule, err := w.db.GetRule(input.RuleID)
	if err != nil {
		return nil, internalError(err)
	}
	return &struct{ Body restmodels.Rule }{Body: rule}, nil
}

// CreateRule creates a new automation rule
func (w WebApp) CreateRule(ctx context.Context, input *struct {
	Body restmodels.Rule
}) (*struct {
	Body restmodels.Rule
}, error) {
	created, err := w.db.CreateRule(input.Body)
	if err != nil {
		return nil, internalError(err)
	}
	evalResult, err := w.orch.EvaluateConditionTree(created.ID)
	if err != nil {
		return nil, internalError(err)
	}
	if err := w.db.UpdateNextOccurrence(created.ID, evalResult.NextOccurrence); err != nil {
		return nil, internalError(err)
	}
	w.publishRuleEvent(created.ID, "upsert")
	return &struct{ Body restmodels.Rule }{Body: created}, nil
}

// UpdateRule replaces an existing rule
func (w WebApp) UpdateRule(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
	Body   restmodels.Rule
}) (*struct{ Body restmodels.Rule }, error) {
	updated, err := w.db.UpdateRule(input.RuleID, input.Body)
	if err != nil {
		return nil, internalError(err)
	}
	evalResult, err := w.orch.EvaluateConditionTree(updated.ID)
	if err != nil {
		return nil, internalError(err)
	}
	if err := w.db.UpdateNextOccurrence(updated.ID, evalResult.NextOccurrence); err != nil {
		return nil, internalError(err)
	}
	w.publishRuleEvent(updated.ID, "upsert")
	return &struct{ Body restmodels.Rule }{Body: updated}, nil
}

// DeleteRule deletes a rule by ID
func (w WebApp) DeleteRule(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
}) (*struct{}, error) {
	if err := w.db.DeleteRule(input.RuleID); err != nil {
		return nil, internalError(err)
	}
	w.publishRuleEvent(input.RuleID, "deleted")
	return nil, nil
}

// GetActions returns all actions for a rule
func (w WebApp) GetActions(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
}) (*struct{ Body []restmodels.Action }, error) {
	actions, err := w.db.GetActions(input.RuleID)
	if err != nil {
		return nil, internalError(err)
	}
	return &struct{ Body []restmodels.Action }{Body: actions}, nil
}

// GetAction returns a single action by ID
func (w WebApp) GetAction(ctx context.Context, input *struct {
	RuleID   int `path:"ruleID"`
	ActionID int `path:"actionID"`
}) (*struct{ Body restmodels.Action }, error) {
	action, err := w.db.GetAction(input.RuleID, input.ActionID)
	if err != nil {
		return nil, internalError(err)
	}
	return &struct{ Body restmodels.Action }{Body: action}, nil
}

// CreateAction adds a new action to a rule
func (w WebApp) CreateAction(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
	Body   restmodels.Action
}) (*struct{ Body restmodels.Action }, error) {
	created, err := w.db.CreateAction(input.RuleID, input.Body)
	if err != nil {
		return nil, internalError(err)
	}
	return &struct{ Body restmodels.Action }{Body: created}, nil
}

// UpdateAction replaces an existing action
func (w WebApp) UpdateAction(ctx context.Context, input *struct {
	RuleID   int `path:"ruleID"`
	ActionID int `path:"actionID"`
	Body     restmodels.Action
}) (*struct{ Body restmodels.Action }, error) {
	updated, err := w.db.UpdateAction(input.RuleID, input.ActionID, input.Body)
	if err != nil {
		return nil, internalError(err)
	}
	return &struct{ Body restmodels.Action }{Body: updated}, nil
}

// DeleteAction removes an action from a rule
func (w WebApp) DeleteAction(ctx context.Context, input *struct {
	RuleID   int `path:"ruleID"`
	ActionID int `path:"actionID"`
}) (*struct{}, error) {
	if err := w.db.DeleteAction(input.RuleID, input.ActionID); err != nil {
		return nil, internalError(err)
	}
	return nil, nil
}

// EvaluateRuleOutput is the response body for the evaluate endpoint
type EvaluateRuleOutput struct {
	Result         bool       `json:"result"`
	Reason         string     `json:"reason,omitempty"`
	NextOccurrence *time.Time `json:"next-occurrence,omitempty"`
}

// EvaluateRule evaluates the condition tree of a rule against the current state
func (w WebApp) EvaluateRule(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
}) (*struct{ Body EvaluateRuleOutput }, error) {
	evalResult, err := w.orch.EvaluateConditionTree(input.RuleID)
	if err != nil {
		return nil, internalError(err)
	}
	return &struct{ Body EvaluateRuleOutput }{Body: EvaluateRuleOutput{
		Result:         evalResult.Result,
		Reason:         evalResult.Reason,
		NextOccurrence: evalResult.NextOccurrence,
	}}, nil
}

// StatusOutput is the response body for the health endpoint
type StatusOutput struct {
	Status string `json:"status"`
}

// GetStatus returns the service health status
func (w WebApp) GetStatus(_ context.Context, _ *struct{}) (*struct{ Body StatusOutput }, error) {
	return &struct{ Body StatusOutput }{Body: StatusOutput{Status: "ok"}}, nil
}

// deleteResponse is returned with a 204 No Content for deletes
func deleteResponse() *huma.ErrorModel {
	return &huma.ErrorModel{Status: http.StatusNoContent}
}
