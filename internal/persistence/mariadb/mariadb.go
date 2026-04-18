package mariadb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Kaese72/ittt-orchestrator/internal/config"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
	"github.com/danielgtaylor/huma/v2"
	_ "github.com/go-sql-driver/mysql"
)

type mariadbPersistence struct {
	db *sql.DB
}

func NewMariadbPersistence(conf config.DatabaseConfig) (*mariadbPersistence, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		conf.User, conf.Password, conf.Host, conf.Port, conf.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return &mariadbPersistence{db: db}, nil
}

// conditionRow holds a single row from the conditions table.
type conditionRow struct {
	ID             int
	Type           string
	FromTime       sql.NullString
	ToTime         sql.NullString
	Timezone       sql.NullString
	DeviceID       sql.NullInt64
	Attribute      sql.NullString
	Boolean        sql.NullInt64 // NULL = not set, 0 = false, 1 = true
	AndConditionID sql.NullInt64
	OrConditionID  sql.NullInt64
}

// loadConditions fetches all condition rows for a rule and returns them keyed by ID.
func (p *mariadbPersistence) loadConditions(ruleID int) (map[int]conditionRow, error) {
	rows, err := p.db.Query(`
		SELECT id, type, from_time, to_time, timezone, device_id, attribute, boolean, and_condition_id, or_condition_id
		FROM conditions WHERE rule_id = ?
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	condMap := make(map[int]conditionRow)
	for rows.Next() {
		var row conditionRow
		if err := rows.Scan(
			&row.ID, &row.Type, &row.FromTime, &row.ToTime, &row.Timezone, &row.DeviceID, &row.Attribute,
			&row.Boolean, &row.AndConditionID, &row.OrConditionID,
		); err != nil {
			return nil, err
		}
		condMap[row.ID] = row
	}
	return condMap, rows.Err()
}

// buildConditionTree reconstructs a ConditionTree from a flat map of condition rows,
// starting at rootID.
func buildConditionTree(condMap map[int]conditionRow, rootID int) *restmodels.ConditionTree {
	row, ok := condMap[rootID]
	if !ok {
		return nil
	}
	tree := &restmodels.ConditionTree{
		Condition: restmodels.Condition{Type: row.Type},
	}
	if row.FromTime.Valid {
		tree.Condition.From = row.FromTime.String
	}
	if row.ToTime.Valid {
		tree.Condition.To = row.ToTime.String
	}
	if row.Timezone.Valid {
		tree.Condition.Timezone = row.Timezone.String
	}
	if row.DeviceID.Valid {
		tree.Condition.ID = int(row.DeviceID.Int64)
	}
	if row.Attribute.Valid {
		tree.Condition.Attribute = row.Attribute.String
	}
	if row.Boolean.Valid {
		b := row.Boolean.Int64 != 0
		tree.Condition.Boolean = &b
	}
	if row.AndConditionID.Valid {
		tree.And = buildConditionTree(condMap, int(row.AndConditionID.Int64))
	}
	if row.OrConditionID.Valid {
		tree.Or = buildConditionTree(condMap, int(row.OrConditionID.Int64))
	}
	return tree
}

// insertConditionTree inserts the tree nodes into the conditions table using a
// post-order traversal (children first) so each parent can reference its children's IDs.
// Returns the inserted ID of the root node.
func insertConditionTree(tx *sql.Tx, ruleID int, tree *restmodels.ConditionTree) (int64, error) {
	if tree == nil {
		return 0, nil
	}

	// Insert children first to get their IDs.
	var andID, orID interface{}
	if tree.And != nil {
		id, err := insertConditionTree(tx, ruleID, tree.And)
		if err != nil {
			return 0, err
		}
		andID = id
	}
	if tree.Or != nil {
		id, err := insertConditionTree(tx, ruleID, tree.Or)
		if err != nil {
			return 0, err
		}
		orID = id
	}

	cond := tree.Condition
	result, err := tx.Exec(`
		INSERT INTO conditions (rule_id, type, from_time, to_time, timezone, device_id, attribute, boolean, and_condition_id, or_condition_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ruleID,
		cond.Type,
		emptyStringToNil(cond.From),
		emptyStringToNil(cond.To),
		emptyStringToNil(cond.Timezone),
		zeroIntToNil(cond.ID),
		emptyStringToNil(cond.Attribute),
		boolPtrToNullInt(cond.Boolean),
		andID,
		orID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// loadRule fetches a single rule, its condition tree, and its actions.
func (p *mariadbPersistence) loadRule(id int) (restmodels.Rule, error) {
	var (
		ruleName        string
		ruleEnabledInt  int
		rootConditionID sql.NullInt64
		nextOccurence   sql.NullTime
	)
	err := p.db.QueryRow(`SELECT name, enabled, root_condition_id, next_occurence FROM rules WHERE id = ?`, id).
		Scan(&ruleName, &ruleEnabledInt, &rootConditionID, &nextOccurence)
	if err == sql.ErrNoRows {
		return restmodels.Rule{}, huma.Error404NotFound(fmt.Sprintf("rule %d not found", id))
	}
	if err != nil {
		return restmodels.Rule{}, err
	}

	rule := restmodels.Rule{
		ID:      id,
		Name:    ruleName,
		Enabled: ruleEnabledInt != 0,
	}
	if nextOccurence.Valid {
		t := nextOccurence.Time
		rule.NextOccurrence = &t
	}

	if rootConditionID.Valid {
		condMap, err := p.loadConditions(id)
		if err != nil {
			return restmodels.Rule{}, err
		}
		rule.ConditionTree = buildConditionTree(condMap, int(rootConditionID.Int64))
	}

	actions, err := p.GetActions(id)
	if err != nil {
		return restmodels.Rule{}, err
	}
	rule.Actions = actions

	return rule, nil
}

func (p *mariadbPersistence) GetRules() ([]restmodels.Rule, error) {
	rows, err := p.db.Query(`SELECT id FROM rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var rules []restmodels.Rule
	for _, id := range ids {
		rule, err := p.loadRule(id)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (p *mariadbPersistence) GetRule(id int) (restmodels.Rule, error) {
	return p.loadRule(id)
}

func (p *mariadbPersistence) CreateRule(rule restmodels.Rule) (restmodels.Rule, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return restmodels.Rule{}, err
	}
	defer tx.Rollback()

	enabledInt := 0
	if rule.Enabled {
		enabledInt = 1
	}
	result, err := tx.Exec(`INSERT INTO rules (name, enabled, root_condition_id) VALUES (?, ?, NULL)`,
		rule.Name, enabledInt)
	if err != nil {
		return restmodels.Rule{}, err
	}
	ruleID, err := result.LastInsertId()
	if err != nil {
		return restmodels.Rule{}, err
	}

	var rootCondIDValue interface{}
	if rule.ConditionTree != nil {
		rootID, err := insertConditionTree(tx, int(ruleID), rule.ConditionTree)
		if err != nil {
			return restmodels.Rule{}, err
		}
		rootCondIDValue = rootID
	}
	if _, err = tx.Exec(`UPDATE rules SET root_condition_id = ? WHERE id = ?`, rootCondIDValue, ruleID); err != nil {
		return restmodels.Rule{}, err
	}

	if err := tx.Commit(); err != nil {
		return restmodels.Rule{}, err
	}
	return p.GetRule(int(ruleID))
}

func (p *mariadbPersistence) UpdateRule(id int, rule restmodels.Rule) (restmodels.Rule, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return restmodels.Rule{}, err
	}
	defer tx.Rollback()

	enabledInt := 0
	if rule.Enabled {
		enabledInt = 1
	}
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(1) FROM rules WHERE id = ?`, id).Scan(&exists); err != nil {
		return restmodels.Rule{}, err
	}
	if exists == 0 {
		return restmodels.Rule{}, huma.Error404NotFound(fmt.Sprintf("rule %d not found", id))
	}

	if _, err := tx.Exec(`UPDATE rules SET name = ?, enabled = ? WHERE id = ?`, rule.Name, enabledInt, id); err != nil {
		return restmodels.Rule{}, err
	}

	// Replace the condition tree: delete existing conditions, insert new ones.
	if _, err = tx.Exec(`DELETE FROM conditions WHERE rule_id = ?`, id); err != nil {
		return restmodels.Rule{}, err
	}

	var rootCondIDValue interface{}
	if rule.ConditionTree != nil {
		rootID, err := insertConditionTree(tx, id, rule.ConditionTree)
		if err != nil {
			return restmodels.Rule{}, err
		}
		rootCondIDValue = rootID
	}
	if _, err = tx.Exec(`UPDATE rules SET root_condition_id = ? WHERE id = ?`, rootCondIDValue, id); err != nil {
		return restmodels.Rule{}, err
	}

	if err := tx.Commit(); err != nil {
		return restmodels.Rule{}, err
	}
	return p.GetRule(id)
}

func (p *mariadbPersistence) DeleteRule(id int) error {
	result, err := p.db.Exec(`DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return huma.Error404NotFound(fmt.Sprintf("rule %d not found", id))
	}
	return nil
}

func (p *mariadbPersistence) GetActions(ruleID int) ([]restmodels.Action, error) {
	rows, err := p.db.Query(
		`SELECT id, type, target_id, capability, args FROM rule_actions WHERE rule_id = ? ORDER BY id`,
		ruleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanActions(rows)
}

func (p *mariadbPersistence) GetAction(ruleID, actionID int) (restmodels.Action, error) {
	rows, err := p.db.Query(
		`SELECT id, type, target_id, capability, args FROM rule_actions WHERE rule_id = ? AND id = ?`,
		ruleID, actionID,
	)
	if err != nil {
		return restmodels.Action{}, err
	}
	defer rows.Close()
	actions, err := scanActions(rows)
	if err != nil {
		return restmodels.Action{}, err
	}
	if len(actions) == 0 {
		return restmodels.Action{}, huma.Error404NotFound(fmt.Sprintf("action %d not found for rule %d", actionID, ruleID))
	}
	return actions[0], nil
}

func (p *mariadbPersistence) CreateAction(ruleID int, action restmodels.Action) (restmodels.Action, error) {
	argsJSON, err := marshalArgs(action.Args)
	if err != nil {
		return restmodels.Action{}, err
	}
	result, err := p.db.Exec(
		`INSERT INTO rule_actions (rule_id, type, target_id, capability, args) VALUES (?, ?, ?, ?, ?)`,
		ruleID, action.Type, action.ID, action.Capability, argsJSON,
	)
	if err != nil {
		return restmodels.Action{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return restmodels.Action{}, err
	}
	return p.GetAction(ruleID, int(id))
}

func (p *mariadbPersistence) UpdateAction(ruleID, actionID int, action restmodels.Action) (restmodels.Action, error) {
	argsJSON, err := marshalArgs(action.Args)
	if err != nil {
		return restmodels.Action{}, err
	}
	var exists int
	if err := p.db.QueryRow(`SELECT COUNT(1) FROM rule_actions WHERE id = ? AND rule_id = ?`, actionID, ruleID).Scan(&exists); err != nil {
		return restmodels.Action{}, err
	}
	if exists == 0 {
		return restmodels.Action{}, huma.Error404NotFound(fmt.Sprintf("action %d not found for rule %d", actionID, ruleID))
	}

	if _, err := p.db.Exec(
		`UPDATE rule_actions SET type = ?, target_id = ?, capability = ?, args = ? WHERE id = ? AND rule_id = ?`,
		action.Type, action.ID, action.Capability, argsJSON, actionID, ruleID,
	); err != nil {
		return restmodels.Action{}, err
	}
	return p.GetAction(ruleID, actionID)
}

func (p *mariadbPersistence) DeleteAction(ruleID, actionID int) error {
	result, err := p.db.Exec(`DELETE FROM rule_actions WHERE id = ? AND rule_id = ?`, actionID, ruleID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return huma.Error404NotFound(fmt.Sprintf("action %d not found for rule %d", actionID, ruleID))
	}
	return nil
}

func scanActions(rows *sql.Rows) ([]restmodels.Action, error) {
	var actions []restmodels.Action
	for rows.Next() {
		var (
			id         int
			actionType string
			targetID   int
			capability string
			argsJSON   []byte
		)
		if err := rows.Scan(&id, &actionType, &targetID, &capability, &argsJSON); err != nil {
			return nil, err
		}
		var args map[string]any
		if argsJSON != nil {
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return nil, err
			}
		}
		actions = append(actions, restmodels.Action{
			ActionID:   id,
			Type:       actionType,
			ID:         targetID,
			Capability: capability,
			Args:       args,
		})
	}
	return actions, rows.Err()
}

func (p *mariadbPersistence) UpdateNextOccurrence(ruleID int, t *time.Time) error {
	var val interface{}
	if t != nil {
		val = t.UTC()
	}
	_, err := p.db.Exec(`UPDATE rules SET next_occurence = ? WHERE id = ?`, val, ruleID)
	return err
}

func emptyStringToNil(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func zeroIntToNil(i int) interface{} {
	if i == 0 {
		return nil
	}
	return i
}

func boolPtrToNullInt(b *bool) interface{} {
	if b == nil {
		return nil
	}
	if *b {
		return 1
	}
	return 0
}

func marshalArgs(args map[string]any) ([]byte, error) {
	if args == nil {
		return nil, nil
	}
	return json.Marshal(args)
}
