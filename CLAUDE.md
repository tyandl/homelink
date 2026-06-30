# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run (requires env vars)
YOLINK_CLIENT_ID=<id> YOLINK_CLIENT_SECRET=<secret> FRIGATE_BASE_URL=<url> go run ./cmd/homelink

# Test
go test ./...

# Build Docker image
docker build -t homelink .

# Run via Docker Compose (reads from .env)
docker compose up
```

## Architecture

**homelink** is a Go service that bridges YoLink smart-home devices to Frigate NVR.
It is deployed as a Docker container on TrueNAS Scale.

### Flow

1. **YoLink** — connects via HTTP (auth/device list) and MQTT (real-time device reports).
   Uses `github.com/tyandl/yolink-api` as the client library.
2. **Event routing** — when a watched device fires an alert, the watcher creates a manual
   event on the configured Frigate camera.
3. **Frigate** — receives manual event creation requests via its HTTP API. Supports
   optional native auth (FRIGATE_USER / FRIGATE_PASSWORD).
4. **HTTP server** — exposes `GET /healthz` for container health checks.

### Package layout

```
cmd/homelink/       — main binary; wires config, watcher, HTTP server
internal/config/    — config file + env var loading and validation
internal/frigate/   — Frigate HTTP client (login, event creation)
internal/watcher/   — subscribes to mapped YoLink devices, routes alerts to Frigate
```

### Configuration

Settings are loaded in priority order: **defaults → config file → environment variables**.
Environment variables always win.

**Config file** — mount `config/homelink.toml` (or any directory) at `/config` in the
container. The file path can be overridden with the `CONFIG_FILE` env var.

```toml
[yolink]
api_host    = "https://api.yosmart.com"
mqtt_broker = "tcp://mqtt.api.yosmart.com:8003"

[frigate]
base_url = "http://frigate:5000"

[server]
port      = 8080
log_level = "warn"

[[mappings]]
yolink_device = "Mailbox Sensor"
camera        = "Street Camera"
label         = "mail"
sub_label     = "mailbox"
duration      = 30
# bounding_box = [0.0, 0.0, 1.0, 1.0]
```

**Environment variables** (credentials must be set here):

| Variable | Required | Default | Description |
|---|---|---|---|
| `YOLINK_CLIENT_ID` | yes | — | YoLink API client ID |
| `YOLINK_CLIENT_SECRET` | yes | — | YoLink API client secret |
| `FRIGATE_BASE_URL` | yes* | — | Frigate base URL (*or set via config file) |
| `FRIGATE_USER` | no | — | Frigate native auth username |
| `FRIGATE_PASSWORD` | no | — | Frigate native auth password |
| `YOLINK_API_HOST` | no | see file | Override YoLink API host |
| `MQTT_BROKER` | no | see file | Override MQTT broker URL |
| `PORT` | no | `8080` | HTTP server port |
| `LOG_LEVEL` | no | `warn` | Log verbosity: debug, info, warn, error |
| `CONFIG_FILE` | no | `/config/homelink.toml` | Path to config file |

### Device mappings

Each `[[mappings]]` entry in the config file subscribes to a YoLink device and creates
a Frigate manual event when it fires an alert. Supported device types and their triggers:

| Type | Triggers when |
|---|---|
| `DoorSensor` | state → `open` |
| `LeakSensor` | state → `alert` |
| `Lock` | state → `unlocked` |
| `LockV2` | state → `unlocked` |

## Style

- Prefer descriptive variable names over single- or two-letter abbreviations.
