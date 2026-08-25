from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

from sqlalchemy import desc, select
from sqlalchemy.orm import Session

from .models import AgentCommand, Device, DeviceUpdateState, UpdatePolicy


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


def as_utc(value: datetime | None) -> datetime | None:
    if value is None:
        return None
    if value.tzinfo is None:
        return value.replace(tzinfo=timezone.utc)
    return value.astimezone(timezone.utc)


def json_object(raw: str | None) -> dict:
    if not raw:
        return {}
    try:
        value = json.loads(raw)
        return value if isinstance(value, dict) else {}
    except (TypeError, ValueError):
        return {}


def json_list(raw: str | None) -> list:
    if not raw:
        return []
    try:
        value = json.loads(raw)
        return value if isinstance(value, list) else []
    except (TypeError, ValueError):
        return []


def get_update_state(db: Session, device_id: int, *, create: bool = False) -> DeviceUpdateState | None:
    state = db.get(DeviceUpdateState, device_id)
    if state is None and create:
        state = DeviceUpdateState(device_id=device_id)
        db.add(state)
        db.flush()
    return state


def active_update_command(db: Session, device_id: int) -> AgentCommand | None:
    return db.scalar(
        select(AgentCommand)
        .where(
            AgentCommand.device_id == device_id,
            AgentCommand.command_type.in_(["updates.scan", "updates.install"]),
            AgentCommand.status.in_(["queued", "running"]),
        )
        .order_by(desc(AgentCommand.created_at))
        .limit(1)
    )


def queue_agent_command(
    db: Session,
    device: Device,
    command_type: str,
    payload: dict | None = None,
    *,
    created_by: int | None = None,
    deduplicate: bool = True,
) -> tuple[AgentCommand, bool]:
    if deduplicate:
        existing = db.scalar(
            select(AgentCommand)
            .where(
                AgentCommand.device_id == device.id,
                AgentCommand.command_type == command_type,
                AgentCommand.status.in_(["queued", "running"]),
            )
            .order_by(desc(AgentCommand.created_at))
            .limit(1)
        )
        if existing:
            return existing, False

    command = AgentCommand(
        device_id=device.id,
        company_id=device.company_id,
        created_by=created_by,
        command_type=command_type,
        payload_json=json.dumps(payload or {}, ensure_ascii=False),
        status="queued",
    )
    db.add(command)
    db.flush()
    return command, True


def update_items(state: DeviceUpdateState | None) -> list[dict]:
    return json_list(state.inventory_json if state else None)


def build_install_payload(state: DeviceUpdateState, item_keys: list[str] | None = None) -> dict:
    inventory = update_items(state)
    selected_keys = set(item_keys or [str(item.get("key") or "") for item in inventory])
    selected = [item for item in inventory if str(item.get("key") or "") in selected_keys]
    if item_keys and len(selected) != len(selected_keys):
        known = {str(item.get("key") or "") for item in selected}
        missing = sorted(selected_keys - known)
        raise ValueError("Atualizações não pertencem ao inventário atual: " + ", ".join(missing[:5]))

    windows_ids: list[str] = []
    driver_ids: list[str] = []
    app_ids: list[str] = []
    for item in selected:
        source = str(item.get("source") or "").lower()
        item_id = str(item.get("id") or "").strip()
        if not item_id:
            continue
        if source == "windows":
            windows_ids.append(item_id)
        elif source == "driver":
            driver_ids.append(item_id)
        elif source == "app":
            app_ids.append(item_id)
    return {
        "windows_ids": sorted(set(windows_ids)),
        "driver_ids": sorted(set(driver_ids)),
        "app_ids": sorted(set(app_ids)),
    }


def apply_scan_result(db: Session, device: Device, result: dict) -> DeviceUpdateState:
    now = utcnow()
    state = get_update_state(db, device.id, create=True)
    items: list[dict] = []
    for source in ("windows", "drivers", "apps"):
        values = result.get(source)
        if not isinstance(values, list):
            continue
        for raw in values:
            if not isinstance(raw, dict):
                continue
            item = dict(raw)
            normalized_source = "driver" if source == "drivers" else "app" if source == "apps" else "windows"
            item["source"] = normalized_source
            item_id = str(item.get("id") or "").strip()
            if not item_id:
                continue
            item["key"] = f"{normalized_source}:{item_id}"
            items.append(item)

    state.last_scan_at = now
    state.windows_pending = sum(1 for item in items if item.get("source") == "windows")
    state.driver_pending = sum(1 for item in items if item.get("source") == "driver")
    state.app_pending = sum(1 for item in items if item.get("source") == "app")
    state.critical_pending = sum(
        1 for item in items if str(item.get("severity") or "").strip().lower() == "critical"
    )
    state.reboot_required = bool(result.get("reboot_required"))
    state.inventory_json = json.dumps(items, ensure_ascii=False)
    warnings = result.get("warnings") if isinstance(result.get("warnings"), list) else []
    state.last_error = "; ".join(str(value) for value in warnings if value)[:2000] or None
    state.updated_at = now
    return state


def allowed_now(policy: UpdatePolicy, now: datetime) -> bool:
    try:
        zone = ZoneInfo(policy.timezone or "UTC")
    except ZoneInfoNotFoundError:
        zone = timezone.utc
    local = now.astimezone(zone)
    try:
        days = {int(part) for part in (policy.allowed_days or "0,1,2,3,4,5,6").split(",") if part.strip()}
    except ValueError:
        days = set(range(7))
    if local.weekday() not in days:
        return False
    start = max(0, min(23, int(policy.start_hour)))
    end = max(0, min(23, int(policy.end_hour)))
    if start == end:
        return True
    if start < end:
        return start <= local.hour < end
    return local.hour >= start or local.hour < end




def _agent_version_supported(value: str | None) -> bool:
    parts = []
    for piece in str(value or "").strip().lstrip("vV").split(".")[:3]:
        digits = "".join(character for character in piece if character.isdigit())
        parts.append(int(digits or 0))
    version = tuple((parts + [0, 0, 0])[:3])
    return version >= (0, 5, 0)

def maybe_enqueue_update_policy(db: Session, device: Device, now: datetime | None = None) -> None:
    if not _agent_version_supported(device.agent_version):
        return
    now = now or utcnow()
    policies = list(
        db.scalars(
            select(UpdatePolicy)
            .where(UpdatePolicy.company_id == device.company_id, UpdatePolicy.active.is_(True))
            .order_by(UpdatePolicy.id)
        ).all()
    )
    if not policies or active_update_command(db, device.id):
        return

    state = get_update_state(db, device.id)
    inventory = update_items(state)
    for policy in policies:
        if not allowed_now(policy, now):
            continue
        last_action = as_utc(policy.last_auto_action_at)

        scan_age = None
        if state and state.last_scan_at:
            scan_age = now - (as_utc(state.last_scan_at) or now)

        last_install = as_utc(state.last_install_at) if state else None
        install_on_cooldown = bool(last_install and now - last_install < timedelta(hours=6))
        if policy.auto_install and not install_on_cooldown and state and inventory and scan_age is not None and scan_age <= timedelta(hours=24):
            keys: list[str] = []
            for item in inventory:
                source = str(item.get("source") or "")
                if source == "windows" and policy.include_windows:
                    keys.append(str(item.get("key")))
                elif source == "driver" and policy.include_drivers:
                    keys.append(str(item.get("key")))
                elif source == "app" and policy.include_apps:
                    keys.append(str(item.get("key")))
            if keys:
                payload = build_install_payload(state, keys)
                payload["policy_id"] = policy.id
                queue_agent_command(db, device, "updates.install", payload, created_by=policy.created_by)
                policy.last_auto_action_at = now
                return

        interval = max(1, min(168, int(policy.scan_interval_hours or 24)))
        scan_on_cooldown = bool(last_action and now - last_action < timedelta(minutes=45))
        if policy.auto_scan and not scan_on_cooldown and (scan_age is None or scan_age >= timedelta(hours=interval)):
            queue_agent_command(
                db,
                device,
                "updates.scan",
                {"policy_id": policy.id},
                created_by=policy.created_by,
            )
            policy.last_auto_action_at = now
            return
