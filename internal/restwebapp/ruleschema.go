package restwebapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/danielgtaylor/huma/v2"
)

// componentsRefPrefix is how huma's default registry refers to its schemas.
const componentsRefPrefix = "#/components/schemas/"

var rxComponentsRef = regexp.MustCompile(`"` + regexp.QuoteMeta(componentsRefPrefix) + `([^"]+)"`)

// BundleSchema returns schema name from registry as one self-contained JSON
// Schema (2020-12) document: the schema itself at the top level ("$ref"), and
// it and every schema it transitively references under "$defs", with every
// reference rewritten from "#/components/schemas/X" to "#/$defs/X".
//
// It reads the registry the API's OpenAPI spec is generated from, so the
// result is exactly what GET /ittt-orchestrator/openapi publishes for those
// schemas, with nothing to maintain separately. Call it after every route is
// registered: huma adds a type to the registry when a route using it is
// registered, and transformers (the $schema link) only once its responses are.
func BundleSchema(registry huma.Registry, name string) (json.RawMessage, error) {
	all := registry.Map()
	defs := map[string]json.RawMessage{}
	pending := []string{name}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if _, done := defs[current]; done {
			continue
		}
		schema, ok := all[current]
		if !ok {
			return nil, fmt.Errorf("schema %q is not in the registry", current)
		}
		body, err := json.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("encoding schema %q: %w", current, err)
		}
		for _, match := range rxComponentsRef.FindAllSubmatch(body, -1) {
			pending = append(pending, string(match[1]))
		}
		defs[current] = rxComponentsRef.ReplaceAll(body, []byte(`"#/$$defs/$1"`))
	}

	names := make([]string, 0, len(defs))
	for n := range defs {
		names = append(names, n)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `{"$schema":"https://json-schema.org/draft/2020-12/schema","$ref":%q,"$defs":{`, "#/$defs/"+name)
	for i, n := range names {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, _ := json.Marshal(n)
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(defs[n])
	}
	buf.WriteString("}}")
	return buf.Bytes(), nil
}

// RuleSchemaHandler serves a pre-built rule schema bundle (see BundleSchema
// and RegisterRoutes). Anyone who may view rules may read their format.
func RuleSchemaHandler(schema json.RawMessage) func(context.Context, *struct{}) (*struct{ Body json.RawMessage }, error) {
	return func(ctx context.Context, _ *struct{}) (*struct{ Body json.RawMessage }, error) {
		if err := requireRulesView(ctx); err != nil {
			return nil, err
		}
		return &struct{ Body json.RawMessage }{Body: schema}, nil
	}
}

// RegisterRoutes registers every api-mode route on api. The rule schema
// route is registered last, from a bundle built once all the others (and so
// every rule-related schema) are in the registry, so it costs nothing per
// request; a bundle that can't be built fails startup rather than a request.
func RegisterRoutes(api huma.API, w WebApp) error {
	huma.Get(api, "/ittt-orchestrator/v0/status", w.GetStatus)
	huma.Get(api, "/ittt-orchestrator/v0/globals/timezones", w.GetTimezones)
	huma.Get(api, "/ittt-orchestrator/v0/rules", w.GetRules)
	huma.Get(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}", w.GetRule)
	huma.Post(api, "/ittt-orchestrator/v0/rules", w.CreateRule)
	huma.Put(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}", w.UpdateRule)
	huma.Delete(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}", w.DeleteRule)

	huma.Get(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/evaluate", w.EvaluateRule)
	huma.Post(api, "/ittt-orchestrator/v0/evaluate", w.EvaluateConditionTreeDirect)

	ruleSchema, err := BundleSchema(api.OpenAPI().Components.Schemas, "Rule")
	if err != nil {
		return fmt.Errorf("building rule schema: %w", err)
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-rule-schema",
		Method:      "GET",
		Path:        "/ittt-orchestrator/v0/rules/schema",
		Summary:     "Get the rule JSON Schema",
		Description: "The JSON Schema of a rule as one self-contained document (Rule, the condition tree, every condition type and actions under $defs), including descriptions of how each part behaves. It is the same schema this API's OpenAPI spec publishes, for clients such as AI agents that need to write rules without reading the whole spec.",
	}, RuleSchemaHandler(ruleSchema))
	return nil
}
