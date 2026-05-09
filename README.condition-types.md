# Condition types

- [Condition types](#condition-types)
  - ["time-range"](#time-range)
  - ["time-range-days"](#time-range-days)
  - ["device-id-attribute-boolean-eq"](#device-id-attribute-boolean-eq)
  - [\[NOT IMPLEMENTED\] "day-of-week"](#not-implemented-day-of-week)
    - [Incompatability with time based conditions](#incompatability-with-time-based-conditions)

This document describes the different condition types the system supports.
One section for each condition type

## "time-range"

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

## "time-range-days"

An extension of `time-range` that also restricts which days of the week the range is active on.

All fields from `time-range` apply (`from`, `to`, `timezone`). The additional field `days` is an array of lowercase day names: `"monday"`, `"tuesday"`, `"wednesday"`, `"thursday"`, `"friday"`, `"saturday"`, `"sunday"`. Between 1 and 6 days must be specified — at least one is required, and all seven may not be set (if all days should match, use `time-range` instead). Order does not matter, and duplicates are ignored.

```json
{
    "type": "time-range-days",
    "from": "22:00:00",
    "to": "02:00:00",
    "timezone": "Europe/Stockholm",
    "days": ["friday", "saturday"]
}
```

The active window for a given day is anchored to the day on which `from` falls, not on which `to` falls. For a midnight-crossing range (e.g. `from: 22:00`, `to: 02:00`) with `days: ["friday"]`, the active window runs from Friday 22:00 to Saturday 02:00. The Saturday portion is included because the window *started* on Friday.

When evaluated, this emits a *next occurence* at the next state-change boundary:

* If currently **inside** an active window (condition is `true`): the `to` boundary of the current window — i.e. when the condition next becomes `false`.
* If currently **outside** any active window (condition is `false`): the `from` boundary of the next active window — i.e. the next occurrence of `from` on a matching day.

## "device-id-attribute-boolean-eq"

True when the device identified by the id stored in `id` has an attribute matching 
the name stored in `attribute` which has a boolean state that matches what is stored
in `boolean`.

If any of these are not true, the `condition` evaluates to `false`. For example, when
* The device identified by `id` does not exist
* The device identified by `id` does not have the "active" attribute
* The named attribute does not have a boolean state (null)

This type never emits any *next occurence* when used in a `condition tree`.

## [NOT IMPLEMENTED] "day-of-week"

True when the current day of the week (in the given timezone) is one of the days listed in `days`.

`days` is an array of lowercase day names: `"monday"`, `"tuesday"`, `"wednesday"`, `"thursday"`, `"friday"`, `"saturday"`, `"sunday"`. Between 1 and 6 days must be specified — at least one day is required, and all seven may not be set (if all days should match, omit the condition from the rule entirely). Order does not matter, and duplicates are ignored.

`timezone` is a required IANA timezone identifier (e.g. `"Europe/Stockholm"`). It determines which calendar day "now" falls on, making the condition sensitive to local midnight boundaries rather than UTC.

```json
{
    "type": "day-of-week",
    "days": ["monday", "tuesday", "wednesday", "thursday", "friday"],
    "timezone": "Europe/Stockholm"
}
```

When evaluated, this emits a *next occurence* at the next state-change boundary:

* If today is **not** a matching day (condition is `false`): midnight at the start of the next matching day — i.e. when the condition next becomes `true`.
* If today **is** a matching day (condition is `true`): midnight at the end of the current consecutive run of matching days — i.e. when the condition next becomes `false`.

For the second case, consecutive matching days are treated as a single block. For example, with `days` set to `["monday", "tuesday", "wednesday"]` and "now" being Monday afternoon, the *next occurence* is midnight between Wednesday and Thursday — not midnight between Monday and Tuesday — because the condition remains `true` throughout the entire Monday–Wednesday block.

"Midnight" here means 00:00:00 in the configured timezone, converted to an absolute timestamp.

### Incompatability with time based conditions

When using time based conditions like `time-range` adjacent to this, information
may be lost around midninght. For example, when having a `time-range` from `22:00` to `02:00`
and a `day-of-week` set to Monday, the rule will evaluate to `true` on two unexpected times

* Monday `00:00` - `02:00`
* Monday `22:00` - `00:00`

However, it will not be `true` on Tuesday `00:00` - `02:00` which might be expected.

