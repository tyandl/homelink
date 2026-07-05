# homelink

A Go service that bridges [YoLink](https://www.yosmart.com/) smart-home devices to
[Frigate](https://frigate.video/) NVR. When a watched device fires an alert (door opens,
leak detected, lock unlocked), homelink creates a manual event on the configured Frigate
camera.

## How it works

1. Connects to the YoLink API to enumerate devices and authenticate to the MQTT broker.
2. Subscribes to real-time reports for every device listed in `[[mappings]]`.
3. When a mapped device fires an alert, calls the Frigate `/api/events` endpoint to open
   a manual event on the associated camera.

Supported device types and their triggers:

| Device type | Triggers when |
|---|---|
| `DoorSensor` | state → `open` |
| `LeakSensor` | state → `alert` |
| `Lock` | state → `unlocked` |
| `LockV2` | state → `unlocked` |

## Configuration

Settings are loaded in priority order: **defaults → config file → environment variables**.
Environment variables always win.

### Config file

Mount a TOML file at `/config/homelink.toml` (or set `CONFIG_FILE` to override the path).

```toml
[yolink]
api_host    = "https://api.yosmart.com"
mqtt_broker = "tcp://mqtt.api.yosmart.com:8003"

[frigate]
base_url = "http://frigate:5000"

[server]
port      = 8080
log_level = "warn"   # debug | info | warn | error

[[mappings]]
yolink_device = "Back Door"     # exact name from the YoLink app
camera        = "Patio"         # Frigate camera name
label         = "door"          # Frigate event label
sub_label     = "back door"     # optional secondary label
duration      = 30              # event duration in seconds (default 30)
# pre_capture     = 5           # seconds of footage before trigger (Frigate default if omitted)
# include_recording = true      # attach a recording clip (Frigate default if omitted)
# bounding_box  = [0.0, 0.0, 1.0, 1.0]  # [x, y, dx, dy]: top-left corner + size, 0.0–1.0
```

### Environment variables

Credentials must be supplied via environment variables and are never read from the config file.

| Variable | Required | Default | Description |
|---|---|---|---|
| `YOLINK_CLIENT_ID` | yes | — | YoLink API client ID (UAID) |
| `YOLINK_CLIENT_SECRET` | yes | — | YoLink API secret key |
| `FRIGATE_BASE_URL` | yes* | — | Frigate base URL (*or set via `[frigate] base_url`) |
| `FRIGATE_USER` | no | — | Frigate native auth username |
| `FRIGATE_PASSWORD` | no | — | Frigate native auth password |
| `FRIGATE_INSECURE_SKIP_VERIFY` | no | — | Set `true` to skip TLS cert verification (self-signed certs) |
| `YOLINK_API_HOST` | no | `https://api.yosmart.com` | Override YoLink API host |
| `MQTT_BROKER` | no | `tcp://mqtt.api.yosmart.com:8003` | Override MQTT broker URL |
| `PORT` | no | `8080` | HTTP server port |
| `LOG_LEVEL` | no | `warn` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `CONFIG_FILE` | no | `/config/homelink.toml` | Path to config file |

### YoLink credentials

Create a **User Access Credential** in the YoLink mobile app:

> **Settings → Account → Advanced Settings → User Access Credentials → Create**

This yields a **UAID** (`YOLINK_CLIENT_ID`) and a **Secret Key** (`YOLINK_CLIENT_SECRET`).

## Deployment

### Docker Compose

Create a `.env` file with your credentials:

```env
YOLINK_CLIENT_ID=your-uaid
YOLINK_CLIENT_SECRET=your-secret
```

```yaml
services:
  homelink:
    image: ghcr.io/tyandl/homelink:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config:/config:ro
    environment:
      YOLINK_CLIENT_ID: ${YOLINK_CLIENT_ID}
      YOLINK_CLIENT_SECRET: ${YOLINK_CLIENT_SECRET}
      FRIGATE_USER: ${FRIGATE_USER:-}
      FRIGATE_PASSWORD: ${FRIGATE_PASSWORD:-}
      PORT: ${PORT:-8080}
      LOG_LEVEL: ${LOG_LEVEL:-warn}
```

Place your `homelink.toml` at `./config/homelink.toml` alongside the compose file.

### Building from source

Requires Go 1.25+.

```bash
go build ./cmd/homelink

YOLINK_CLIENT_ID=<id> YOLINK_CLIENT_SECRET=<secret> ./homelink
```

## Health check

`GET /healthz` returns `200 OK` when the service is running. Use it for container health
checks or uptime monitoring.

## License

Released under the [MIT License](LICENSE).
