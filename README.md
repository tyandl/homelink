# homelink

A Go service that routes trigger events from one smart-home service to actions on another (or
the same) service, via a generic rules config. Originally built to bridge
[YoLink](https://www.yosmart.com/) device alerts to [Frigate](https://frigate.video/) NVR
events (replacing an IFTTT integration TP-Link/YoLink dropped support for); the same rules
schema now also drives [TP-Link Kasa](https://www.kasasmart.com/) smart plugs and switches, plus
a built-in timer service for delays and fixed daily schedules.

## How it works

1. Each `[[rules]]` entry in the config binds one **trigger** (a device event on some service)
   to one or more **actions** (something to do on some service). A single trigger can fan out
   to actions on several services at once.
2. Only services actually referenced by a rule are ever connected to — a service with no
   configured trigger never subscribes or polls, and a service with no configured action never
   gets a client built.
3. Supported today:
   - **Trigger**: `yolink` — door sensors, leak sensors, and locks, via YoLink's MQTT reports.
   - **Trigger + action**: `timer` — set/cancel a named countdown and trigger on its expiry, or
     trigger on a fixed daily time (`at = "16:00"`).
   - **Actions**: `frigate` — opens a manual event on a camera; `kasa` — turns a plug/switch/bulb
     on or off, toggles it, sets brightness, or controls its status LED.

See `CLAUDE.md` for the package layout and how to add a new trigger or action service.

## Configuration

Settings are loaded per-service from an optional config file, overlaid by environment
variables — credentials are environment-variable only and always win.

### Config file

Mount a TOML file at `/config/homelink.toml` (or set `CONFIG_FILE` to override the path).

```toml
[yolink]
api_host    = "https://api.yosmart.com"
mqtt_broker = "tcp://mqtt.api.yosmart.com:8003"

[frigate]
base_url = "http://frigate:5000"

[kasa]
agent_base_url = "http://kasa-agent:8090"
# No device list: `device` below is the exact name from the Kasa app.
# kasa-agent resolves it to an IP itself (see "Kasa devices" below).

[server]
port      = 8080
log_level = "warn"   # debug | info | warn | error

[[rules]]
name = "mailbox opened"

[rules.trigger]
service = "yolink"
device  = "Mailbox Sensor"   # exact name from the YoLink app
event   = "door.opened"

[[rules.actions]]
service   = "frigate"
device    = "Street Camera"  # Frigate camera name
action    = "create_event"
label     = "mail"
sub_label = "mailbox"

[[rules.actions]]
service = "kasa"
device  = "Porch Light"      # exact name from the Kasa app
action  = "turn_on"

[[rules]]
name = "porch light at 4pm"

[rules.trigger]
service = "timer"
device  = "afternoon"        # arbitrary schedule label
event   = "time_of_day"
at      = "16:00"            # local time — set TZ

[[rules.actions]]
service = "kasa"
device  = "Porch Light"
action  = "turn_on"
```

Full trigger event vocabulary and action verb reference is in `CLAUDE.md`.

### Environment variables

| Variable | Required when | Default | Description |
|---|---|---|---|
| `YOLINK_CLIENT_ID` | a rule triggers on `yolink` | — | YoLink API client ID (UAID) |
| `YOLINK_CLIENT_SECRET` | a rule triggers on `yolink` | — | YoLink API secret key |
| `FRIGATE_BASE_URL` | a rule acts on `frigate` | — | Frigate base URL (or `[frigate] base_url`) |
| `FRIGATE_USER` | no | — | Frigate native auth username |
| `FRIGATE_PASSWORD` | no | — | Frigate native auth password |
| `FRIGATE_INSECURE_SKIP_VERIFY` | no | — | `true` to skip TLS cert verification (self-signed certs) |
| `KASA_AGENT_BASE_URL` | a rule acts on `kasa` | — | kasa-agent sidecar base URL (or `[kasa] agent_base_url`) |
| `KASA_SCAN_SUBNET` *(kasa-agent)* | a rule acts on `kasa` | — | CIDR subnet the agent scans to resolve device names, e.g. `192.168.1.0/24` |
| `KASA_SCAN_INTERVAL_SECONDS` *(kasa-agent)* | no | `300` | Background rescan interval |
| `KASA_LOOKUP_MAX_RETRIES` *(kasa-agent)* | no | `2` | On-demand rescans on a cache-miss lookup before failing |
| `KASA_LOOKUP_RETRY_DELAY_SECONDS` *(kasa-agent)* | no | `2` | Delay between those rescans |
| `KASA_USERNAME` / `KASA_PASSWORD` *(kasa-agent)* | only for a device linked to a TP-Link cloud account | — | Both-or-neither; the agent exits at startup if only one is set |
| `YOLINK_API_HOST` | no | `https://api.yosmart.com` | Cloud API host override (or `[yolink.cloud] api_host`). Cannot combine with `YOLINK_LOCAL_HOST`/`[yolink.local]`. |
| `MQTT_BROKER` | no | `tcp://mqtt.api.yosmart.com:8003` | Cloud MQTT broker override (or `[yolink.cloud] mqtt_broker`). Cannot combine with `YOLINK_LOCAL_HOST`/`[yolink.local]`. |
| `YOLINK_LOCAL_HOST` | no | — | Connect to a Local Hub instead of the cloud (or `[yolink.local] host`). Setting this alone (with `YOLINK_NET_ID`) is enough to switch modes, even with no `[yolink.local]` in the file. |
| `YOLINK_NET_ID` | with `YOLINK_LOCAL_HOST` | — | The Local Hub's "Net Id" from the YoLink app's hub details (or `[yolink.local] net_id`); not obtainable via the API. |
| `YOLINK_LOCAL_HTTP_PORT` / `YOLINK_LOCAL_MQTT_PORT` | no | `1080` / `18080` | Override the Local Hub's ports (or `[yolink.local] http_port`/`mqtt_port`). |
| `TZ` | a rule uses `time_of_day` | UTC | IANA zone name `at` times are interpreted in |
| `TIMER_PERSIST_PATH` | no | `/cache/timers.json` | Where armed `set` countdowns are persisted so they survive a restart; must be on a writable volume |
| `PORT` | no | `8080` | HTTP server port |
| `LOG_LEVEL` | no | `warn` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `CONFIG_FILE` | no | `/config/homelink.toml` | Path to config file |

### YoLink credentials

Create a **User Access Credential** in the YoLink mobile app:

> **Settings → Account → Advanced Settings → User Access Credentials → Create**

This yields a **UAID** (`YOLINK_CLIENT_ID`) and a **Secret Key** (`YOLINK_CLIENT_SECRET`).

### Kasa devices

homelink talks to Kasa devices through a small sidecar, `kasa-agent` (see `kasa-agent/`), which
wraps the [python-kasa](https://github.com/python-kasa/python-kasa) library — current-firmware
Kasa devices mostly require the KLAP protocol, and no mature Go implementation of it was
available.

Devices are addressed by name only — `device` in a rule is the exact name set in the Kasa app
(e.g. "Porch Light"), never an IP. `kasa-agent` resolves that name itself: it scans
`KASA_SCAN_SUBNET` at startup and periodically thereafter (`KASA_SCAN_INTERVAL_SECONDS`,
default 5 min), building a name → host cache from each device's own self-reported alias. A
lookup that misses the cache triggers up to `KASA_LOOKUP_MAX_RETRIES` on-demand rescans (default
2, `KASA_LOOKUP_RETRY_DELAY_SECONDS` apart, default 2s) before failing with a 404 that lists
every currently-known device name — so a typo'd name fails fast and clearly, while a
newly-added or just-renamed device is still found. `GET /devices` on the agent shows what it
currently believes, useful when a name won't resolve.

Most Kasa devices authenticate locally with no credentials at all. A device actually linked to
a TP-Link cloud account is the exception — set `KASA_USERNAME`/`KASA_PASSWORD` (the cloud
account email/password, both-or-neither) and the agent applies them to every device it talks
to. That's safe for the rest of your fleet: the protocol itself tries the given credentials
first, then TP-Link's own hardcoded setup defaults, then blank, so a device that already worked
with no credentials keeps working exactly the same.

If your Kasa devices sit on a different VLAN/subnet than the Docker host (a common setup for
IoT devices), make sure the `kasa-agent` container has a network path to it — bridge networking
is usually enough since outbound traffic is NATed through the host, but `network_mode: host` in
`docker-compose.yml` is the fallback if it isn't.

## Deployment

### Docker Compose

Create a `.env` file with credentials for whichever services your rules use:

```env
YOLINK_CLIENT_ID=your-uaid
YOLINK_CLIENT_SECRET=your-secret
FRIGATE_BASE_URL=http://frigate:5000
```

```bash
docker compose up
```

This builds and runs both `homelink` and the `kasa-agent` sidecar. Place your `homelink.toml`
at `./config/homelink.toml` alongside the compose file.

### Building from source

Requires Go 1.25+.

```bash
go build ./cmd/homelink

FRIGATE_BASE_URL=<url> YOLINK_CLIENT_ID=<id> YOLINK_CLIENT_SECRET=<secret> ./homelink
```

The `kasa-agent` sidecar requires Python 3.13+ and the packages in `kasa-agent/requirements.txt`
if you want to run it outside Docker.

## Health check

`GET /healthz` returns `200 OK` when every service actually in use (YoLink MQTT, Frigate, the
Kasa agent) is reachable. Use it for container health checks or uptime monitoring.

## License

Released under the [MIT License](LICENSE).
