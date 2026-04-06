package restmodels

// ConditionTree is a node in the logical expression tree.
// Each node holds a condition check and optional AND/OR child nodes.
type ConditionTree struct {
	Condition Condition      `json:"condition"`
	And       *ConditionTree `json:"and,omitempty"`
	Or        *ConditionTree `json:"or,omitempty"`
}

// Condition is a single boolean check within a ConditionTree node.
type Condition struct {
	Type string `json:"type"`
	// Used by time-range
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// Used by device-id-attribute-boolean-eq
	ID        int    `json:"id,omitempty"`
	Attribute string `json:"attribute,omitempty"`
	Boolean   *bool  `json:"boolean,omitempty"`
}

// Action describes a capability trigger that fires when a rule's conditions are true.
type Action struct {
	// ActionID is the resource identifier used when managing actions individually.
	ActionID   int            `json:"action-id,omitempty"`
	Type       string         `json:"type"`
	ID         int            `json:"id"`
	Capability string         `json:"capability"`
	Args       map[string]any `json:"args,omitempty"`
}

// Rule is an ITTT automation rule.
type Rule struct {
	ID            int            `json:"id,omitempty"`
	Name          string         `json:"name"`
	Enabled       bool           `json:"enabled"`
	ConditionTree *ConditionTree `json:"condition-tree,omitempty"`
	Actions       []Action       `json:"actions,omitempty"`
}
