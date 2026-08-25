"""HTTP sidecar wrapping python-kasa for the homelink kasa actuator.

homelink (Go) has no mature local-protocol client for current-firmware Kasa
devices, most of which require KLAP. This agent is a thin HTTP front end
onto python-kasa, which does implement KLAP.

Devices are addressed by name only -- homelink's config never contains an
IP. This agent owns discovery: on startup, and periodically thereafter, it
scans KASA_SCAN_SUBNET for devices listening on any of KASA_SCAN_PORTS
(legacy devices use 9999; current-firmware KLAP devices like the HS300 and
KS225/KS205 use 80 instead and never open 9999 at all) and builds a name ->
host cache from each device's own self-reported alias (the name set in the
Kasa app). A lookup that misses the cache triggers a bounded number of
on-demand rescans before failing, so a device that just came online or was
just renamed is still found, but a genuinely wrong name fails promptly
instead of hanging.
"""

import asyncio
import ipaddress
import logging
import os
import sys
import time
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from kasa import Discover

logging.basicConfig(level=os.environ.get("KASA_AGENT_LOG_LEVEL", "INFO"))
logger = logging.getLogger("kasa-agent")


def _require_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        sys.exit(f"kasa-agent: required environment variable {name} is not set")
    return value


def _auth_env() -> dict:
    """Optional TP-Link cloud credentials, for devices actually linked to a
    cloud account -- most Kasa devices work with no credentials at all (see
    the module docstring), but a device that has been linked needs the real
    account email/password. Passing these to every device, linked or not,
    is safe and doesn't change behavior for the rest of the fleet: the KLAP
    transport already tries the given credentials, then TP-Link's hardcoded
    setup defaults, then blank, in that order, so an unlinked device that
    already worked with no credentials keeps working identically.
    """
    username = os.environ.get("KASA_USERNAME")
    password = os.environ.get("KASA_PASSWORD")
    if bool(username) != bool(password):
        sys.exit("kasa-agent: KASA_USERNAME and KASA_PASSWORD must both be set, or neither")
    if not username:
        return {}
    return {"username": username, "password": password}


SCAN_SUBNET = _require_env("KASA_SCAN_SUBNET")  # e.g. "192.168.1.0/24"
# Legacy devices listen on 9999; current-firmware KLAP devices (e.g. HS300,
# KS225/KS205) instead expose their control API on port 80 and never open
# 9999 at all. A host is a scan candidate if *any* of these ports is open --
# Discover.discover_single then does the real, protocol-appropriate
# identification itself regardless of which port got it noticed.
SCAN_PORTS = [int(p) for p in os.environ.get("KASA_SCAN_PORTS", "9999,80").split(",") if p.strip()]
SCAN_CONCURRENCY = int(os.environ.get("KASA_SCAN_CONCURRENCY", "64"))
SCAN_CONNECT_TIMEOUT_SECONDS = float(os.environ.get("KASA_SCAN_CONNECT_TIMEOUT_SECONDS", "0.75"))
DISCOVER_TIMEOUT_SECONDS = float(os.environ.get("KASA_DISCOVER_TIMEOUT_SECONDS", "5"))
REFRESH_INTERVAL_SECONDS = float(os.environ.get("KASA_SCAN_INTERVAL_SECONDS", "300"))
LOOKUP_MAX_RETRIES = int(os.environ.get("KASA_LOOKUP_MAX_RETRIES", "2"))
LOOKUP_RETRY_DELAY_SECONDS = float(os.environ.get("KASA_LOOKUP_RETRY_DELAY_SECONDS", "2"))
AUTH_KWARGS = _auth_env()

app = FastAPI()


class DeviceCache:
    """Name -> host cache, built by scanning the subnet. Concurrent callers
    that need a rescan share one in-flight scan (single-flight) rather than
    each kicking off a redundant subnet sweep.
    """

    def __init__(self) -> None:
        self._by_alias: dict[str, str] = {}
        self._scan_task: Optional[asyncio.Task] = None
        self.last_scan_at: Optional[float] = None

    def get(self, alias: str) -> Optional[str]:
        return self._by_alias.get(alias)

    def snapshot(self) -> dict[str, str]:
        return dict(self._by_alias)

    async def rescan(self) -> None:
        if self._scan_task is None or self._scan_task.done():
            self._scan_task = asyncio.create_task(self._do_scan())
        await self._scan_task

    async def _do_scan(self) -> None:
        hosts = await _scan_subnet()
        results = await asyncio.gather(*(_identify(host) for host in hosts))

        by_alias: dict[str, str] = {}
        for result in results:
            if result is None:
                continue
            alias, host = result
            if alias in by_alias:
                logger.warning("kasa: duplicate device alias %r at %s and %s; keeping %s", alias, by_alias[alias], host, host)
            by_alias[alias] = host

        self._by_alias = by_alias
        self.last_scan_at = time.time()
        logger.info("kasa: scan of %s complete, %d device(s) found", SCAN_SUBNET, len(by_alias))


cache = DeviceCache()


async def _scan_subnet() -> list[str]:
    """Concurrently probes every host in SCAN_SUBNET on each of SCAN_PORTS.
    This is a plain TCP connect, not the Kasa protocol itself -- it's just
    how we narrow the subnet down to hosts worth identifying. A host is a
    candidate if any configured port is open.
    """
    hosts = [str(host) for host in ipaddress.ip_network(SCAN_SUBNET).hosts()]
    semaphore = asyncio.Semaphore(SCAN_CONCURRENCY)

    async def probe(ip: str) -> Optional[str]:
        async with semaphore:
            open_ports = await asyncio.gather(*(_port_open(ip, port) for port in SCAN_PORTS))
            return ip if any(open_ports) else None

    results = await asyncio.gather(*(probe(ip) for ip in hosts))
    return [ip for ip in results if ip is not None]


async def _port_open(ip: str, port: int) -> bool:
    try:
        _, writer = await asyncio.wait_for(
            asyncio.open_connection(ip, port), timeout=SCAN_CONNECT_TIMEOUT_SECONDS
        )
    except Exception:
        return False
    writer.close()
    try:
        await writer.wait_closed()
    except Exception:
        pass
    return True


async def _identify(host: str) -> Optional[tuple[str, str]]:
    """Identifies a candidate host, or returns None if it isn't a Kasa
    device, isn't reachable, or fails authentication -- distinguishing why
    isn't needed here, only whether it belongs in the cache. Always
    disconnects, even on failure: Discover.discover_single can succeed (it
    only needs the unauthenticated discovery broadcast) while the following
    update() fails authentication, e.g. a device actually linked to a TP-Link
    cloud account that our blank/default credentials can't satisfy -- left
    unclosed, that leaks an aiohttp session every scan.
    """
    device = None
    try:
        device = await Discover.discover_single(host, timeout=DISCOVER_TIMEOUT_SECONDS, **AUTH_KWARGS)
        await device.update()
        return device.alias, host
    except Exception as exc:
        logger.debug("kasa: could not identify device at %s: %s", host, exc)
        return None
    finally:
        if device is not None:
            await _disconnect(device)


async def resolve_with_retry(alias: str) -> str:
    host = cache.get(alias)
    if host is not None:
        return host

    for attempt in range(1, LOOKUP_MAX_RETRIES + 1):
        logger.info("kasa: %r not in cache, rescanning (attempt %d/%d)", alias, attempt, LOOKUP_MAX_RETRIES)
        await cache.rescan()
        host = cache.get(alias)
        if host is not None:
            return host
        if attempt < LOOKUP_MAX_RETRIES:
            await asyncio.sleep(LOOKUP_RETRY_DELAY_SECONDS)

    known = sorted(cache.snapshot().keys())
    raise HTTPException(
        status_code=404,
        detail=f"kasa device named {alias!r} not found in {SCAN_SUBNET} after {LOOKUP_MAX_RETRIES} scan attempt(s); "
        f"known devices: {known}",
    )


async def _refresh_loop() -> None:
    while True:
        await asyncio.sleep(REFRESH_INTERVAL_SECONDS)
        try:
            await cache.rescan()
        except Exception:
            logger.exception("kasa: periodic rescan failed")


@app.on_event("startup")
async def startup() -> None:
    await cache.rescan()
    asyncio.create_task(_refresh_loop())


@app.get("/healthz")
async def healthz():
    return {"status": "ok", "known_devices": len(cache.snapshot()), "last_scan_at": cache.last_scan_at}


@app.get("/devices")
async def list_devices():
    """Introspection endpoint: what the cache currently believes, for
    debugging a name that won't resolve.
    """
    return {"subnet": SCAN_SUBNET, "last_scan_at": cache.last_scan_at, "devices": cache.snapshot()}


class ActionRequest(BaseModel):
    action: str
    brightness: Optional[int] = None


@app.post("/devices/{alias}/action")
async def perform_action(alias: str, request: ActionRequest):
    host = await resolve_with_retry(alias)

    device = None
    try:
        device = await Discover.discover_single(host, timeout=DISCOVER_TIMEOUT_SECONDS, **AUTH_KWARGS)
        await device.update()
    except Exception as exc:
        if device is not None:
            await _disconnect(device)
        raise HTTPException(status_code=502, detail=f"could not reach kasa device {alias!r} at {host}: {exc}") from exc

    try:
        await _perform(device, alias, request)
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(status_code=502, detail=f"kasa action failed: {exc}") from exc
    finally:
        await _disconnect(device)

    return {"status": "ok"}


async def _disconnect(device) -> None:
    try:
        await device.disconnect()
    except Exception:
        pass


async def _perform(device, alias: str, request: ActionRequest) -> None:
    if request.action == "turn_on":
        await device.turn_on()
    elif request.action == "turn_off":
        await device.turn_off()
    elif request.action == "toggle":
        if device.is_on:
            await device.turn_off()
        else:
            await device.turn_on()
    elif request.action == "set_brightness":
        if request.brightness is None:
            raise HTTPException(status_code=400, detail="set_brightness requires brightness")
        if "brightness" not in device.features:
            raise HTTPException(status_code=422, detail=f"device {alias!r} does not support brightness")
        await device.features["brightness"].set_value(request.brightness)
    elif request.action == "led_on":
        if "led" not in device.features:
            raise HTTPException(status_code=422, detail=f"device {alias!r} does not support led control")
        await device.features["led"].set_value(True)
    elif request.action == "led_off":
        if "led" not in device.features:
            raise HTTPException(status_code=422, detail=f"device {alias!r} does not support led control")
        await device.features["led"].set_value(False)
    else:
        raise HTTPException(status_code=400, detail=f"unknown action {request.action!r}")
