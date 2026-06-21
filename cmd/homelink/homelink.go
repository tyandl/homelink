package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/tyandl/homelink/pkg/yolink/controller"
	// Imported for side effects: each generated device file registers its typed controller
	// with the controller package from init(), so GetDeviceList returns concrete devices.*
	// types. `do` then dispatches to their typed methods (via reflection) and `listen` types
	// each report, instead of falling back to the untyped Device.
	_ "github.com/tyandl/homelink/pkg/yolink/devices"
)

// apiHost is the YoLink HTTP API base URL.
const apiHost = "https://api.yosmart.com"

// mqttBroker is plaintext (tcp) by design: as of 2026-06-18 the YoLink cloud broker exposes
// no TLS endpoint. The Open API V2 docs only list TCP 8003 / WebSocket 8004, the vendor's own
// yolink-api client connects without TLS, and a probe found 8003 plaintext-only and 8883
// refused. The access token travels as the MQTT username in cleartext; for encryption use the
// Local Hub broker on a trusted LAN. Re-check periodically for a documented TLS port.
const mqttBroker = "tcp://mqtt.api.yosmart.com:8003"

// Credentials supplied via global flags. When non-empty they take precedence over the
// yolink_client_id / yolink_client_secret environment variables.
var (
	clientId     string
	clientSecret string
)

// outputFormat selects how typed results are rendered: "json" (default, pipe-friendly) or
// "go" (the value's Go String() form). Set from the global --output flag.
var outputFormat = "json"

func main() {
	// Global flags precede the subcommand, e.g. `homelink --log info do ...`. They are kept
	// out of the subcommands so the custom `do` parser only ever sees method params.
	global := flag.NewFlagSet("homelink", flag.ContinueOnError)
	global.Usage = usage
	logLevel := global.String("log", "warn", "log verbosity on stderr: debug, info, warn, or error")
	global.StringVar(&clientId, "client-id", "", "YoLink client id (overrides $yolink_client_id)")
	global.StringVar(&clientSecret, "client-secret", "", "YoLink client secret (overrides $yolink_client_secret)")
	global.StringVar(&outputFormat, "output", outputFormat, "output format for typed results: json or go")
	if err := global.Parse(os.Args[1:]); err != nil {
		os.Exit(2) // flag already printed the error and usage
	}
	if err := configureLogging(*logLevel); err != nil {
		fmt.Fprintf(os.Stderr, "homelink: %v\n", err)
		os.Exit(2)
	}
	if outputFormat != "json" && outputFormat != "go" {
		fmt.Fprintf(os.Stderr, "homelink: invalid --output %q (want json or go)\n", outputFormat)
		os.Exit(2)
	}

	args := global.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	command, commandArgs := args[0], args[1:]
	var err error
	switch command {
	case "ls":
		err = runList(commandArgs)
	case "listen":
		err = runListen(commandArgs)
	case "do":
		err = runDo(commandArgs)
	case "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "homelink: unknown command %q\n\n", command)
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "homelink: %v\n", err)
		os.Exit(1)
	}
}

// configureLogging points slog at stderr filtered to the named level, so log output never
// mixes into the stdout data that commands pipe to grep/jq.
func configureLogging(levelName string) error {
	var level slog.Level
	switch strings.ToLower(levelName) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return fmt.Errorf("invalid --log level %q (want debug, info, warn, or error)", levelName)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	return nil
}

func usage() {
	fmt.Fprint(os.Stderr, `homelink - control YoLink devices from the command line

Usage:
  homelink [--log <level>] <command> [args]

Global flags:
  --log <level>
        Log verbosity on stderr: debug, info, warn (default), or error.
  --output <format>
        Output format for typed results: json (default) or go.
  --client-id <id>, --client-secret <secret>
        YoLink API credentials. Take precedence over the yolink_client_id
        and yolink_client_secret environment variables.

Commands:
  homelink ls
        List every device as "name<TAB>type<TAB>id".

  homelink listen [--on <device_name>]
        Subscribe to reports and print one JSON object per line. With --on,
        only the named device's reports are printed; otherwise the whole home.

  homelink do <method> --on <device_name> [--<param> <value> ...]
        Invoke an API method on a device and print the JSON response. If
        <method> has no "Type." prefix, the device's type is prepended
        (e.g. "setState" on a Switch becomes "Switch.setState").

Credentials are read from the yolink_client_id and yolink_client_secret
environment variables.
`)
}

// connect builds a Home from the resolved credentials: the --client-id/--client-secret flags
// when given, otherwise the yolink_client_id/yolink_client_secret environment variables.
func connect() (*controller.Home, error) {
	id := firstNonEmpty(clientId, os.Getenv("yolink_client_id"))
	secret := firstNonEmpty(clientSecret, os.Getenv("yolink_client_secret"))
	if id == "" || secret == "" {
		return nil, fmt.Errorf("provide credentials via --client-id/--client-secret or the yolink_client_id/yolink_client_secret environment variables")
	}
	return controller.NewHome(
		apiHost,
		func() string { return id },
		func() string { return secret },
	)
}

// emit writes a typed value to stdout in the selected output format: its Go String()/struct
// form when --output go, otherwise JSON. Types with json tags (the device response/report
// types) marshal to their documented field names.
func emit(value any) error {
	if outputFormat == "go" {
		fmt.Printf("%+v\n", value) // Stringer types render via String(); others show field names
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// firstNonEmpty returns the first non-empty string, used to prefer an explicit flag over an
// environment fallback.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// connectWithDevices builds a Home and preloads its device list, so the typed device
// controllers exist before any command or report needs to (de)serialize against them.
// Commands that interact with devices use this rather than relying on lazy loading, which
// for a whole-home subscription would otherwise leave the first reports untyped.
func connectWithDevices() (*controller.Home, error) {
	home, err := connect()
	if err != nil {
		return nil, err
	}
	if _, err := home.GetDeviceList(); err != nil {
		return nil, err
	}
	return home, nil
}

// runList implements `homelink ls`.
func runList(args []string) error {
	flags := flag.NewFlagSet("ls", flag.ExitOnError)
	flags.Parse(args)

	home, err := connect()
	if err != nil {
		return err
	}
	devices, err := home.GetDeviceList()
	if err != nil {
		return err
	}
	for _, device := range devices {
		fmt.Printf("%s\t%s\t%s\n", device.GetName(), device.GetType(), device.GetId())
	}
	return nil
}

// runListen implements `homelink listen [--on <device_name>]`.
func runListen(args []string) error {
	flags := flag.NewFlagSet("listen", flag.ExitOnError)
	deviceName := flags.String("on", "", "subscribe to a single device by name (default: whole home)")
	flags.Parse(args)

	home, err := connectWithDevices()
	if err != nil {
		return err
	}
	if err := home.InitializeMqtt(mqttBroker); err != nil {
		return err
	}
	defer home.CloseMqtt()

	// Pick the subscriber: a single device when --on is given, otherwise the whole home.
	var subscriber controller.DeviceController = home
	if *deviceName != "" {
		device, err := home.GetDeviceByName(*deviceName)
		if err != nil {
			return err
		}
		subscriber = device
	}
	reports, stop, err := subscriber.Subscribe()
	if err != nil {
		return err
	}
	defer stop()

	// Translate Ctrl-C / SIGTERM into closing the report channel so the loop below ends cleanly.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		stop()
	}()

	for report := range reports {
		if err := emit(newReportLine(report)); err != nil {
			return err
		}
	}
	return nil
}

// reportLine is the one-object-per-line output of `homelink listen`. It deliberately omits
// the device token (which Device's own JSON form would include) so reports are safe to pipe.
type reportLine struct {
	Device   string `json:"device"`
	DeviceId string `json:"deviceId"`
	Type     string `json:"type"`
	Event    string `json:"event"`
	Data     any    `json:"data"`
}

func newReportLine(report controller.Report) reportLine {
	line := reportLine{Event: report.Event, Data: report.Data}
	if report.Device != nil {
		line.Device = report.Device.GetName()
		line.DeviceId = report.Device.GetId()
		line.Type = report.Device.GetType()
	}
	return line
}

// runDo implements `homelink do <method> --on <device_name> [--<param> <value> ...]`.
func runDo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("do: missing method name (usage: homelink do <method> --on <device> [--<param> <value> ...])")
	}
	method, rest := args[0], args[1:]

	deviceName, params, err := parseDoFlags(rest)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	if deviceName == "" {
		return fmt.Errorf("do: --on <device_name> is required")
	}

	home, err := connectWithDevices()
	if err != nil {
		return err
	}
	device, err := home.GetDeviceByName(deviceName)
	if err != nil {
		return err
	}

	// Allow a bare method like "setState"; the API expects it qualified by device type.
	if !strings.Contains(method, ".") {
		method = device.GetType() + "." + method
	}

	// Prefer the device's typed method, so the response is decoded into its real response
	// type (e.g. DoorSensorGetStateResponse) through the existing Call pathway.
	result, typed, err := callTypedMethod(device, method, params)
	if err != nil {
		return err
	}
	if typed {
		if result != nil {
			return emit(result) // json (default) or the Go String() form, per --output
		}
		return nil
	}

	// Fallback for untyped devices or methods without a generated wrapper: raw JSON out.
	var requestParams any
	if len(params) > 0 {
		requestParams = params
	}
	var response json.RawMessage
	if err := device.Call(method, requestParams, &response); err != nil {
		return err
	}
	fmt.Println(string(response))
	return nil
}

// callTypedMethod invokes the device's generated method matching the API method name (e.g.
// "DoorSensor.getState" -> the GetState method on the concrete *devices.DoorSensor), so the
// call runs through the existing typed Call pathway and yields a typed response. It reports
// typed=false when the concrete device exposes no such method, so the caller can fall back to
// a generic raw call.
func callTypedMethod(device controller.DeviceController, method string, params map[string]any) (result any, typed bool, err error) {
	name := method
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	if name == "" {
		return nil, false, nil
	}
	goMethod := strings.ToUpper(name[:1]) + name[1:]

	call := reflect.ValueOf(device).MethodByName(goMethod)
	if !call.IsValid() {
		return nil, false, nil
	}
	signature := call.Type()
	// A device command returns (response, error) or just error; anything else (e.g. a promoted
	// getter like GetId) is not a command, so leave it to the raw fallback.
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if signature.NumOut() == 0 || !signature.Out(signature.NumOut()-1).Implements(errorType) {
		return nil, false, nil
	}

	var callArgs []reflect.Value
	switch signature.NumIn() {
	case 0:
		// no parameters
	case 1:
		paramValue, perr := buildParams(signature.In(0), params)
		if perr != nil {
			return nil, false, fmt.Errorf("invalid params for %s: %w", method, perr)
		}
		callArgs = append(callArgs, paramValue)
	default:
		return nil, false, nil
	}

	results := call.Call(callArgs)
	if last := results[len(results)-1]; !last.IsNil() {
		return nil, true, last.Interface().(error)
	}
	if len(results) >= 2 {
		return results[0].Interface(), true, nil
	}
	return nil, true, nil
}

// buildParams constructs a method's parameter struct from the CLI's string-valued flags by
// marshaling them to JSON and decoding into the typed struct. It first tries coercing numeric
// and boolean literals (so integer fields accept "5"), then retries with plain strings in case
// a value belongs to a string field.
func buildParams(paramType reflect.Type, params map[string]any) (reflect.Value, error) {
	target := reflect.New(paramType)
	coerced, _ := json.Marshal(coerceParams(params))
	if err := json.Unmarshal(coerced, target.Interface()); err != nil {
		target = reflect.New(paramType) // discard any partial decode
		plain, _ := json.Marshal(params)
		if err := json.Unmarshal(plain, target.Interface()); err != nil {
			return reflect.Value{}, err
		}
	}
	return target.Elem(), nil
}

// coerceParams renders each flag value as a JSON literal: a bare number or boolean when it
// parses as one, otherwise a quoted string.
func coerceParams(params map[string]any) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(params))
	for key, value := range params {
		text := fmt.Sprint(value)
		switch {
		case text == "true" || text == "false":
			out[key] = json.RawMessage(text)
		case isNumber(text):
			out[key] = json.RawMessage(text)
		default:
			quoted, _ := json.Marshal(text)
			out[key] = quoted
		}
	}
	return out
}

func isNumber(text string) bool {
	if _, err := strconv.ParseInt(text, 10, 64); err == nil {
		return true
	}
	_, err := strconv.ParseFloat(text, 64)
	return err == nil
}

// parseDoFlags reads the `--key value` (or `--key=value`) arguments of `do`, pulling out the
// reserved --on device name and returning the rest as call params.
func parseDoFlags(args []string) (deviceName string, params map[string]any, err error) {
	params = map[string]any{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			return "", nil, fmt.Errorf("expected --flag, got %q", arg)
		}
		key := strings.TrimPrefix(arg, "--")

		var value string
		if equals := strings.IndexByte(key, '='); equals >= 0 {
			key, value = key[:equals], key[equals+1:]
		} else {
			if index+1 >= len(args) {
				return "", nil, fmt.Errorf("flag --%s has no value", key)
			}
			index++
			value = args[index]
		}

		if key == "on" {
			deviceName = value
		} else {
			params[key] = value
		}
	}
	return deviceName, params, nil
}
