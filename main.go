package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	_ "time/tzdata"

	log "github.com/Kaese72/huemie-lib/logging"
	"github.com/Kaese72/ittt-orchestrator/internal/config"
	"github.com/Kaese72/ittt-orchestrator/internal/devicestore"
	"github.com/Kaese72/ittt-orchestrator/internal/events"
	"github.com/Kaese72/ittt-orchestrator/internal/orchestrator"
	"github.com/Kaese72/ittt-orchestrator/internal/persistence/mariadb"
	"github.com/Kaese72/ittt-orchestrator/internal/restwebapp"
	"github.com/Kaese72/ittt-orchestrator/internal/scheduler"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humamux"
	"github.com/gorilla/mux"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: ittt-orchestrator <command>\n\ncommands:\n  api         Run the REST API server\n  rule-state  Run the rule scheduling state manager\n")
		os.Exit(1)
	}

	if err := config.Loaded.Validate(); err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}

	switch os.Args[1] {
	case "api":
		runAPI()
	case "rule-state":
		runRuleState()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// runAPI starts the REST API server together with the device-update consumer
// and the rule-event publisher. It does not manage any scheduling state.
func runAPI() {
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

	publisher, err := events.NewRuleEventPublisher(config.Loaded.Event.ConnectionString)
	if err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}
	defer publisher.Close()

	webapp := restwebapp.NewWebApp(db, orch, publisher)

	router := mux.NewRouter()
	humaConfig := huma.DefaultConfig("ittt-orchestrator", "1.0.0")
	humaConfig.OpenAPIPath = "/ittt-orchestrator/openapi"
	humaConfig.DocsPath = "/ittt-orchestrator/docs"
	api := humamux.New(router, humaConfig)

	huma.Get(api, "/ittt-orchestrator/v0/status", webapp.GetStatus)
	huma.Get(api, "/ittt-orchestrator/v0/globals/timezones", webapp.GetTimezones)
	huma.Get(api, "/ittt-orchestrator/v0/rules", webapp.GetRules)
	huma.Get(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}", webapp.GetRule)
	huma.Post(api, "/ittt-orchestrator/v0/rules", webapp.CreateRule)
	huma.Put(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}", webapp.UpdateRule)
	huma.Delete(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}", webapp.DeleteRule)

	huma.Get(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/evaluate", webapp.EvaluateRule)
	huma.Get(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions", webapp.GetActions)
	huma.Post(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions", webapp.CreateAction)
	huma.Get(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions/{actionID:[0-9]+}", webapp.GetAction)
	huma.Put(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions/{actionID:[0-9]+}", webapp.UpdateAction)
	huma.Delete(api, "/ittt-orchestrator/v0/rules/{ruleID:[0-9]+}/actions/{actionID:[0-9]+}", webapp.DeleteAction)

	log.Info("Starting ittt-orchestrator api on :8080", map[string]interface{}{})
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}
}

// runRuleState starts the rule scheduling state manager. It consumes rule
// events from RabbitMQ and maintains per-rule timers that trigger evaluation
// and action dispatch at each rule's next occurrence.
func runRuleState() {
	db, err := mariadb.NewMariadbPersistence(config.Loaded.Database)
	if err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}

	dsClient := devicestore.NewClient(config.Loaded.DeviceStore.URL)
	orch := orchestrator.New(db, dsClient)
	sched := scheduler.New(db, orch)

	if err := events.StartRuleConsumer(context.Background(), config.Loaded.Event, sched); err != nil {
		log.Error(err.Error(), map[string]interface{}{})
		os.Exit(1)
	}

	sched.Start()

	log.Info("Starting ittt-orchestrator rule-state manager", map[string]interface{}{})
	// Block forever; all work happens in goroutines driven by RabbitMQ and timers.
	select {}
}
