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
2. **Event routing** — when a watched device fires a matching report (e.g. a door sensor
   opens), homelink triggers an action on the target service.
3. **Frigate** — receives manual event creation requests via its HTTP API. Supports
   Cloudflare Access service-token auth and optional Frigate native auth.
4. **HTTP server** — exposes `GET /healthz` for container health checks. IFTTT webhook
   support is planned but not yet implemented.

### Package layout

```
cmd/homelink/       — main binary; wires config, MQTT listener, HTTP server
internal/config/    — env var loading and validation
internal/frigate/   — Frigate HTTP client (CF Access transport, login, event creation)
```

### Configuration (environment variables)

| Variable | Required | Default | Description |
|---|---|---|---|
| `YOLINK_CLIENT_ID` | yes | — | YoLink API client ID |
| `YOLINK_CLIENT_SECRET` | yes | — | YoLink API client secret |
| `FRIGATE_BASE_URL` | yes | — | Frigate base URL (e.g. `https://frigate.example.com`) |
| `CF_ACCESS_CLIENT_ID` | no | — | Cloudflare Access service-token ID |
| `CF_ACCESS_CLIENT_SECRET` | no | — | Cloudflare Access service-token secret |
| `FRIGATE_USER` | no | — | Frigate native auth username |
| `FRIGATE_PASSWORD` | no | — | Frigate native auth password |
| `PORT` | no | `8080` | HTTP server port |
| `LOG_LEVEL` | no | `warn` | Log verbosity: debug, info, warn, error |

### Planned

- IFTTT outbound webhook (push event to Maker channel)
- Configurable device→action mappings via env vars (device name, camera, label)

## Style

- Prefer descriptive variable names over single- or two-letter abbreviations.
