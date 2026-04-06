package eventmodels

// RuleEvent is published whenever a rule is created, updated, or deleted.
type RuleEvent struct {
	RuleID int    `json:"rule-id"`
	Event  string `json:"event"` // "upsert" or "deleted"
}
