package main

import (
	"context"
	"net/http"
	"os"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/internal/config"
	"github.com/Kaese72/ittt-orchestrator/internal/devicestore"
	"github.com/Kaese72/ittt-orchestrator/internal/events"
	"github.com/Kaese72/ittt-orchestrator/internal/orchestrator"
	"github.com/Kaese72/ittt-orchestrator/internal/persistence/mariadb"
	"github.com/Kaese72/ittt-orchestrator/internal/restwebapp"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humamux"
	"github.com/gorilla/mux"
)

func main() {
	if err := config.Loaded.Validate(); err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}

	db, err := mariadb.NewMariadbPersistence(config.Loaded.Database)
	if err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}

	dsClient := devicestore.NewClient(config.Loaded.DeviceStore.URL)

	orch := orchestrator.New(db, dsClient)

	if err := events.StartConsumer(context.Background(), config.Loaded.Event, orch); err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}

	webapp := restwebapp.NewWebApp(db)

	router := mux.NewRouter()
	humaConfig := huma.DefaultConfig("ittt-orchestrator", "1.0.0")
	humaConfig.OpenAPIPath = "/ittt-orchestrator/openapi"
	humaConfig.DocsPath = "/ittt-orchestrator/docs"
	api := humamux.New(router, humaConfig)

	huma.Get(api, "/ittt-orchestrator/v0/status", webapp.GetStatus)
	huma.Get(api, "/ittt-orchestrator/v0/rules", webapp.GetRules)
	huma.Get(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}", webapp.GetRule)
	huma.Post(api, "/ittt-orchestrator/v0/rules", webapp.CreateRule)
	huma.Put(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}", webapp.UpdateRule)
	huma.Delete(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}", webapp.DeleteRule)

	huma.Get(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions", webapp.GetActions)
	huma.Post(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions", webapp.CreateAction)
	huma.Get(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions/{actionID:[0-9]+}", webapp.GetAction)
	huma.Put(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions/{actionID:[0-9]+}", webapp.UpdateAction)
	huma.Delete(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions/{actionID:[0-9]+}", webapp.DeleteAction)

	log.Info("Starting ittt-orchestrator on :8080", map[string]interface{}{})
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}
}
