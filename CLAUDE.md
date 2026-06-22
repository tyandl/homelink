# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run the main binary (requires env vars)
yolink_client_id=<id> yolink_client_secret=<secret> go run ./cmd/yolink

# Test
go test ./...

# Run a single test
go test ./cmd/yolink/... -run TestGetPhrase
```

## Architecture

This is a Go client for the **YoLink/YoSmart smart home API** (`api.yosmart.com`). The module is `github.com/tyandl/homelink`.

### Request/response flow

1. **Auth** — `cmd/yolink/yolink_api.go` calls `POST /open/yolink/token` with `client_id`/`client_secret` from env vars, receiving an `AuthResponse` (access token).
2. **API calls** — subsequent requests hit `POST /open/yolink/v2/api` with a `Bearer` token. The request body is a `BasicDownloadDataPacket` (BDDP) and the response is a `BasicUploadDataPacket` (BUDP).

### Key types (`pkg/types/`)

- `AuthResponse` — OAuth token response fields.
- `Timestamp` — wraps `time.Time`; marshals/unmarshals as Unix epoch int64 (required by the API).

### YoLink protocol types (`pkg/types/yolink/`)

- `BasicDownloadDataPacket[T]` (BDDP) — outgoing request envelope. `method` identifies the API action (e.g. `"Home.getDeviceList"`). Use `NewBDDP(method, params)` to construct one with the current timestamp.
- `BasicUploadDataPacket[T]` (BUDP) — incoming response envelope. `Code` is `"000000"` on success; see `api_doc.txt` for error codes.
- `Device` / `Devices` — device list payload; `Data` field of a BUDP response.

### Planned structure

Empty placeholder dirs exist for future expansion:
- `internal/` — internal packages
- `pkg/` — shared packages (currently holds `types/`)
- `scripts/` — utility scripts
- `test/` — integration tests
- `docs/` — documentation

## Style

- Prefer descriptive variable names over single- or two-letter abbreviations. For example, use `statusCode` instead of `sc`, `response` instead of `resp`, `device` instead of `d`.

### API reference

`api_doc.txt` lists all supported device types and their methods, plus the full error code table. The upstream docs are at `http://doc.yosmart.com/docs/yolinkapi`.
