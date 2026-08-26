# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run (requires env vars for whichever services your rules reference)
FRIGATE_BASE_URL=<url> YOLINK_CLIENT_ID=<id> YOLINK_CLIENT_SECRET=<secret> go run ./cmd/homelink

# Test
go test ./...

# Build Docker images (homelink + kasa-agent sidecar)
docker compose build

# Run via Docker Compose (reads from .env)
docker compose up
```

## Architecture

**homelink** is a Go service that routes trigger events from smart-home services to actions
on other (or the same) services, via a generic `[[rules]]` config — a device event on service
A does something to a device on service B. It is deployed as a Docker container on TrueNAS
Scale.

### Flow

1. **Config load** (`internal/config`) parses `homelink.toml` down to the generic rule shape —
   name + trigger + actions — leaving each trigger/action's service-specific fields undecoded
   as a `toml.Primitive`.
2. **Rule building** (`cmd/homelink/main.go: buildRules`) reads the `service` field off each
   trigger/action primitive and dispatches to that service package's own `DecodeTrigger` /
   `DecodeAction` function, producing typed, validated configs. This is the only place that
   imports every service package — each service package is otherwise independent of the others.
3. **Selective connection**: only services actually referenced by a rule are ever connected to.
   A service with no trigger in any rule never subscribes or polls; a service with no action in
   any rule never gets a client constructed.
4. **Engine** (`internal/engine`) starts one `Source` per referenced trigger service, matches
   incoming `Event{Service, Device, Name}` values against rules, and calls the corresponding
   `Actuator.Execute` for every configured action. It knows nothing about YoLink, Frigate, or
   Kasa specifically — only the `Source`/`Actuator` interfaces.
5. **HTTP server** exposes `GET /healthz`, checking whichever of YoLink MQTT / Frigate / the
   Kasa agent are actually in use.

### Package layout

```
cmd/homelink/       — main binary; decodes rules, wires services, runs the engine + HTTP server
internal/config/    — generic (service-agnostic) TOML parsing: rules, server settings
internal/engine/    — Event, Source, Actuator interfaces; rule matching and dispatch
internal/yolink/    — YoLink Source: settings, trigger decode + event vocabulary, MQTT subscription
internal/frigate/   — Frigate Actuator: settings, action decode, HTTP client (login, create_event)
internal/kasa/      — Kasa Actuator: settings, action decode, HTTP client to the kasa-agent
                       sidecar. Owns no device addressing at all — see below.
internal/timer/     — Timer Source *and* Actuator: named countdowns (set/cancel actions, expired
                       trigger) and fixed daily schedules (time_of_day trigger) — see below
kasa-agent/          — Python FastAPI sidecar wrapping python-kasa (see "Why a Python sidecar" below)
```

### The timer service is both a Source and an Actuator

Every other service is only ever a trigger source or only ever an actuator. Timer is both,
because its trigger ("a named timer expired") and its actions ("set"/"cancel" a named timer)
share state — a `set` action has to reach the same countdown a rule elsewhere is watching for
expiry. `internal/timer.Service` implements `engine.Source` and `engine.Actuator` on one
instance; `main.go` constructs it once (if timer is used as either) and registers it into
`sources["timer"]` and/or `actuators["timer"]` depending on which roles are actually referenced.

Two consequences that came out of adding this second kind of source:

- `engine.Source`'s channel no longer has to be closed on shutdown — `timer.Service`'s event
  channel is long-lived and fed from arbitrary goroutines (`time.AfterFunc` callbacks) that
  `Stop()`'s context cancellation doesn't reach, so closing it would risk a
  send-on-a-closed-channel panic. The engine's per-source consumer loop now also selects on
  `ctx.Done()`, so it exits cleanly whether or not a source ever closes its channel.
- `engine.Rule` gained a `TriggerConfig any` field holding the trigger service's fully-decoded
  config (not just `Device`/`Event`), because `timer.TriggerConfig` carries an extra `At` field
  for `time_of_day` schedules that the generic `Device`/`Event` strings can't represent.
  `main.go` reads it back via type assertion when building the schedule list passed to
  `timer.NewService`.

`time_of_day` schedules are deduplicated inside `timer.NewService` by `(Device, At)`, so multiple
rules sharing one schedule label fire once each rather than once-per-rule-per-firing.

### Named countdowns are persisted; schedules are not

`set`/`cancel`-armed countdowns (but not `time_of_day` schedules) survive a process restart via
`internal/timer/persist.go`: `Service` tracks each armed timer's absolute expiry instant
alongside its `*time.Timer` handle (the latter exposes no way to ask "when does this fire," so
the expiry has to be tracked separately), and rewrites a small JSON file at `TIMER_PERSIST_PATH`
on every `Set`/`Cancel`/expiry. On startup, `Service.resume` reads that file: a countdown still
in the future is re-armed for its remaining duration; one whose expiry already passed while the
process was down fires immediately rather than being dropped — every timer in this codebase
represents a bounded "do X after some inactivity" window (e.g. `garage_lights_off`), so a late
fire is judged safer than silently losing the eventual action. `time_of_day` schedules need none
of this: they're recomputed fresh from config on every `Start()` regardless of restarts, since
"next 4pm" doesn't depend on any prior process state.

### Why a Python sidecar for Kasa

Current-firmware TP-Link Kasa devices mostly require KLAP (TP-Link's newer local-protocol
encryption). No Go library found at the time this was built documented KLAP support, and
implementing that handshake from scratch in Go would mean unverified custom crypto code
controlling smart locks and plugs. `python-kasa` is the actively-maintained, protocol-complete
reference implementation and was verified against this deployment's actual device fleet, so
`internal/kasa` delegates to it over HTTP via `kasa-agent` rather than reimplementing the
protocol.

### Kasa device name resolution lives entirely in kasa-agent

`internal/kasa` carries no device directory at all — a rule's `device` field is sent to the
agent as-is (URL-escaped) and homelink's Go code never learns or stores an IP. `kasa-agent`
resolves names to hosts itself:

- **Startup + periodic refresh**: it scans `KASA_SCAN_SUBNET` (a plain TCP connect to any of
  `KASA_SCAN_PORTS`, default `9999,80`, to narrow the subnet down to hosts worth identifying —
  legacy devices only open 9999, but current-firmware KLAP devices like the HS300 and
  KS225/KS205 use 80 instead and never open 9999 at all), then runs `Discover.discover_single`
  against each candidate host and reads its self-reported `alias` (the name from the Kasa app)
  into an in-memory `name -> host` cache. A background task repeats this every
  `KASA_SCAN_INTERVAL_SECONDS` (default 300) to absorb DHCP churn or renames without a restart.
  `_identify()` always calls `device.disconnect()` in a `finally`, even on failure — some
  devices (a cloud-linked HS300, for instance) pass the unauthenticated discovery step but fail
  the following `update()`, and without an explicit disconnect that leaks an `aiohttp` session
  on every single scan cycle for as long as that device sits on the network.
- **Limited retry on a cache miss**: a lookup for a name not currently cached triggers up to
  `KASA_LOOKUP_MAX_RETRIES` (default 2) on-demand rescans, `KASA_LOOKUP_RETRY_DELAY_SECONDS`
  apart (default 2s), before returning 404 with the current list of known names — bounded, not
  infinite, so a genuinely wrong name fails promptly instead of hanging a rule's action forever.
- **Single-flight scanning**: concurrent triggers for a rescan (several misses at once, or a
  miss racing the periodic refresh) share one in-flight scan rather than each starting a
  redundant subnet sweep — `DeviceCache.rescan()` reuses the pending `asyncio.Task` if one is
  already running.
- **`GET /devices`** on the agent returns the current cache verbatim, for debugging a name that
  won't resolve.
- **Optional cloud credentials**: most Kasa devices authenticate with no credentials at all (see
  `internal/kasa`'s Actuator doc comment / the "how do Kasa devices authenticate" background —
  KLAP's blank/default-credential fallback covers them). A device actually linked to a TP-Link
  cloud account needs the real account email/password, set via `KASA_USERNAME`/`KASA_PASSWORD`
  (both-or-neither, agent exits at startup otherwise). These are passed to *every* device, not
  just the linked one — safe to do, since KLAP's own fallback chain (given credentials → TP-Link's
  hardcoded setup defaults → blank) means an unlinked device that already worked with no
  credentials keeps working identically; verified by running the agent with intentionally wrong
  credentials and confirming the other 18 devices still resolved.

Because of the retry path's worst case (roughly two full scans plus the retry delay),
`internal/kasa.Actuator`'s HTTP client timeout is set well above a normal cache-hit response
(30s) — otherwise a real 404 from the agent's retry budget could be masked by a Go-side timeout
first.

### Configuration

Settings are loaded per-service, from a config file (optional, non-sensitive settings only)
overlaid by environment variables (credentials are env-var only, always win). The config file
path defaults to `/config/homelink.toml`, overridable with `CONFIG_FILE`.

```toml
[yolink]
api_host    = "https://api.yosmart.com"
mqtt_broker = "tcp://mqtt.api.yosmart.com:8003"

[frigate]
base_url = "http://frigate:5000"

[kasa]
agent_base_url = "http://kasa-agent:8090"
# No device list: `device` in a rule is the exact Kasa app name, resolved
# by kasa-agent itself (KASA_SCAN_SUBNET etc.) — see above.

[server]
port      = 8080
log_level = "warn"

[[rules]]
name = "mailbox opened"

[rules.trigger]
service = "yolink"
device  = "Mailbox Sensor"
event   = "door.opened"

[[rules.actions]]
service   = "frigate"
device    = "Street Camera"
action    = "create_event"
label     = "mail"
sub_label = "mailbox"

[[rules.actions]]
service = "kasa"
device  = "Porch Light"
action  = "turn_on"
```

**Environment variables:**

| Variable | Required when | Default | Description |
|---|---|---|---|
| `YOLINK_CLIENT_ID` | a rule uses `service = "yolink"` trigger | — | YoLink API client ID |
| `YOLINK_CLIENT_SECRET` | same | — | YoLink API client secret |
| `FRIGATE_BASE_URL` | a rule uses `service = "frigate"` action | — | Frigate base URL (or `[frigate] base_url`) |
| `FRIGATE_USER` | no | — | Frigate native auth username |
| `FRIGATE_PASSWORD` | no | — | Frigate native auth password |
| `FRIGATE_INSECURE_SKIP_VERIFY` | no | — | `true`/`1` to skip TLS cert verification |
| `KASA_AGENT_BASE_URL` | a rule uses `service = "kasa"` action | — | kasa-agent sidecar base URL (or `[kasa] agent_base_url`) |
| `KASA_SCAN_SUBNET` *(kasa-agent, not homelink)* | a rule uses `service = "kasa"` action | — | CIDR subnet the agent scans, e.g. `192.168.1.0/24` |
| `KASA_SCAN_PORTS` *(kasa-agent)* | no | `9999,80` | Comma-separated ports probed to find scan candidates (a host with any one open qualifies) |
| `KASA_SCAN_INTERVAL_SECONDS` *(kasa-agent)* | no | `300` | Background rescan interval |
| `KASA_LOOKUP_MAX_RETRIES` *(kasa-agent)* | no | `2` | On-demand rescans on a cache-miss lookup before returning 404 |
| `KASA_LOOKUP_RETRY_DELAY_SECONDS` *(kasa-agent)* | no | `2` | Delay between those rescans |
| `KASA_USERNAME` / `KASA_PASSWORD` *(kasa-agent)* | only for a cloud-linked device | — | TP-Link cloud account, both-or-neither |
| `YOLINK_API_HOST` | no | see file | Cloud API host override (or `[yolink.cloud] api_host`). Cannot combine with `YOLINK_LOCAL_HOST`/`[yolink.local]`. |
| `MQTT_BROKER` | no | see file | Cloud MQTT broker override (or `[yolink.cloud] mqtt_broker`). Cannot combine with `YOLINK_LOCAL_HOST`/`[yolink.local]`. |
| `YOLINK_LOCAL_HOST` | no | — | Connect to a Local Hub instead of the cloud (or `[yolink.local] host`). Alone with `YOLINK_NET_ID`, switches mode with no `[yolink.local]` in the file needed. |
| `YOLINK_NET_ID` | with `YOLINK_LOCAL_HOST` | — | Local Hub's "Net Id" from the YoLink app's hub details (or `[yolink.local] net_id`); not obtainable via the API. |
| `YOLINK_LOCAL_HTTP_PORT` / `YOLINK_LOCAL_MQTT_PORT` | no | `1080` / `18080` | Override the Local Hub's ports (or `[yolink.local] http_port`/`mqtt_port`). |
| `TZ` | a rule uses `time_of_day` | UTC | IANA zone name `at` times are interpreted in (zoneinfo is embedded in the binary, no volume mount needed) |
| `TIMER_PERSIST_PATH` | no | `/cache/timers.json` | Where armed `set` countdowns are persisted so they survive a restart (see `internal/timer/persist.go`); must be on a writable volume, unlike the read-only `/config` mount |
| `PORT` | no | `8080` | HTTP server port |
| `LOG_LEVEL` | no | `warn` | Log verbosity: debug, info, warn, error |
| `CONFIG_FILE` | no | `/config/homelink.toml` | Path to config file |

### Rules

Each `[[rules]]` entry binds one trigger to one or more actions. `[rules.trigger]` and each
`[[rules.actions]]` entry start with a `service` field identifying which service package
decodes the rest of that table's fields; unknown services/fields fail config load with a clear
error naming the rule.

**Trigger event vocabulary** (`service = "yolink"`):

| `event` | Fires when |
|---|---|
| `door.opened` / `door.closed` | `DoorSensor` state transitions |
| `leak.detected` / `leak.dry` | `LeakSensor` state transitions |
| `lock.unlocked` / `lock.locked` | `Lock` / `LockV2` state transitions |
| `motion.detected` / `motion.clear` | `MotionSensor` state transitions |

**Trigger event vocabulary** (`service = "timer"`):

| `event` | Fires when | Extra fields |
|---|---|---|
| `expired` | the named timer (`device`) counts down to zero, armed by a `set` action | — |
| `time_of_day` | local wall-clock time crosses `at` each day | `at` (`"HH:MM"`, 24-hour, required) |

**Action verbs:**

| Service | `action` | Fields |
|---|---|---|
| `frigate` | `create_event` | `label`, `sub_label`, `duration`, `bounding_box`, `pre_capture`, `include_recording` |
| `kasa` | `turn_on`, `turn_off`, `toggle`, `set_brightness`, `led_on`, `led_off` | `set_brightness` requires `brightness` (0-100) |
| `timer` | `set`, `cancel` | `set` requires `duration` (Go syntax, e.g. `"5m"`, `"1h30m"`); `cancel` on an unknown/already-fired timer is a no-op |

Adding a new source or action service means adding a new `internal/<service>` package with its
own `Settings`/`TriggerConfig` or `ActionConfig`/`Decode*` functions and a `Source` or
`Actuator` implementation, then wiring its `service` name into `buildRules` and the
construction block in `cmd/homelink/main.go`. No other service package needs to change.

## Style

- Prefer descriptive variable names over single- or two-letter abbreviations.
