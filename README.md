# homelink

A Go client library for smart home APIs. `homelink` currently supports:

- **[YoLink/YoSmart](https://www.yosmart.com/)** (`api.yosmart.com`) — authentication and token
  refresh, HTTP and MQTT transports, typed controller for every supported YoLink device; list
  devices, read state, send commands, subscribe to real‑time reports.
- **[Flair](https://flair.co/)** (`api.flair.co`) — OAuth 2.0 client-credentials auth with
  token refresh, typed REST/JSON:API client for structures, rooms, vents, pucks, HVAC units,
  thermostats, schedules, geofences, and sensor readings.

The module is `github.com/tyandl/homelink`. It is primarily a **library**; the programs under
`cmd/` demonstrate each API (see [Example CLIs](#example-clis)).

## Installation

```bash
go get github.com/tyandl/homelink
```

Requires Go 1.24+.

## Credentials

Both libraries accept credentials as `func() string` suppliers so they can be read from a
vault, environment, or any other source at call time. Neither library reads the environment
itself.

### YoLink

Authenticate with a YoLink **User Access Credential** (UAC): a client id (UAID) and a secret
key. Create one in the YoLink mobile app:

> **Settings → Account → Advanced Settings → User Access Credentials → Create** — this yields a
> **UAID** (your client id) and a **Secret Key** (your client secret).

- YoLink API documentation: <http://doc.yosmart.com/docs/yolinkapi>
- Getting started / credentials & authorization: <http://doc.yosmart.com>

The example CLI reads credentials from the `yolink_client_id` and `yolink_client_secret`
environment variables.

### Flair

Authenticate with Flair OAuth 2.0 credentials (client id and client secret). Obtain them in
the Flair mobile app:

> **Account → Account Settings → Developer Settings** — create a new OAuth 2.0 client to
> receive a **Client ID** and **Client Secret**.

- Flair API documentation: <https://documenter.getpostman.com/view/5353571/TzsbKTAG>

The example CLI reads credentials from the `flair_client_id` and `flair_client_secret`
environment variables.

> **Note:** the Flair API rate-limits access-token creation to roughly 50 requests per day.
> The client automatically reuses the refresh token to renew access tokens without counting
> against that limit.

## Library usage

### YoLink

#### Connect and list devices

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/tyandl/homelink/pkg/yolink/controller"
	"github.com/tyandl/homelink/pkg/yolink/devices" // registers the typed device controllers
)

func main() {
	home, err := controller.NewHome(
		"https://api.yosmart.com",
		func() string { return os.Getenv("yolink_client_id") },
		func() string { return os.Getenv("yolink_client_secret") },
	)
	if err != nil {
		log.Fatal(err)
	}

	deviceList, err := home.GetDeviceList()
	if err != nil {
		log.Fatal(err)
	}
	for _, device := range deviceList {
		fmt.Printf("%s\t%s\t%s\n", device.GetName(), device.GetType(), device.GetId())
	}

	// Resolve a device by name and call a typed method on its concrete controller.
	device, err := home.GetDeviceByName("Front Door")
	if err != nil {
		log.Fatal(err)
	}
	if sensor, ok := device.(*devices.DoorSensor); ok {
		state, err := sensor.GetState()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("state=%s battery=%s\n", state.State.State, state.State.Battery)
	}
}
```

Importing `pkg/yolink/devices` (even as a blank import `_`) registers the typed controllers, so
`GetDeviceList`/`GetDeviceByName` return concrete types such as `*devices.DoorSensor` or
`*devices.Switch` that you can type‑assert and call typed methods on (e.g. `GetState`,
`SetState`). Devices without a registered type fall back to the untyped `controller.Device`.

#### Send a command

```go
if lamp, ok := device.(*devices.Switch); ok {
	_, err := lamp.SetState(devices.SwitchSetStateParams{State: types.SwitchCommandOpen})
	if err != nil {
		log.Fatal(err)
	}
}
```

#### Subscribe to real‑time reports (MQTT)

```go
if err := home.InitializeMqtt("tcp://mqtt.api.yosmart.com:8003"); err != nil {
	log.Fatal(err)
}
defer home.CloseMqtt()

reports, stop, err := home.Subscribe() // whole home; or device.Subscribe() for one device
if err != nil {
	log.Fatal(err)
}
defer stop()

for report := range reports {
	fmt.Printf("%s reported %s: %v\n", report.Device.GetName(), report.Event, report.Data)
}
```

> **Note:** the YoLink cloud MQTT broker is plaintext TCP (port 8003); it exposes no TLS
> endpoint, and the access token travels as the MQTT username. Use the Local Hub broker on a
> trusted LAN if you need transport encryption.

#### Save and restore a home

`Home.Save` serializes the host, home info, and device list to JSON so a later `RestoreHome`
can rebuild the home without re‑fetching the device list (credentials are supplied again at
restore time and are never written to the blob):

```go
blob, err := home.Save()
// ... persist blob ...

home, err = controller.RestoreHome(idFn, secretFn, blob)
```

### Flair

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/tyandl/homelink/pkg/flair"
	flairTypes "github.com/tyandl/homelink/pkg/flair/types"
)

func main() {
	client, err := flair.NewClient(
		"https://api.flair.co",
		func() string { return os.Getenv("flair_client_id") },
		func() string { return os.Getenv("flair_client_secret") },
	)
	if err != nil {
		log.Fatal(err)
	}

	structures, err := client.GetStructures()
	if err != nil {
		log.Fatal(err)
	}

	for _, structure := range structures.Resources {
		fmt.Println(structure.Attributes.Name)

		rooms, err := client.GetRooms(structure.ID)
		if err != nil {
			log.Fatal(err)
		}
		for _, room := range rooms.Resources {
			vents, err := client.GetVents(room.ID)
			if err != nil {
				log.Fatal(err)
			}
			for _, vent := range vents.Resources {
				fmt.Printf("  %s  %d%%\n", vent.Attributes.Name, vent.Attributes.PercentOpen)
			}
		}
	}

	// Set a structure to manual mode and open all vents to 75 %.
	structure := structures.Resources[0]
	_, err = client.UpdateStructure(structure.ID, flair.StructurePatch{
		Mode: flair.Ptr(flairTypes.SystemModeManual),
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

Relationship links embedded in each resource let you traverse the object graph without
hardcoding URLs. Use the package‑level helpers when you want to follow them directly:

```go
// Follow a relationship link returned by the API rather than constructing a URL.
rooms, err := flair.ListRelated[flair.RoomAttributes](client, structure.Rel("rooms").Related())
```

## Example CLIs

The programs under `cmd/` are demonstration tools built on the library — handy references and
a convenient way to poke at each API from the shell. They are **not** the library's purpose;
treat them as example code.

### `cmd/yolink` — YoLink CLI

[`cmd/yolink`](cmd/yolink/yolink_api.go) is a full‑featured CLI for the YoLink API.

```bash
go build ./cmd/yolink

export yolink_client_id=<UAID>
export yolink_client_secret=<secret>

./yolink_api ls                                   # list devices: name<TAB>type<TAB>id
./yolink_api do getState --on "Front Door"        # call a method, print the typed response
./yolink_api do setState --on "Lamp" --state open # pass command parameters
./yolink_api listen                               # stream reports as JSON, one per line
./yolink_api listen --on "Front Door" | jq        # stream a single device's reports
```

Global flags: `--output json|go`, `--log debug|info|warn|error`, and
`--client-id` / `--client-secret` (override the environment variables). Run `./yolink_api help`
for the full usage.

### `cmd/flair` — Flair CLI (stub)

[`cmd/flair`](cmd/flair/flair_api.go) is a minimal stub that authenticates and lists
structures. It is the intended starting point for a fuller Flair CLI.

```bash
go build ./cmd/flair

export flair_client_id=<client-id>
export flair_client_secret=<client-secret>

./flair_api
```

## Scripts

The scripts in [`scripts/`](scripts/) are plain Python 3 (standard library only — no
dependencies to install).

- **`gen_device_types.py`** regenerates the typed device controllers
  (`pkg/yolink/devices/*_gen.go`) from the per‑device `pkg/yolink/devices/*.json` definitions.
  Run it after adding or editing a device definition:

  ```bash
  python3 scripts/gen_device_types.py
  go build ./...   # verify the generated code compiles
  ```

- **`gen_bash_completion.py`** (in `cmd/yolink/`) generates a bash completion script for the
  example CLI from the same device definitions (completing commands, device types, methods, and
  parameters):

  ```bash
  python3 cmd/yolink/gen_bash_completion.py > completions/yolink_api.bash
  source completions/yolink_api.bash   # or install under /etc/bash_completion.d/
  ```

## Project layout

```
pkg/yolink/controller/  HTTP + MQTT transport, Home, device controllers, save/restore
pkg/yolink/devices/     generated typed device controllers + their JSON definitions
pkg/yolink/types/       shared API types (timestamps, enums, status codes, temperature)
pkg/flair/              Flair REST/JSON:API client (structures, rooms, vents, pucks, …)
pkg/flair/types/        Flair-specific enums and auth types
cmd/yolink/             YoLink example CLI (binary: yolink_api)
cmd/flair/              Flair example CLI stub (binary: flair_api)
scripts/                code generators (YoLink device types)
```

## Contributing

Contributions are welcome. To keep history clean and verifiable:

- **Sign your commits.** All commits must be cryptographically signed and verify (`git commit -S`).
  Configure a signing key once, e.g.:

  ```bash
  git config user.signingkey <your-key>
  git config commit.gpgsign true   # sign every commit automatically
  ```

  (SSH signing works too: `git config gpg.format ssh`.)

- **Follow [Conventional Commits](https://www.conventionalcommits.org/).** Commit messages use a
  `type(scope): summary` header — e.g. `feat(devices): add support for the X sensor`,
  `fix(controller): handle expired token on refresh`, `docs: clarify credential setup`.

- **Regenerate, don't hand‑edit.** `pkg/yolink/devices/*_gen.go` and `completions/yolink_api.bash`
  are generated. Change the source (`*.json` definitions or the scripts) and re‑run the
  [scripts](#scripts).

- **Keep it green.** Before opening a pull request:

  ```bash
  gofmt -l .          # should print nothing
  go vet ./...
  go test ./...
  ```

## License

Released under the [MIT License](LICENSE).
