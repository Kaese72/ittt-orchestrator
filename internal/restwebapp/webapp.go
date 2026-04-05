package restwebapp

import (
	"context"
	"net/http"

	"github.com/Kaese72/ittt-orchestrator/internal/persistence"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
	"github.com/danielgtaylor/huma/v2"
)

type WebApp struct {
	db persistence.PersistenceDB
}

func NewWebApp(db persistence.PersistenceDB) WebApp {
	return WebApp{db: db}
}

// GetRules returns all automation rules
func (w WebApp) GetRules(ctx context.Context, _ *struct{}) (*struct{ Body []restmodels.Rule }, error) {
	rules, err := w.db.GetRules()
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &struct{ Body []restmodels.Rule }{Body: rules}, nil
}

// GetRule returns a single rule by ID
func (w WebApp) GetRule(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
}) (*struct{ Body restmodels.Rule }, error) {
	rule, err := w.db.GetRule(input.RuleID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
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
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &struct{ Body restmodels.Rule }{Body: created}, nil
}

// UpdateRule replaces an existing rule
func (w WebApp) UpdateRule(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
	Body   restmodels.Rule
}) (*struct{ Body restmodels.Rule }, error) {
	updated, err := w.db.UpdateRule(input.RuleID, input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &struct{ Body restmodels.Rule }{Body: updated}, nil
}

// DeleteRule deletes a rule by ID
func (w WebApp) DeleteRule(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
}) (*struct{}, error) {
	if err := w.db.DeleteRule(input.RuleID); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

// GetActions returns all actions for a rule
func (w WebApp) GetActions(ctx context.Context, input *struct {
	RuleID int `path:"ruleID"`
}) (*struct{ Body []restmodels.Action }, error) {
	actions, err := w.db.GetActions(input.RuleID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
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
		return nil, huma.Error500InternalServerError(err.Error())
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
		return nil, huma.Error500InternalServerError(err.Error())
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
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &struct{ Body restmodels.Action }{Body: updated}, nil
}

// DeleteAction removes an action from a rule
func (w WebApp) DeleteAction(ctx context.Context, input *struct {
	RuleID   int `path:"ruleID"`
	ActionID int `path:"actionID"`
}) (*struct{}, error) {
	if err := w.db.DeleteAction(input.RuleID, input.ActionID); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
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
