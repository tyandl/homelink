#!/usr/bin/env python3
"""Generate Go device files from pkg/yolink/devices/{snake_name}.json.

All devices share a single `devices` package. Each definition is co-located with
its generated file directly in pkg/yolink/devices/ (no per-device subdirectories).

Outputs (per device):
  pkg/yolink/devices/{snake_name}_gen.go   — struct + methods + event response types

Enum types are hand-maintained in pkg/yolink/types/enums.go (not generated).
The home-level API (Home, Device, the device-controller registry, and device lookups)
is hand-maintained in pkg/yolink/controller/device.go (not generated). Generated device
files live in the devices package and register themselves with that registry.
"""
import json
import re
from pathlib import Path

_GO_IDENT_RE = re.compile(r'^[A-Za-z_][A-Za-z0-9_]*$')

ROOT        = Path(__file__).parent.parent
DEVICES_DIR = ROOT / "pkg/yolink/devices"
MODULE         = "github.com/tyandl/homelink"
TYPES_PKG      = f"{MODULE}/pkg/yolink/types"
CONTROLLER_PKG = f"{MODULE}/pkg/yolink/controller"
DEVICES_PKG    = "devices"

# Keys that are schema metadata, never property names — except "items" (see note below).
# "items" IS metadata inside an Array-typed leaf, but can also be a real property name
# in implicit-object nodes (e.g., Lock.listPasswords.response.data.items).
# Resolution: build_struct_body skips only STRUCT_META; is_typed_leaf allows LEAF_META.
STRUCT_META = {"type", "required", "comment"}
LEAF_META   = {"type", "required", "comment", "items"}


# ---------------------------------------------------------------------------
# Name helpers
# ---------------------------------------------------------------------------

def to_pascal(name: str) -> str:
    """camelCase / snake_case → PascalCase. Strips bracket suffixes (e.g. sches[weekday] → Sches)."""
    name = name.strip("`")
    name = re.sub(r"\[.*", "", name)  # drop trailing [...]
    parts = re.split(r"[_\-]", name)
    return "".join(p[0].upper() + p[1:] if p else "" for p in parts)


def to_snake(name: str) -> str:
    """PascalCase → snake_case  (COSmokeSensor → co_smoke_sensor, LockV2 → lock_v2)."""
    s = re.sub(r"([A-Z]+)([A-Z][a-z])", r"\1_\2", name)
    s = re.sub(r"([a-z\d])([A-Z])", r"\1_\2", s)
    return s.lower()


def to_go_method(name: str) -> str:
    """Capitalise first letter for an exported Go method."""
    return name[0].upper() + name[1:] if name else name


# ---------------------------------------------------------------------------
# Node classification
# ---------------------------------------------------------------------------

def is_typed_leaf(node) -> bool:
    """True when the node is a typed scalar / array leaf (no sub-properties)."""
    if not isinstance(node, dict) or "type" not in node:
        return False
    extra = [k for k in node if k not in LEAF_META]
    return len(extra) == 0


def is_implicit_object(node) -> bool:
    """True when node is a dict with sub-properties (not a typed leaf, not a string)."""
    if not isinstance(node, dict):
        return False
    if is_typed_leaf(node):
        return False
    return any(k not in STRUCT_META for k in node)


# ---------------------------------------------------------------------------
# Go-type conversion
# ---------------------------------------------------------------------------

SCALAR_MAP = {
    "String":        "string",
    "Integer":       "int",
    "Boolean":       "bool",
    "Float":         "float32",
    "Double":        "float64",
    "Number":        "float64",
    "Long":          "int64",
    "Timestamp":     "types.Timestamp",
    "LoraInfo":      "types.LoraInfo",
    "Date":          "time.Time",
    "Datetime":      "time.Time",
    "Object":        "interface{}",
    "String[]":       "[]string",
    "Boolean[64]":    "[]bool",
    "String|Number":  "interface{}",
    "Temperature":     "types.Temperature",
    "TemperatureUnit": "types.TemperatureUnit",
    # Enum types — defined in the types package, always qualified
    "SwitchState":          "types.SwitchState",
    "SwitchCommand":        "types.SwitchCommand",
    "AlarmState":           "types.AlarmState",
    "LockState":            "types.LockState",
    "PowerSupply":          "types.PowerSupply",
    "ThermostatMode":       "types.ThermostatMode",
    "FanMode":              "types.FanMode",
    "LatchSide":            "types.LatchSide",
    "DoorState":            "types.DoorState",
    "ScheduleType":         "types.ScheduleType",
    "SprinklerMode":        "types.SprinklerMode",
    "SprinklerCommand":     "types.SprinklerCommand",
    "WaterMode":            "types.WaterMode",
    "SprinklerWaterType":   "types.SprinklerWaterType",
    "ThermostatRunning":    "types.ThermostatRunning",
    "ThermostatSchedule":   "types.ThermostatSchedule",
    "ThermostatMaster":     "types.ThermostatMaster",
    "LeakSensorMode":       "types.LeakSensorMode",
    "LeakSensorSensitivity":"types.LeakSensorSensitivity",
    "WaterLeakPlan":        "types.WaterLeakPlan",
    "WaterMeterLeakPlan":   "types.WaterMeterLeakPlan",
    "RecordMode":           "types.RecordMode",
    "NightVision":          "types.NightVision",
    "SpeakerTone":          "types.SpeakerTone",
    "LockCredentialType":   "types.LockCredentialType",
    "LockCipherType":       "types.LockCipherType",
    "CellularRegState":     "types.CellularRegState",
    "SmartRemoterEventType":"types.SmartRemoterEventType",
    "ThermostatSwitch":     "types.ThermostatSwitch",
    "SprinklerDaysType":    "types.SprinklerDaysType",
    "WiFiEncryption":       "types.WiFiEncryption",
    "AlertState":           "types.AlertState",
    # Ordinal enums
    "BatteryLevel":          "types.BatteryLevel",
    "BatteryState":          "types.BatteryState",
    "LockSoundLevel":        "types.LockSoundLevel",
    "MeterUnit":             "types.MeterUnit",
    "WaterTemperatureLevel": "types.WaterTemperatureLevel",
}


def node_to_go_type(node, indent: int = 0) -> str:
    """Return the Go type string for a schema node (possibly multi-line for structs)."""
    pad = "    " * indent

    # Bare string descriptor like "<List<String>,Necessary> description"
    if isinstance(node, str):
        lower = node.lower()
        if "list<string>" in lower or "array<string>" in lower:
            return "[]string"
        if "list<integer>" in lower or "array<integer>" in lower:
            return "[]int"
        return "[]string"

    if not isinstance(node, dict):
        return "interface{}"

    if is_typed_leaf(node):
        raw = node["type"]
        if raw in SCALAR_MAP:
            return SCALAR_MAP[raw]
        if raw == "Array":
            items = node.get("items")
            if items and isinstance(items, dict):
                inner = build_struct_body(items, indent + 1)
                return f"[]struct {{\n{inner}{pad}}}"
            return "[]interface{}"
        return "string"  # unknown scalar fallback

    # Implicit object — recurse into sub-properties
    inner = build_struct_body(node, indent + 1)
    return f"struct {{\n{inner}{pad}}}"


def build_struct_body(props_node: dict, indent: int = 1, type_prefix: str = "", hoisted: list = None) -> str:
    """Return Go struct field lines for a properties dict.

    Implicit object fields are hoisted to named types (appended to hoisted) so
    they can carry String() methods. hoisted is ordered deepest-first so Go sees
    declarations before use.
    """
    if hoisted is None:
        hoisted = []
    if not isinstance(props_node, dict):
        return ""
    pad = "    " * indent
    lines = []
    for raw_key, value in props_node.items():
        if raw_key in STRUCT_META:
            continue
        key = raw_key.strip("`")
        field_name = to_pascal(key)
        if not _GO_IDENT_RE.match(field_name):
            continue
        required = value.get("required", True) if isinstance(value, dict) else True

        if isinstance(value, dict) and is_implicit_object(value):
            nested_name = f"{type_prefix}{field_name}" if type_prefix else field_name
            nested_body = build_struct_body(value, 1, nested_name, hoisted)
            nested_str = build_string_method(nested_name, value)
            hoisted.append(f"type {nested_name} struct {{\n{nested_body}}}\n\n{nested_str}")
            go_type = nested_name
        elif isinstance(value, dict) and is_typed_leaf(value) and value.get("type") == "Array":
            items = value.get("items")
            if isinstance(items, dict) and is_implicit_object(items):
                item_name = f"{type_prefix}{field_name}Item" if type_prefix else f"{field_name}Item"
                item_body = build_struct_body(items, 1, item_name, hoisted)
                item_str = build_string_method(item_name, items)
                hoisted.append(f"type {item_name} struct {{\n{item_body}}}\n\n{item_str}")
                go_type = f"[]{item_name}"
            else:
                go_type = node_to_go_type(value, indent)
        else:
            go_type = node_to_go_type(value, indent)
            if not required and isinstance(value, dict) and is_typed_leaf(value):
                go_type = f"*{go_type}"

        json_tag = key if required else f"{key},omitempty"
        comment = ""
        if isinstance(value, dict) and value.get("comment"):
            comment = f" // {value['comment']}"
        lines.append(f"{pad}{field_name} {go_type} `json:\"{json_tag}\"`{comment}")
    return "\n".join(lines) + ("\n" if lines else "")


# ---------------------------------------------------------------------------
# Per-method type generation
# ---------------------------------------------------------------------------

def build_string_method(type_name: str, props_node: dict) -> str:
    """Return a String() method for a named struct type.

    Non-pointer fields are passed directly to fmt.Sprintf (%v calls String() on
    named types, prints primitives as-is).  Pointer fields are dereferenced via a
    local variable so nil prints as <nil> and non-nil prints the underlying value.
    """
    if not isinstance(props_node, dict):
        return ""

    fmt_parts: list[str] = []
    pre_lines: list[str] = []
    args: list[str] = []
    ptr_idx = 0

    for raw_key, value in props_node.items():
        if raw_key in STRUCT_META:
            continue
        key = raw_key.strip("`")
        field_name = to_pascal(key)
        if not _GO_IDENT_RE.match(field_name):
            continue
        required = value.get("required", True) if isinstance(value, dict) else True
        is_ptr = not required and isinstance(value, dict) and is_typed_leaf(value)

        fmt_parts.append(f"{field_name}:%v")

        if is_ptr:
            var = f"ptr{ptr_idx}"
            ptr_idx += 1
            pre_lines += [
                f'\tvar {var} any = "<nil>"',
                f"\tif r.{field_name} != nil {{",
                f"\t\t{var} = fmt.Sprint(*r.{field_name})",
                "\t}",
            ]
            args.append(var)
        else:
            args.append(f"r.{field_name}")

    fmt_str = "{" + " ".join(fmt_parts) + "}"
    args_str = ", ".join(args)

    lines = [f"func (r {type_name}) String() string {{"]
    lines.extend(pre_lines)
    lines.append(f'\treturn fmt.Sprintf("{fmt_str}", {args_str})')
    lines.append("}")
    return "\n".join(lines) + "\n"


def build_params_type(prefix: str, params_node):
    """Return (type_name, type_decl) for a method's params, or (None, None) if absent."""
    if not params_node or not isinstance(params_node, dict):
        return None, None
    skip = {"targetDevice", "token", "method"}
    props = {k: v for k, v in params_node.items() if k not in skip and k not in STRUCT_META}
    if not props:
        return None, None
    type_name = f"{prefix}Params"
    hoisted = []
    body = build_struct_body(props, 1, type_name, hoisted)
    string_method = build_string_method(type_name, props)
    hoisted_str = "".join(h.rstrip() + "\n\n" for h in hoisted)
    return type_name, f"{hoisted_str}type {type_name} struct {{\n{body}}}\n\n{string_method}"


def build_response_type(prefix: str, response_node):
    """Return (type_name, type_decl) for a method's response data.
    Falls back to ("any", None) when there is no data.
    """
    if not response_node or not isinstance(response_node, dict):
        return "any", None
    ref = response_node.get("ref")
    if ref:
        return ref, None
    # Accept either "data" or "params" as the response payload key
    data_node = response_node.get("data") or response_node.get("params")
    if not data_node:
        return "any", None

    type_name = f"{prefix}Response"

    if is_typed_leaf(data_node):
        go_type = node_to_go_type(data_node, 0)
        return type_name, f"type {type_name} = {go_type}\n"

    hoisted = []
    body = build_struct_body(data_node, 1, type_name, hoisted)
    string_method = build_string_method(type_name, data_node)
    hoisted_str = "".join(h.rstrip() + "\n\n" for h in hoisted)
    return type_name, f"{hoisted_str}type {type_name} struct {{\n{body}}}\n\n{string_method}"


# ---------------------------------------------------------------------------
# File builders
# ---------------------------------------------------------------------------

def make_header(package_name: str) -> str:
    return f"// Code generated by scripts/gen_device_types.py; DO NOT EDIT.\npackage {package_name}"


def build_device_file(device_name: str, methods: list,
                      events: list = None, package_name: str = None) -> str:
    pkg = package_name or DEVICES_PKG
    body: list[str] = []

    body.append(f"type {device_name} struct{{ *controller.Device }}")
    body.append("")
    body.append("func init() {")
    body.append(f'\tcontroller.RegisterDeviceFactory("{device_name}", func(device *controller.Device) controller.DeviceController {{')
    body.append(f"\t\treturn &{device_name}{{Device: device}}")
    body.append("\t})")
    body.append("}")
    body.append("")

    for method in methods:
        method_name = method.get("name", "")
        if not method_name or " " in method_name or method_name.startswith("/"):
            continue

        go_method   = to_go_method(method_name)
        type_prefix = f"{device_name}{go_method}"
        description = method.get("description", "")
        request_node  = method.get("request", {})
        response_node = method.get("response", {})

        params_name, params_decl     = build_params_type(type_prefix, request_node.get("params"))
        response_type, response_decl = build_response_type(type_prefix, response_node)

        if params_decl:
            body.append(params_decl.rstrip())
            body.append("")
        if response_decl:
            body.append(response_decl.rstrip())
            body.append("")

        if description:
            body.append(f"// {go_method} {description}")

        signature_params = f"params {params_name}" if params_name else ""
        params_value     = "params" if params_name else "nil"
        has_response     = response_type != "any"

        # Synchronous method: calls through the Transporter (device.Call carries this device's
        # context) and deserializes the response payload. Methods with a typed response return
        # it; the rest return only an error.
        if has_response:
            body.append(
                f"func (device *{device_name}) {go_method}({signature_params}) "
                f"({response_type}, error) {{"
            )
            body.append(f"\tvar response {response_type}")
            body.append(f'\terr := device.Call("{device_name}.{method_name}", {params_value}, &response)')
            body.append("\treturn response, err")
        else:
            body.append(f"func (device *{device_name}) {go_method}({signature_params}) error {{")
            body.append(f'\treturn device.Call("{device_name}.{method_name}", {params_value}, nil)')

        body.append("}")
        body.append("")

        # Asynchronous variant: returns a channel carrying the typed result. device.CallAsync
        # publishes (carrying this device's context); controller.DecodeAsync types the response.
        body.append(
            f"func (device *{device_name}) {go_method}Async({signature_params}) "
            f"<-chan controller.AsyncResult[{response_type}] {{"
        )
        body.append(
            f'\treturn controller.DecodeAsync[{response_type}](device.CallAsync("{device_name}.{method_name}", {params_value}))'
        )
        body.append("}")
        body.append("")

    for event in (events or []):
        event_name = event.get("name", "")
        if not event_name or " " in event_name or event_name.startswith("/"):
            continue
        go_event = to_go_method(event_name)
        type_prefix = f"{device_name}{go_event}"
        response_node = event.get("response", {})
        _, response_decl = build_response_type(type_prefix, response_node)
        if response_decl:
            body.append(response_decl.rstrip())
            body.append("")

    # ParseEvent — delegate to the embedded Device's base implementation to decode the
    # envelope, then attribute the report to this concrete device and type the payload by
    # event name.
    cases = []
    for entry in list(methods) + list(events or []):
        entry_name = entry.get("name", "")
        if not entry_name or " " in entry_name or entry_name.startswith("/"):
            continue
        type_prefix = f"{device_name}{to_go_method(entry_name)}"
        response_type, _ = build_response_type(type_prefix, entry.get("response", {}))
        if response_type == "any":
            continue
        cases.append((entry_name, response_type))

    body.append(f"func (device *{device_name}) ParseEvent(message controller.MqttMessage) (controller.Report, bool) {{")
    body.append("\treport, ok := device.Device.ParseEvent(message) // base impl decodes event + raw data")
    body.append("\tif !ok {")
    body.append("\t\treturn report, false")
    body.append("\t}")
    body.append("\treport.Device = device // attribute the report to this concrete device")
    if cases:
        body.append("\tdata := report.Data.(controller.Unknown).Json // raw payload, re-typed below")
        body.append("\tswitch report.Event {")
        for entry_name, response_type in cases:
            body.append(f'\tcase "{device_name}.{entry_name}":')
            body.append(f"\t\tvar response {response_type}")
            body.append("\t\tif err := json.Unmarshal(data, &response); err == nil {")
            body.append("\t\t\treport.Data = response")
            body.append("\t\t}")
        body.append("\t}")
    body.append("\treturn report, true")
    body.append("}")
    body.append("")

    body_str = "\n".join(body)

    out: list[str] = [make_header(pkg), ""]
    stdlib, external = [], []
    if "fmt." in body_str:
        stdlib.append('"fmt"')
    if "json." in body_str:
        stdlib.append('"encoding/json"')
    if "time.Time" in body_str:
        stdlib.append('"time"')
    if "controller." in body_str:
        external.append(f'"{CONTROLLER_PKG}"')
    if "types." in body_str:
        external.append(f'"{TYPES_PKG}"')

    if len(stdlib) + len(external) == 1:
        out.append(f"import {(stdlib + external)[0]}")
    elif stdlib or external:
        out.append("import (")
        for imp in stdlib:
            out.append(f"\t{imp}")
        if stdlib and external:
            out.append("")
        for imp in external:
            out.append(f"\t{imp}")
        out.append(")")
    out.append("")
    out.append(body_str)
    return "\n".join(out)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def generate():
    # Read all device definitions (one <snake>.json per device, all in DEVICES_DIR).
    devices: dict[str, dict] = {}
    for defn_path in sorted(DEVICES_DIR.glob("*.json")):
        defn = json.load(open(defn_path))
        devices[defn["name"]] = defn

    method_count = 0
    for device_name, device_data in devices.items():
        methods = device_data.get("methods", [])
        events  = device_data.get("events", [])
        snake   = to_snake(device_name)

        content = build_device_file(
            device_name, methods,
            events=events,
            package_name=DEVICES_PKG,
        )
        out_path = DEVICES_DIR / f"{snake}_gen.go"
        out_path.write_text(content)

        count = sum(1 for m in methods if m.get("name") and " " not in m["name"] and not m["name"].startswith("/"))
        method_count += count
        print(f"  {snake}_gen.go  ({count} methods)")

    print(f"\n{len(devices)} devices  ({method_count} total methods)")


if __name__ == "__main__":
    generate()
