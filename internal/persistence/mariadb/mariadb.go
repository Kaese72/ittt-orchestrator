package mariadb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Kaese72/ittt-orchestrator/internal/config"
	"github.com/Kaese72/ittt-orchestrator/internal/persistence"
	"github.com/Kaese72/ittt-orchestrator/restmodels"
	// aliased: GetRules (below) builds its SQL as a local "query" variable,
	// which would otherwise shadow the package.
	libquery "github.com/Kaese72/huemie-lib/query"
	"github.com/Kaese72/huemie-lib/telemetry"
	"github.com/danielgtaylor/huma/v2"
	_ "github.com/go-sql-driver/mysql"
)

// ruleFilters defines what filters are available for the rules model.
var ruleFilters = map[string]libquery.FieldSpec{
	"name":            libquery.Merge(libquery.TextOperators("name")),
	"enabled":         libquery.Merge(libquery.BoolOperator("enabled")),
	"related-devices": libquery.Merge(relatedInOperator("related_devices")),
	"related-groups":  libquery.Merge(relatedInOperator("related_groups")),
	// date-eq/date-lt/date-gt against an RFC3339 value, plus is-null for
	// "is (not) scheduled at all".
	"next-occurence": libquery.Merge(libquery.DateOperators("next_occurence"), nullOperator("next_occurence")),
}

// nullOperator returns an "is-null" operator for column: value "true"
// matches rows where it is NULL, "false" rows where it is set.
func nullOperator(column string) map[string]libquery.OperatorFunc {
	return map[string]libquery.OperatorFunc{
		"is-null": func(value string) (string, []any, error) {
			switch value {
			case "true":
				return column + " IS NULL", nil, nil
			case "false":
				return column + " IS NOT NULL", nil, nil
			default:
				return "", nil, huma.Error400BadRequest(fmt.Sprintf("filter value %q must be \"true\" or \"false\"", value))
			}
		},
	}
}

// relatedInOperator returns an "in" operator matching rules whose JSON ID
// array column shares at least one ID with a comma-separated list -- the same
// {operator: "in", value: "3,4,5"} shape the UI's id-list filter sends for
// the device and group tables' own ID columns. JSON_OVERLAPS can't use an
// index, so this scans every rule; fine for the number of rules a home has.
func relatedInOperator(column string) map[string]libquery.OperatorFunc {
	return map[string]libquery.OperatorFunc{
		"in": func(value string) (string, []any, error) {
			ids := []int{}
			for _, raw := range strings.Split(value, ",") {
				id, err := strconv.Atoi(strings.TrimSpace(raw))
				if err != nil || id <= 0 {
					return "", nil, huma.Error400BadRequest(fmt.Sprintf("filter value %q must be a comma-separated list of positive integer IDs", value))
				}
				ids = append(ids, id)
			}
			// Marshalled from validated ints, so the parameter is always a
			// JSON array of numbers.
			idsJSON, err := json.Marshal(ids)
			if err != nil {
				return "", nil, err
			}
			return "JSON_OVERLAPS(" + column + ", ?)", []any{string(idsJSON)}, nil
		},
	}
}

// ruleSortFields are the fields "sort" may reference for GetRules. The rules
// table has no timestamp columns, so id/name are the only candidates.
var ruleSortFields = map[string]string{
	"id":   "id",
	"name": "name",
}

type mariadbPersistence struct {
	db *sql.DB
}

func NewMariadbPersistence(conf config.DatabaseConfig) (*mariadbPersistence, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		conf.User, conf.Password, conf.Host, conf.Port, conf.Database)
	db, err := telemetry.OpenDB("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return &mariadbPersistence{db: db}, nil
}

// conditionRow holds a single row from the conditions table.
type conditionRow struct {
	ID              int
	Type            string
	FromTime        sql.NullString
	ToTime          sql.NullString
	Timezone        string
	Days            sql.NullInt64 // bitmask of active weekdays (bit N = time.Weekday(N)), used by time-range-days
	DeviceID        sql.NullInt64
	Attribute       sql.NullString
	Boolean         sql.NullInt64   // NULL = not set, 0 = false, 1 = true
	NumericValue    sql.NullFloat64 // for device-id-attribute-number-*
	NumericMargin   sql.NullFloat64 // for device-id-attribute-number-eq-margin
	TextValue       sql.NullString  // for device-id-attribute-text-*
	CooldownSeconds sql.NullInt64
	AndConditionID  sql.NullInt64
	OrConditionID   sql.NullInt64
}

// loadConditions fetches all condition rows for a rule and returns them keyed by ID.
func (p *mariadbPersistence) loadConditions(ruleID int) (map[int]conditionRow, error) {
	rows, err := p.db.Query(`
		SELECT id, type, from_time, to_time, timezone, days, device_id, attribute, boolean,
		       numeric_value, numeric_margin, text_value, cooldown_seconds,
		       and_condition_id, or_condition_id
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
			&row.ID, &row.Type, &row.FromTime, &row.ToTime, &row.Timezone, &row.Days, &row.DeviceID,
			&row.Attribute, &row.Boolean,
			&row.NumericValue, &row.NumericMargin, &row.TextValue, &row.CooldownSeconds,
			&row.AndConditionID, &row.OrConditionID,
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

	var cond restmodels.Condition
	switch row.Type {
	case "time-range":
		c := restmodels.TimeRangeCondition{
			Type:     row.Type,
			Timezone: row.Timezone,
		}
		if row.FromTime.Valid {
			c.From = row.FromTime.String
		}
		if row.ToTime.Valid {
			c.To = row.ToTime.String
		}
		cond = c
	case "time-range-days":
		c := restmodels.TimeRangeDaysCondition{
			Type:     row.Type,
			Timezone: row.Timezone,
		}
		if row.FromTime.Valid {
			c.From = row.FromTime.String
		}
		if row.ToTime.Valid {
			c.To = row.ToTime.String
		}
		if row.Days.Valid {
			c.Days = bitmaskToDays(uint8(row.Days.Int64))
		}
		cond = c
	case "device-id-attribute-boolean-eq":
		c := restmodels.DeviceAttributeBooleanEqCondition{Type: row.Type}
		if row.DeviceID.Valid {
			c.ID = int(row.DeviceID.Int64)
		}
		if row.Attribute.Valid {
			c.Attribute = row.Attribute.String
		}
		if row.Boolean.Valid {
			c.Boolean = row.Boolean.Int64 != 0
		}
		if row.CooldownSeconds.Valid {
			v := row.CooldownSeconds.Int64
			c.CooldownSeconds = &v
		}
		cond = c
	case "device-id-attribute-number-eq":
		c := restmodels.DeviceAttributeNumberEqCondition{Type: row.Type}
		if row.DeviceID.Valid {
			c.ID = int(row.DeviceID.Int64)
		}
		if row.Attribute.Valid {
			c.Attribute = row.Attribute.String
		}
		if row.NumericValue.Valid {
			c.Value = row.NumericValue.Float64
		}
		if row.CooldownSeconds.Valid {
			v := row.CooldownSeconds.Int64
			c.CooldownSeconds = &v
		}
		cond = c
	case "device-id-attribute-number-eq-margin":
		c := restmodels.DeviceAttributeNumberEqMarginCondition{Type: row.Type}
		if row.DeviceID.Valid {
			c.ID = int(row.DeviceID.Int64)
		}
		if row.Attribute.Valid {
			c.Attribute = row.Attribute.String
		}
		if row.NumericValue.Valid {
			c.Value = row.NumericValue.Float64
		}
		if row.NumericMargin.Valid {
			c.Margin = row.NumericMargin.Float64
		}
		if row.CooldownSeconds.Valid {
			v := row.CooldownSeconds.Int64
			c.CooldownSeconds = &v
		}
		cond = c
	case "device-id-attribute-number-lt":
		c := restmodels.DeviceAttributeNumberLtCondition{Type: row.Type}
		if row.DeviceID.Valid {
			c.ID = int(row.DeviceID.Int64)
		}
		if row.Attribute.Valid {
			c.Attribute = row.Attribute.String
		}
		if row.NumericValue.Valid {
			c.Value = row.NumericValue.Float64
		}
		if row.CooldownSeconds.Valid {
			v := row.CooldownSeconds.Int64
			c.CooldownSeconds = &v
		}
		cond = c
	case "device-id-attribute-number-gt":
		c := restmodels.DeviceAttributeNumberGtCondition{Type: row.Type}
		if row.DeviceID.Valid {
			c.ID = int(row.DeviceID.Int64)
		}
		if row.Attribute.Valid {
			c.Attribute = row.Attribute.String
		}
		if row.NumericValue.Valid {
			c.Value = row.NumericValue.Float64
		}
		if row.CooldownSeconds.Valid {
			v := row.CooldownSeconds.Int64
			c.CooldownSeconds = &v
		}
		cond = c
	case "device-id-attribute-number-lte":
		c := restmodels.DeviceAttributeNumberLteCondition{Type: row.Type}
		if row.DeviceID.Valid {
			c.ID = int(row.DeviceID.Int64)
		}
		if row.Attribute.Valid {
			c.Attribute = row.Attribute.String
		}
		if row.NumericValue.Valid {
			c.Value = row.NumericValue.Float64
		}
		if row.CooldownSeconds.Valid {
			v := row.CooldownSeconds.Int64
			c.CooldownSeconds = &v
		}
		cond = c
	case "device-id-attribute-number-gte":
		c := restmodels.DeviceAttributeNumberGteCondition{Type: row.Type}
		if row.DeviceID.Valid {
			c.ID = int(row.DeviceID.Int64)
		}
		if row.Attribute.Valid {
			c.Attribute = row.Attribute.String
		}
		if row.NumericValue.Valid {
			c.Value = row.NumericValue.Float64
		}
		if row.CooldownSeconds.Valid {
			v := row.CooldownSeconds.Int64
			c.CooldownSeconds = &v
		}
		cond = c
	case "device-id-attribute-text-eq":
		c := restmodels.DeviceAttributeTextEqCondition{Type: row.Type}
		if row.DeviceID.Valid {
			c.ID = int(row.DeviceID.Int64)
		}
		if row.Attribute.Valid {
			c.Attribute = row.Attribute.String
		}
		if row.TextValue.Valid {
			c.Value = row.TextValue.String
		}
		if row.CooldownSeconds.Valid {
			v := row.CooldownSeconds.Int64
			c.CooldownSeconds = &v
		}
		cond = c
	case "device-id-attribute-text-substring":
		c := restmodels.DeviceAttributeTextSubstringCondition{Type: row.Type}
		if row.DeviceID.Valid {
			c.ID = int(row.DeviceID.Int64)
		}
		if row.Attribute.Valid {
			c.Attribute = row.Attribute.String
		}
		if row.TextValue.Valid {
			c.Value = row.TextValue.String
		}
		if row.CooldownSeconds.Valid {
			v := row.CooldownSeconds.Int64
			c.CooldownSeconds = &v
		}
		cond = c
	default:
		return nil
	}

	tree := &restmodels.ConditionTree{
		Condition: restmodels.NewConditionUnion(cond),
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

	var (
		condType        string
		fromTime        interface{}
		toTime          interface{}
		timezone        string
		days            interface{}
		deviceID        interface{}
		attribute       interface{}
		boolean         interface{}
		numericValue    interface{}
		numericMargin   interface{}
		textValue       interface{}
		cooldownSeconds interface{}
	)
	switch c := tree.Condition.Value().(type) {
	case restmodels.TimeRangeCondition:
		condType = "time-range"
		fromTime = emptyStringToNil(c.From)
		toTime = emptyStringToNil(c.To)
		timezone = timezoneOrUTC(c.Timezone)
	case restmodels.TimeRangeDaysCondition:
		condType = "time-range-days"
		fromTime = emptyStringToNil(c.From)
		toTime = emptyStringToNil(c.To)
		timezone = timezoneOrUTC(c.Timezone)
		if len(c.Days) > 0 {
			days = daysToBitmask(c.Days)
		}
	case restmodels.DeviceAttributeBooleanEqCondition:
		condType = "device-id-attribute-boolean-eq"
		timezone = "UTC"
		deviceID = zeroIntToNil(c.ID)
		attribute = emptyStringToNil(c.Attribute)
		boolean = boolToInt(c.Boolean)
		cooldownSeconds = c.CooldownSeconds
	case restmodels.DeviceAttributeNumberEqCondition:
		condType = "device-id-attribute-number-eq"
		timezone = "UTC"
		deviceID = zeroIntToNil(c.ID)
		attribute = emptyStringToNil(c.Attribute)
		numericValue = c.Value
		cooldownSeconds = c.CooldownSeconds
	case restmodels.DeviceAttributeNumberEqMarginCondition:
		condType = "device-id-attribute-number-eq-margin"
		timezone = "UTC"
		deviceID = zeroIntToNil(c.ID)
		attribute = emptyStringToNil(c.Attribute)
		numericValue = c.Value
		numericMargin = c.Margin
		cooldownSeconds = c.CooldownSeconds
	case restmodels.DeviceAttributeNumberLtCondition:
		condType = "device-id-attribute-number-lt"
		timezone = "UTC"
		deviceID = zeroIntToNil(c.ID)
		attribute = emptyStringToNil(c.Attribute)
		numericValue = c.Value
		cooldownSeconds = c.CooldownSeconds
	case restmodels.DeviceAttributeNumberGtCondition:
		condType = "device-id-attribute-number-gt"
		timezone = "UTC"
		deviceID = zeroIntToNil(c.ID)
		attribute = emptyStringToNil(c.Attribute)
		numericValue = c.Value
		cooldownSeconds = c.CooldownSeconds
	case restmodels.DeviceAttributeNumberLteCondition:
		condType = "device-id-attribute-number-lte"
		timezone = "UTC"
		deviceID = zeroIntToNil(c.ID)
		attribute = emptyStringToNil(c.Attribute)
		numericValue = c.Value
		cooldownSeconds = c.CooldownSeconds
	case restmodels.DeviceAttributeNumberGteCondition:
		condType = "device-id-attribute-number-gte"
		timezone = "UTC"
		deviceID = zeroIntToNil(c.ID)
		attribute = emptyStringToNil(c.Attribute)
		numericValue = c.Value
		cooldownSeconds = c.CooldownSeconds
	case restmodels.DeviceAttributeTextEqCondition:
		condType = "device-id-attribute-text-eq"
		timezone = "UTC"
		deviceID = zeroIntToNil(c.ID)
		attribute = emptyStringToNil(c.Attribute)
		textValue = emptyStringToNil(c.Value)
		cooldownSeconds = c.CooldownSeconds
	case restmodels.DeviceAttributeTextSubstringCondition:
		condType = "device-id-attribute-text-substring"
		timezone = "UTC"
		deviceID = zeroIntToNil(c.ID)
		attribute = emptyStringToNil(c.Attribute)
		textValue = emptyStringToNil(c.Value)
		cooldownSeconds = c.CooldownSeconds
	default:
		return 0, fmt.Errorf("unknown condition type: %T", c)
	}

	result, err := tx.Exec(`
		INSERT INTO conditions (rule_id, type, from_time, to_time, timezone, days, device_id, attribute, boolean,
		                        numeric_value, numeric_margin, text_value, cooldown_seconds,
		                        and_condition_id, or_condition_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ruleID,
		condType,
		fromTime,
		toTime,
		timezone,
		days,
		deviceID,
		attribute,
		boolean,
		numericValue,
		numericMargin,
		textValue,
		cooldownSeconds,
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
		cooldownUntil   sql.NullTime
		relatedDevices  []byte
		relatedGroups   []byte
	)
	err := p.db.QueryRow(`SELECT name, enabled, root_condition_id, next_occurence, cooldown_until, related_devices, related_groups FROM rules WHERE id = ?`, id).
		Scan(&ruleName, &ruleEnabledInt, &rootConditionID, &nextOccurence, &cooldownUntil, &relatedDevices, &relatedGroups)
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
	if cooldownUntil.Valid {
		t := cooldownUntil.Time
		rule.CooldownUntil = &t
	}
	if rule.RelatedDevices, err = decodeIDs(relatedDevices); err != nil {
		return restmodels.Rule{}, fmt.Errorf("rule %d related_devices: %w", id, err)
	}
	if rule.RelatedGroups, err = decodeIDs(relatedGroups); err != nil {
		return restmodels.Rule{}, fmt.Errorf("rule %d related_groups: %w", id, err)
	}

	if rootConditionID.Valid {
		condMap, err := p.loadConditions(id)
		if err != nil {
			return restmodels.Rule{}, err
		}
		rule.ConditionTree = buildConditionTree(condMap, int(rootConditionID.Int64))
	}

	actions, err := p.loadActions(id)
	if err != nil {
		return restmodels.Rule{}, err
	}
	rule.Actions = actions

	return rule, nil
}

// decodeIDs decodes a related_devices/related_groups JSON array, as an empty
// (not nil) slice when there are none, so the API always shows a list.
func decodeIDs(raw []byte) ([]int, error) {
	ids := []int{}
	if len(raw) == 0 {
		return ids, nil
	}
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []int{}
	}
	return ids, nil
}

// storeRelatedIDs stores the rule's derived device and group references
// (restmodels.Rule.RelatedIDs) on its row, replacing what was there. Called
// in the same transaction that writes the condition tree and actions, so the
// two can never disagree.
func storeRelatedIDs(tx *sql.Tx, ruleID int, rule restmodels.Rule) error {
	devices, groups := rule.RelatedIDs()
	devicesJSON, err := json.Marshal(devices)
	if err != nil {
		return err
	}
	groupsJSON, err := json.Marshal(groups)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE rules SET related_devices = ?, related_groups = ? WHERE id = ?`, string(devicesJSON), string(groupsJSON), ruleID)
	return err
}

// paginationClause returns the SQL "LIMIT ? OFFSET ?" fragment and its arguments
// for the given pagination. A zero Limit means unbounded, in which case no
// clause is applied.
func paginationClause(pagination libquery.Pagination) (string, []any) {
	if pagination.Limit <= 0 {
		return "", nil
	}
	offset := pagination.Offset
	if offset < 0 {
		offset = 0
	}
	return " LIMIT ? OFFSET ?", []any{pagination.Limit, offset}
}

// countRows executes "SELECT COUNT(*) FROM <table> [WHERE <whereClause>]" and
// returns the total number of matching rows, ignoring pagination.
func countRows(db *sql.DB, table string, whereClause string, args []any) (int, error) {
	q := `SELECT COUNT(*) FROM ` + table
	if whereClause != "" {
		q += " WHERE " + whereClause
	}
	var total int
	row := db.QueryRow(q, args...)
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (p *mariadbPersistence) GetRules(filters []libquery.Filter, sorts []libquery.Sort, pagination libquery.Pagination) ([]restmodels.Rule, int, error) {
	fragments, args, err := libquery.Translate(filters, ruleFilters)
	if err != nil {
		return nil, 0, err
	}
	whereClause := strings.Join(fragments, " AND ")
	total, err := countRows(p.db, "rules", whereClause, args)
	if err != nil {
		return nil, 0, err
	}
	orderBy, err := libquery.BuildOrderBy(sorts, ruleSortFields, "id")
	if err != nil {
		return nil, 0, err
	}
	query := `SELECT id FROM rules`
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " ORDER BY " + orderBy
	limitClause, limitArgs := paginationClause(pagination)
	query += limitClause
	rows, err := p.db.Query(query, append(append([]any{}, args...), limitArgs...)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var rules []restmodels.Rule
	for _, id := range ids {
		rule, err := p.loadRule(id)
		if err != nil {
			return nil, 0, err
		}
		rules = append(rules, rule)
	}
	return rules, total, nil
}

// GetRuleSchedules reads only id and next_occurence, in one query, instead of
// loading each matching rule's conditions and actions like GetRules does.
func (p *mariadbPersistence) GetRuleSchedules(filters []libquery.Filter) ([]persistence.RuleSchedule, error) {
	fragments, args, err := libquery.Translate(filters, ruleFilters)
	if err != nil {
		return nil, err
	}
	fragments = append(fragments, "next_occurence IS NOT NULL")
	rows, err := p.db.Query(`SELECT id, next_occurence FROM rules WHERE `+strings.Join(fragments, " AND ")+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	schedules := []persistence.RuleSchedule{}
	for rows.Next() {
		var schedule persistence.RuleSchedule
		if err := rows.Scan(&schedule.RuleID, &schedule.NextOccurrence); err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	return schedules, rows.Err()
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

	if err := insertActions(tx, int(ruleID), rule.Actions); err != nil {
		return restmodels.Rule{}, err
	}
	if err := storeRelatedIDs(tx, int(ruleID), rule); err != nil {
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

	// Replace the actions the same way: delete existing ones, insert the new set.
	if _, err = tx.Exec(`DELETE FROM rule_actions WHERE rule_id = ?`, id); err != nil {
		return restmodels.Rule{}, err
	}
	if err := insertActions(tx, id, rule.Actions); err != nil {
		return restmodels.Rule{}, err
	}
	if err := storeRelatedIDs(tx, id, rule); err != nil {
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

// loadActions returns all actions belonging to a rule.
func (p *mariadbPersistence) loadActions(ruleID int) ([]restmodels.Action, error) {
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

// insertActions inserts actions for a rule in the given order. Any ActionID set
// on the input is ignored; the database assigns new IDs.
func insertActions(tx *sql.Tx, ruleID int, actions []restmodels.Action) error {
	for _, action := range actions {
		argsJSON, err := marshalArgs(action.Args)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO rule_actions (rule_id, type, target_id, capability, args) VALUES (?, ?, ?, ?, ?)`,
			ruleID, action.Type, action.ID, action.Capability, argsJSON,
		); err != nil {
			return err
		}
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

func (p *mariadbPersistence) UpdateCooldownUntil(ruleID int, t *time.Time) error {
	var val interface{}
	if t != nil {
		val = t.UTC()
	}
	_, err := p.db.Exec(`UPDATE rules SET cooldown_until = ? WHERE id = ?`, val, ruleID)
	return err
}

func timezoneOrUTC(s string) string {
	if s == "" {
		return "UTC"
	}
	return s
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// orderedWeekdays defines the canonical day order used for bitmask decode.
// Bit N corresponds to time.Weekday(N): Sunday=0, Monday=1, …, Saturday=6.
var orderedWeekdays = []struct {
	name string
	wd   time.Weekday
}{
	{"monday", time.Monday},
	{"tuesday", time.Tuesday},
	{"wednesday", time.Wednesday},
	{"thursday", time.Thursday},
	{"friday", time.Friday},
	{"saturday", time.Saturday},
	{"sunday", time.Sunday},
}

func daysToBitmask(days []string) uint8 {
	nameToWd := make(map[string]time.Weekday, len(orderedWeekdays))
	for _, e := range orderedWeekdays {
		nameToWd[e.name] = e.wd
	}
	var mask uint8
	for _, d := range days {
		if wd, ok := nameToWd[d]; ok {
			mask |= 1 << uint(wd)
		}
	}
	return mask
}

func bitmaskToDays(mask uint8) []string {
	var days []string
	for _, e := range orderedWeekdays {
		if mask&(1<<uint(e.wd)) != 0 {
			days = append(days, e.name)
		}
	}
	return days
}

func marshalArgs(args map[string]any) ([]byte, error) {
	if args == nil {
		return nil, nil
	}
	return json.Marshal(args)
}
