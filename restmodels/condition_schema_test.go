package restmodels

import (
	"reflect"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

// TestConditionSchemasDescribed checks that every condition type in
// ConditionUnion's schema has a description and a "type" property pinned to
// its discriminator value (see condition_schema.go), and that every field of
// every rule-related schema is documented -- the schema is what AI agents
// learn the rule format from.
func TestConditionSchemasDescribed(t *testing.T) {
	registry := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	registry.Schema(reflect.TypeOf(Rule{}), true, "Rule")
	schemas := registry.Map()

	union := ConditionUnion{}.Schema(registry)
	if union.Discriminator == nil || len(union.Discriminator.Mapping) == 0 {
		t.Fatal("condition union has no discriminator mapping")
	}
	if len(union.Discriminator.Mapping) != len(union.OneOf) {
		t.Fatalf("discriminator maps %d types but oneOf lists %d", len(union.Discriminator.Mapping), len(union.OneOf))
	}
	for conditionType, ref := range union.Discriminator.Mapping {
		s := registry.SchemaFromRef(ref)
		if s == nil {
			t.Fatalf("%s: %s is not in the registry", conditionType, ref)
		}
		if s.Description == "" {
			t.Errorf("%s: no description; add a TransformSchema in condition_schema.go", conditionType)
		}
		typeProp := s.Properties["type"]
		if typeProp == nil || len(typeProp.Enum) != 1 || typeProp.Enum[0] != conditionType {
			t.Errorf("%s: type property is not pinned to %q; use describeCondition", conditionType, conditionType)
		}
	}

	for name, s := range schemas {
		if s.Description == "" {
			t.Errorf("schema %s has no description", name)
		}
		for field, prop := range s.Properties {
			// type is described by the schema's own description; name is
			// self-explanatory.
			if field == "type" || (name == "Rule" && field == "name") {
				continue
			}
			if prop.Description == "" {
				t.Errorf("%s.%s has no doc tag", name, field)
			}
		}
	}
	if !strings.Contains(schemas["ConditionTree"].Description, "\"or\"") {
		t.Error("ConditionTree description should explain and/or")
	}
}
