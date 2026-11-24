package mariadb

import (
	"testing"

	"github.com/Kaese72/huemie-lib/query"
	"github.com/Kaese72/ittt-orchestrator/internal/scheduler"
)

// These exercise ruleFilters/ruleSortFields directly (no DB needed, since
// query.Translate/BuildOrderBy are pure functions) - they catch a typo'd
// column name or operator wiring without needing testcontainers/Docker.
func TestRuleFilters(t *testing.T) {
	t.Run("name text-contains", func(t *testing.T) {
		fragments, args, err := query.Translate([]query.Filter{
			{Field: "name", Operator: "text-contains", Value: "kitchen"},
		}, ruleFilters)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fragments) != 1 || len(args) != 1 {
			t.Fatalf("unexpected result: fragments=%v args=%v", fragments, args)
		}
	})

	t.Run("enabled bool-eq", func(t *testing.T) {
		fragments, args, err := query.Translate([]query.Filter{
			{Field: "enabled", Operator: "bool-eq", Value: "true"},
		}, ruleFilters)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fragments) != 1 || len(args) != 1 || args[0] != true {
			t.Fatalf("unexpected result: fragments=%v args=%v", fragments, args)
		}
	})

	for field, column := range map[string]string{"related-devices": "related_devices", "related-groups": "related_groups"} {
		t.Run(field+" in", func(t *testing.T) {
			fragments, args, err := query.Translate([]query.Filter{
				{Field: field, Operator: "in", Value: "3, 4,5"},
			}, ruleFilters)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(fragments) != 1 || fragments[0] != "JSON_OVERLAPS("+column+", ?)" {
				t.Fatalf("unexpected fragment: %v", fragments)
			}
			// The IDs go in as one JSON array parameter, never into the SQL.
			if len(args) != 1 || args[0] != "[3,4,5]" {
				t.Fatalf("unexpected args: %v", args)
			}
		})
		for _, bad := range []string{"", "abc", "3,", "0", "-1", "3;DROP TABLE rules"} {
			t.Run(field+" rejects "+bad, func(t *testing.T) {
				if _, _, err := query.Translate([]query.Filter{
					{Field: field, Operator: "in", Value: bad},
				}, ruleFilters); err == nil {
					t.Fatalf("expected an error for %q", bad)
				}
			})
		}
	}

	t.Run("unknown field", func(t *testing.T) {
		if _, _, err := query.Translate([]query.Filter{
			{Field: "root_condition_id", Operator: "eq", Value: "1"},
		}, ruleFilters); err == nil {
			t.Fatal("expected an error for an unfiltered field")
		}
	})
}

func TestRuleSortFields(t *testing.T) {
	t.Run("empty falls back to id", func(t *testing.T) {
		clause, err := query.BuildOrderBy(nil, ruleSortFields, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if clause != "id" {
			t.Fatalf("expected fallback \"id\", got %q", clause)
		}
	})

	t.Run("sort by name desc", func(t *testing.T) {
		clause, err := query.BuildOrderBy([]query.Sort{{Field: "name", Direction: "desc"}}, ruleSortFields, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if clause != "name DESC" {
			t.Fatalf("unexpected clause: %q", clause)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		if _, err := query.BuildOrderBy([]query.Sort{{Field: "root_condition_id", Direction: "asc"}}, ruleSortFields, "id"); err == nil {
			t.Fatal("expected an error for an unsortable field")
		}
	})
}

// The scheduler filters rules for each device update through these same
// fields; a renamed field or operator here would otherwise only fail at
// runtime, on every device update.
func TestDeviceUpdateFiltersAccepted(t *testing.T) {
	if _, _, err := query.Translate(scheduler.DeviceUpdateFilters(7), ruleFilters); err != nil {
		t.Fatalf("scheduler's device-update filters rejected: %v", err)
	}
}

func TestStartupFiltersAccepted(t *testing.T) {
	if _, _, err := query.Translate(scheduler.StartupFilters(), ruleFilters); err != nil {
		t.Fatalf("scheduler's startup filters rejected: %v", err)
	}
}

func TestNextOccurrenceFilters(t *testing.T) {
	for _, tc := range []struct {
		operator, value, fragment string
		args                      int
	}{
		{"is-null", "true", "next_occurence IS NULL", 0},
		{"is-null", "false", "next_occurence IS NOT NULL", 0},
		{"date-gt", "2026-11-25T06:00:00Z", "next_occurence > ?", 1},
		{"date-lt", "2026-11-25T06:00:00+01:00", "next_occurence < ?", 1},
	} {
		fragments, args, err := query.Translate([]query.Filter{{Field: "next-occurence", Operator: tc.operator, Value: tc.value}}, ruleFilters)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.operator, tc.value, err)
		}
		if len(fragments) != 1 || fragments[0] != tc.fragment || len(args) != tc.args {
			t.Fatalf("%s %s: got %v %v", tc.operator, tc.value, fragments, args)
		}
	}
	for _, bad := range []query.Filter{
		{Field: "next-occurence", Operator: "is-null", Value: "maybe"},
		{Field: "next-occurence", Operator: "date-gt", Value: "tomorrow"},
	} {
		if _, _, err := query.Translate([]query.Filter{bad}, ruleFilters); err == nil {
			t.Fatalf("expected an error for %+v", bad)
		}
	}
}
