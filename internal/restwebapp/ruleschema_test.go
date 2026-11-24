package restwebapp

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Kaese72/authentication/usertoken"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humamux"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/gorilla/mux"
)

// newTestAPI registers the real api-mode routes, configured like main.go, on
// a test API whose requests carry the given permissions.
func newTestAPI(t *testing.T, permissions *usertoken.Permissions) humatest.TestAPI {
	t.Helper()
	config := huma.DefaultConfig("ittt-orchestrator", "1.0.0")
	config.OpenAPIPath = "/ittt-orchestrator/openapi"
	config.DocsPath = "/ittt-orchestrator/docs"
	// Same router as main.go: its regex path params keep /rules/schema from
	// matching /rules/{ruleID}.
	api := humatest.Wrap(t, humamux.New(mux.NewRouter(), config))
	if permissions != nil {
		api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
			next(huma.WithContext(ctx, usertoken.ContextWithAuth(ctx.Context(), 1, *permissions)))
		})
	}
	if err := RegisterRoutes(api, NewWebApp(nil, nil, nil)); err != nil {
		t.Fatal(err)
	}
	return api
}

// TestRuleSchemaMatchesOpenAPI checks the rule schema endpoint serves exactly
// the schemas the OpenAPI spec publishes (references rewritten to $defs),
// complete enough to describe a whole rule.
func TestRuleSchemaMatchesOpenAPI(t *testing.T) {
	view := usertoken.NewPermissions(map[string]usertoken.ResourceGrant{
		usertoken.ResourceAutomationRules: {View: "*"},
	})
	api := newTestAPI(t, &view)

	resp := api.Get("/ittt-orchestrator/v0/rules/schema")
	if resp.Code != http.StatusOK {
		t.Fatalf("rule schema: got %d: %s", resp.Code, resp.Body.String())
	}
	var bundle struct {
		Schema string                     `json:"$schema"`
		Ref    string                     `json:"$ref"`
		Defs   map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Ref != "#/$defs/Rule" {
		t.Fatalf("top-level $ref = %q, want #/$defs/Rule", bundle.Ref)
	}
	if strings.Contains(resp.Body.String(), "#/components/schemas/") {
		t.Fatal("bundle still contains #/components/schemas/ references")
	}
	for _, want := range []string{"Rule", "Action", "ConditionTree", "TimeRangeCondition", "DeviceAttributeTextSubstringCondition"} {
		if _, ok := bundle.Defs[want]; !ok {
			t.Errorf("bundle is missing %s", want)
		}
	}

	specResp := api.Get("/ittt-orchestrator/openapi.json")
	if specResp.Code != http.StatusOK {
		t.Fatalf("openapi: got %d", specResp.Code)
	}
	var spec struct {
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(specResp.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	for name, got := range bundle.Defs {
		published, ok := spec.Components.Schemas[name]
		if !ok {
			t.Errorf("%s is in the bundle but not in the OpenAPI spec", name)
			continue
		}
		want := rxComponentsRef.ReplaceAll(published, []byte(`"#/$$defs/$1"`))
		var gotValue, wantValue any
		json.Unmarshal(got, &gotValue)
		json.Unmarshal(want, &wantValue)
		if !reflect.DeepEqual(gotValue, wantValue) {
			t.Errorf("%s differs from the OpenAPI spec:\nbundle:  %s\nopenapi: %s", name, got, want)
		}
	}
}

func TestRuleSchemaRequiresView(t *testing.T) {
	none := usertoken.NewPermissions(nil)
	api := newTestAPI(t, &none)
	if resp := api.Get("/ittt-orchestrator/v0/rules/schema"); resp.Code != http.StatusForbidden {
		t.Fatalf("without view access: got %d, want 403", resp.Code)
	}
}

func TestBundleSchemaUnknownName(t *testing.T) {
	registry := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	if _, err := BundleSchema(registry, "Nope"); err == nil {
		t.Fatal("want an error for a schema that isn't registered")
	}
}
