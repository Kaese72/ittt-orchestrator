# (I)f (T)his (T)hen (T)that orchestrator

This orchestrator keeps track of rules aimed at automating some workflow.
A rule consists of `conditions`, organized into a `condition tree` and `actions`.

`Conditions` are effectively logical expressions consisting of nodes that can be true or false.

`Actions` are simply capability triggers of either groups or devices that triggers when the
conditions are true.

A `rule` can be put on cooldown, meaning it is scheduled to trigger once the cooldown
expires. This functionality can primarily be trigged through the `conditions`, where 
a change to a device state we rely on will put the rule on cooldown for a while.

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

#### "device-id-attribute-boolean-eq"

True when the device identified by the id stored in `id` has an attribute matching 
the name stored in `attribute` which has a boolean state that matches what is stored
in `boolean`.

If any of these are not true, the `condition` evaluates to `false`. For example, when
* The device identified by `id` does not exist
* The device identified by `id` does not have the "active" attribute
* The named attribute does not have a boolean state (null)

## Cooldown

A rule can be put on "cooldown", meaning it gets scheduled to trigger after some time
and it does not matter how many times it would otherwise trigger.

A typical example of this functionality would be to allow an "interrupt"
of a schedule like rule. For example, if a rule says that lights should be
off after 20.00, we may put a cooldown on the device state check such that if 
the lights are turned on at 21.00, the rule may be put on cooldown for 30 minutes,
which means that the lights will remain on for the duration of the cooldown, and then
the rule will be evaluated after 30 minutes, evaluate to true, and turn off the lights.

The cooldown is set on the entire `rule`, and each things that interacts with the
cooldown should document how that interraction works. 

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