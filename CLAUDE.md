# ittt-orchestrator

ITTT (If-This-Then-That) automation orchestrator. Evaluates condition trees and triggers device actions when conditions are met.

## Architecture

Two deployment modes, both share the same binary:

- **`api`** — REST API (port 8080). Handles CRUD for rules/actions, publishes rule events to RabbitMQ. No rule evaluation, no action triggering.
- **`rule-state`** — Evaluation and scheduling daemon. Consumes both device-update events and rule events from RabbitMQ. Evaluates condition trees, triggers actions, and owns all scheduling state. No API surface.

```
User / UI
    |
    | HTTP
    v
[ api mode ]
    |  CRUD rules/actions      |  rule upsert/delete events
    +---------> MariaDB <------+
    |                          |
    | RabbitMQ upsert/delete   |
    v                          |
[ rule-state mode ] <----------+
    ^
    | RabbitMQ device-update events
    |
[ device-store ]
    |
    | capability triggers (HTTP)
    v
[ device-store ]
```

## Mode responsibilities

The two modes have strict, non-overlapping responsibilities. Do not blur this boundary when adding features.

**`api` owns:**
- REST API — all CRUD for rules and actions
- Publishing rule events to RabbitMQ so `rule-state` stays in sync
- Never writes `next_occurrence` or `backoff_until` — those are `rule-state`'s exclusive domain

**`rule-state` owns:**
- All rule evaluation — both scheduled (timer-based) and reactive (device updates)
- All action triggering via device-store
- All scheduling state: `next_occurrence`, `backoff_until`, in-process timers
- Consumes both the device-update fanout and the rule-events fanout
- On receiving an upsert event, always evaluates the rule immediately (in addition to rescheduling based on the result), so a newly created or updated rule takes effect without delay

**Consequences of this split:**
- `rule-state` is the only process that calls device-store capability endpoints
- `next_occurrence` in DB has one meaning: the time rule-state last calculated for the next scheduled evaluation. It is written only by rule-state and read only by rule-state
- If a feature needs to evaluate conditions or trigger actions, it belongs in `rule-state`
- If a feature needs to expose or mutate rule/action data, it belongs in `api`

## Project Layout

```
main.go                  # Entry point; dispatches to api or rule-state command
internal/                # Internal logic
   persistence/          # Logic related to storing state in the database
restmodels/              # REST/JSON data models; condition tree logic lives here
eventmodels/             # RabbitMQ event schemas
migrations/              # Flyway SQL migrations
```

## Key Patterns

**Condition tree**: Discriminated union on `type` field. AND is expressed as nested children; OR is same-level siblings. Evaluation is recursive with short-circuit on false-AND.

**Condition types**:
See `README.md`

**Actions**: Each rule has N actions. An action targets either a device or a group, specifies a capability, and passes args as a JSON blob.

**Scheduling**: `rule-state` holds a lock-protected map of `ruleID → timer`. On evaluation, it recalculates and resets the timer. Past-due timestamps are handled gracefully (fire immediately).

## Configuration (Environment Variables)

| Variable | Default | Required |
|---|---|---|
| `DATABASE_HOST` | — | yes |
| `DATABASE_PORT` | `3306` | no |
| `DATABASE_USER` | — | yes |
| `DATABASE_PASSWORD` | — | yes |
| `DATABASE_DATABASE` | `itttorchestrator` | no |
| `EVENT_CONNECTIONSTRING` | — | yes |
| `EVENT_DEVICE_UPDATES` | `deviceUpdates` | no |
| `DEVICE_STORE_URL` | `http://device-store:8080` | no |

Viper maps dots/hyphens to underscores. Nested config keys use `_` as separator.

## Development

```bash
go test ./...
go build -o ittt-orchestrator .
./ittt-orchestrator api          # start REST API
./ittt-orchestrator rule-state   # start scheduler
```

API docs available at `/ittt-orchestrator/docs` (Swagger UI) and `/ittt-orchestrator/openapi` (raw spec) when running locally.

## Database Migrations

Managed by Flyway via `Dockerfile.migrater`. Run the migrater container before deploying a new API version. Migrations live in `migrations/` as `VNNN__description.sql`.

When things are changing in the API that should affect some kind of persistence, database migrations
for MariaDB needs to be created

## Documentation

* Documentation in README.md (and therein linked markdown files) needs to be kept up to date with next functionality

## Quality Assurance

* Each condition type should have its own unit test file where complexity based on the condition is tested
  * Triggers during different circumstances
  * Does not trigger during other circumstances
  * What "next occurence" it should get
