package mariadb

import (
	"testing"

	"github.com/Kaese72/huemie-lib/query"
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
