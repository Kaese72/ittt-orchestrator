# (I)f (T)his (T)hen (T)that orchestrator

This orchestrator keeps track of rules aimed at automating some workflow.
A rule consists of `conditions`, organized into a `condition tree` and `actions`.

`Conditions` are effectively logical expressions consisting of nodes that can be true or false.

`Actions` are simply capability triggers of either groups or devices that triggers when the
conditions are true.

A `rule` simply describes what the state should be for the rule to trigger.
The actual evaluation happens on a dynamic scheduled,
which is set based on events happening outside the orchestrator (like devices
changing state) or any time based nodes in the `condition tree`. See "Scheduling".

## Condition tree

The `condition tree` is a logical expression tree with `conditions` representing different
states of devices or similar entities. Each `condition` in the tree consists of a single
boolean check. Each `condition` can also *optionally* link to another `condition` with an 
AND relationship and/or a `condition` with an OR relationship. Can link to none or both
relationships.

When the expression tree is represented in text, indentation represents
AND relationships while no additional indentation represents OR relationships. 

For example, a simple rule for an outdoor light which should be turned on in the morning if any of its two
lights are off at the time.

* Time in range 06:00–10:00 UTC
  * device[id=1].active == false
  * device[id=2].active == false

In the API, a rule with this `condition tree` would then have the `"condition-tree"` attribute and look something like this

```json
{
    "condition-tree": {
        "condition": {
            "type": "time-range",
            "from": "06:00:00",
            "to": "10:00:00"
        },
        "and": {
            "condition": {
                "type": "device-id-attribute-boolean-eq",
                "id": 1,
                "attribute": "active",
                "boolean": false
            },
            "or": {
                "condition": {
                    "type": "device-id-attribute-boolean-eq",
                    "id": 2,
                    "attribute": "active",
                    "boolean": false
                }
            }
        }
    }
}
```

When interacting with the API, you can not edit each `condition` individually, but need to work on the entire `condition tree` at once. There are no API endpoints for individual `conditions`.

Each `condition` has a type leading to different functionality and looks for different things. Different `condition types` are documented below

### Condition types

Determined by the `type` attribute of a `condition`. The subsections describe what the different options lead to.

#### "time-range"

True when the current time (UTC) falls within the range defined by `from` (inclusive) and `to` (exclusive).

Both `from` and `to` are strings in `HH:MM:SS` format.

If `from` is earlier than `to` the range falls within a single day — for example `"from": "06:00:00", "to": "22:00:00"` is true between 06:00 and 22:00.

If `from` is later than `to` the range wraps around midnight — for example `"from": "22:00:00", "to": "06:00:00"` is true between 22:00 and 06:00 the following morning.

When evaluated, this emits a *next occurence* matching
* `from` if we are outside the range
  * Ie. the *node* evalautes to false
* `to` if we are inside the range
  * Ie. the *node* evaluates to *true*

The `from` and `to` may be the next day depending on when we are
within or outside the range we are.

#### "device-id-attribute-boolean-eq"

True when the device identified by the id stored in `id` has an attribute matching 
the name stored in `attribute` which has a boolean state that matches what is stored
in `boolean`.

If any of these are not true, the `condition` evaluates to `false`. For example, when
* The device identified by `id` does not exist
* The device identified by `id` does not have the "active" attribute
* The named attribute does not have a boolean state (null)

This type never emits any *next occurence* when used in a `condition tree`.

## Scheduling

A `rule` is expected to be triggered, and one of the triggers is through *scheduling* 
where the rule triggers based on its *next occurence*. The *next occurence* is determined
when the `rule` is evaluated and the *next occurence* is then set based on 
what nodes there are in the `condition tree`, or not set at all if there is nothing
that interacts with scheduling. 

When a `rule` is evaluated there may be multiple things interacting with the *scheduling*.
In such a case the *next occurence* should be set to the closes non-null value returned
from the evaluation.

The *next occurence* is a date and time, and not just a time.

If no *next occurence* is set, the `rule` will not be triggered on a schedule.

The *next occurence* is stored in the `rule` on the `next_occurence` attribute
and should be presented to the user in the UI.

When an update is made to a `rule`, the *next occurence* should be calculated
and stored, then an event should be omitted such that the scheduler can fetch
the value from the database and refresh the timer. 

When a `rule` is evaluated, either via schedule or from an event, 
the *next occurence* should be updated as well.

## Actions

Each `rule` when triggered will result in a set of `actions` triggering.
An `action` is simply a capability with arguments that will be used when the 
`rule` triggers. The capabilities can be for either `groups` or `devices`. 

In the API, for a `rule`, it looks something like

```json
{
    "actions": [
        {
            "type": "device-capability",
            "id": 1,
            "capability": "activate",
            "args": {}
        }
    ]
}
```

Through the API, `actions` should be individually mantainable, but an action
is always related to only a single `rule`. 

# Implementation details

## Service interactions

* Capabilities are triggered via the `device-store`
* When devices are updates, the updates are received via rabbitmq from the `device-store`
* Current state of devices are fetched from the `device-store` API and not stored locally.

## Database

### Database schema overview

* Rule -> condition
  * Links to the "root" of the `condition tree`
* Condition -> Rule
  * What `rule` does the `condition` belong to
* Condition -> Condition
  * Not a foreign key, only used for `condition tree` reconstruction.
* Action -> Rule
  * What `rule` does the action belong to

### Database migrations

The database authentication is expected to be setup ahead of time.
The username and password is supplied to the service for authentication towards
the database.

Migrations are handled by flyway in a separate container that is expected to run **before**
a new version of the service is setup. The new version of the database is expected
to be backwards compatible.