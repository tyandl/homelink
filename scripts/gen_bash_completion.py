#!/usr/bin/env python3
"""Generate a bash completion script for the homelink CLI from the device definitions.

Reads every pkg/yolink/devices/{name}.json and emits (to stdout) a bash completion script
that completes:

  - subcommands              ls, listen, do, help
  - the global --log level   debug, info, warn, error
  - `do` method names        both qualified ("Switch.setState") and bare ("setState"),
                             so the device type can be tab-completed as the method prefix
  - `do` parameter flags     the --<param> keys of the selected method's request
  - `do` parameter values    enum choices parsed from a param's comment (e.g. ['open','close'])

Usage:
  python3 scripts/gen_bash_completion.py > completions/homelink.bash
  # then `source completions/homelink.bash`, or install under /etc/bash_completion.d/

Regenerate after changing any device definition (same source as gen_device_types.py).
"""
import json
import re
import sys
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).parent.parent
DEVICES_DIR = ROOT / "pkg/yolink/devices"

# Request keys that are envelope metadata, not user-facing call params.
REQUEST_ENVELOPE = {"method", "targetDevice", "token"}
# Keys that are schema metadata on a param leaf, never param names themselves.
LEAF_META = {"type", "required", "comment", "items"}

# Subcommands and global flag values are defined by the CLI (cmd/homelink), not the devices.
COMMANDS = ["ls", "listen", "do", "help"]
LOG_LEVELS = ["debug", "info", "warn", "error"]
OUTPUT_FORMATS = ["json", "go"]
# Global flags that precede the subcommand. --client-id/--client-secret take free-form values
# (no completion offered for them); --log and --output have fixed value sets handled separately.
GLOBAL_FLAGS = ["--log", "--output", "--client-id", "--client-secret"]


def enum_values(comment: str) -> list[str]:
    """Extract enum choices from a comment like "['open','close']"; empty if none documented."""
    if not comment or "['" not in comment:
        return []
    return re.findall(r"'([^']*)'", comment)


def load_devices() -> list[dict]:
    """Parse every device JSON definition, sorted by device name for stable output."""
    devices = []
    for path in sorted(DEVICES_DIR.glob("*.json")):
        with path.open() as handle:
            devices.append(json.load(handle))
    return devices


def collect() -> tuple[list[str], list[str], dict, dict]:
    """Walk the device definitions, returning (device_types, method_tokens, params, values).

    params maps a method token ("Type.method" and bare "method") to its set of --flags.
    values maps "token\\x00--flag" to the set of enum choices for that param.
    """
    device_types: set[str] = set()
    method_tokens: set[str] = set()
    params: dict[str, set[str]] = defaultdict(set)
    values: dict[str, set[str]] = defaultdict(set)

    for device in load_devices():
        device_type = device["name"]
        device_types.add(device_type)

        for method in device.get("methods", []):
            method_name = method["name"]
            # A few definitions (e.g. Debugger) carry raw HTTP routes as "method names" with
            # spaces and slashes; they would split a space-delimited word list, so skip them.
            if not re.fullmatch(r"[A-Za-z0-9_]+", method_name):
                continue
            qualified = f"{device_type}.{method_name}"
            method_tokens.add(qualified)
            method_tokens.add(method_name)

            request = method.get("request", {})
            param_block = request.get("params")
            # `params` may be absent, a typed leaf (no named sub-params), or an object whose
            # keys are the param names. Only an object contributes completable flags.
            if not isinstance(param_block, dict):
                continue
            for name, spec in param_block.items():
                if name in LEAF_META:
                    continue  # `params` was itself a leaf; these keys are metadata
                flag = f"--{name}"
                # A bare method name can resolve to several device types, so its flags and
                # values are the union across them; the qualified token stays type-specific.
                params[qualified].add(flag)
                params[method_name].add(flag)
                if isinstance(spec, dict):
                    for value in enum_values(spec.get("comment", "")):
                        values[f"{qualified}\x00{flag}"].add(value)
                        values[f"{method_name}\x00{flag}"].add(value)

    return sorted(device_types), sorted(method_tokens), params, values


def group_by_output(mapping: dict[str, set[str]]) -> dict[str, list[str]]:
    """Invert token->set into output-string->[tokens] so case patterns can be merged."""
    grouped: dict[str, list[str]] = defaultdict(list)
    for token, items in mapping.items():
        grouped[" ".join(sorted(items))].append(token)
    return grouped


def render_case(mapping: dict[str, set[str]], key_split: bool) -> str:
    """Render a bash `case` body. When key_split, tokens carry an embedded \\x00 to be matched
    on "$1::$2" (method token + flag); otherwise they match "$1" (method token)."""
    lines = []
    for output, tokens in sorted(group_by_output(mapping).items()):
        if key_split:
            patterns = "|".join(t.replace("\x00", "::") for t in sorted(tokens))
        else:
            patterns = "|".join(sorted(tokens))
        lines.append(f'        {patterns}) echo "{output}" ;;')
    return "\n".join(lines)


def main() -> None:
    device_types, method_tokens, params, values = collect()

    script = f"""\
#!/usr/bin/env bash
# homelink bash completion — generated by scripts/gen_bash_completion.py. Do not edit by hand;
# regenerate after changing device definitions in pkg/yolink/devices/.

__homelink_commands="{' '.join(COMMANDS)}"
__homelink_global_flags="{' '.join(GLOBAL_FLAGS)}"
__homelink_log_levels="{' '.join(LOG_LEVELS)}"
__homelink_output_formats="{' '.join(OUTPUT_FORMATS)}"
__homelink_methods="{' '.join(method_tokens)}"

# Param flags for a `do` method token (qualified "Type.method" or bare "method").
__homelink_method_params() {{
    case "$1" in
{render_case(params, key_split=False)}
        *) echo "" ;;
    esac
}}

# Enum values for a `do` method token and param flag, keyed "token::--flag".
__homelink_param_values() {{
    case "$1::$2" in
{render_case(values, key_split=True)}
        *) echo "" ;;
    esac
}}

_homelink() {{
    local cur prev
    cur="${{COMP_WORDS[COMP_CWORD]}}"
    prev="${{COMP_WORDS[COMP_CWORD-1]}}"

    # Values of global flags, wherever they appear.
    case "$prev" in
        --log)
            COMPREPLY=( $(compgen -W "$__homelink_log_levels" -- "$cur") )
            return ;;
        --output)
            COMPREPLY=( $(compgen -W "$__homelink_output_formats" -- "$cur") )
            return ;;
        --client-id|--client-secret)
            return ;;  # free-form secret; nothing to suggest
    esac

    # Locate the subcommand: the first recognised command word.
    local i cmd="" cmd_idx=-1
    for (( i=1; i < COMP_CWORD; i++ )); do
        case "${{COMP_WORDS[i]}}" in
            ls|listen|do|help) cmd="${{COMP_WORDS[i]}}"; cmd_idx=$i; break ;;
        esac
    done

    # Before the subcommand: complete the global flag or the command list.
    if [[ -z "$cmd" ]]; then
        if [[ "$cur" == -* ]]; then
            COMPREPLY=( $(compgen -W "$__homelink_global_flags" -- "$cur") )
        else
            COMPREPLY=( $(compgen -W "$__homelink_commands" -- "$cur") )
        fi
        return
    fi

    case "$cmd" in
        listen)
            [[ "$prev" == "--on" ]] && return  # device name is a runtime value; nothing to offer
            COMPREPLY=( $(compgen -W "--on" -- "$cur") )
            ;;
        do)
            local method_idx=$((cmd_idx + 1))
            # The method is the first word after `do`; complete names and type prefixes there.
            if (( COMP_CWORD == method_idx )); then
                COMPREPLY=( $(compgen -W "$__homelink_methods" -- "$cur") )
                return
            fi
            [[ "$prev" == "--on" ]] && return  # device name is a runtime value

            local method="${{COMP_WORDS[method_idx]}}"
            # After a param flag, offer its enum values when the API documents them.
            if [[ "$prev" == --* && "$prev" != "--on" ]]; then
                local choices
                choices="$(__homelink_param_values "$method" "$prev")"
                if [[ -n "$choices" ]]; then
                    COMPREPLY=( $(compgen -W "$choices" -- "$cur") )
                    return
                fi
            fi
            # Otherwise offer --on and this method's param flags.
            COMPREPLY=( $(compgen -W "--on $(__homelink_method_params "$method")" -- "$cur") )
            ;;
    esac
}}

complete -F _homelink homelink
"""
    sys.stdout.write(script)
    print(
        f"generated completion: {len(COMMANDS)} commands, {len(device_types)} device types, "
        f"{len(method_tokens)} method tokens",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
