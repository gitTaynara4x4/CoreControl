from __future__ import annotations

import hashlib
import io
import json
import re
import secrets
from pathlib import Path
from datetime import datetime, timedelta, timezone
from typing import Annotated

from fastapi import APIRouter, Depends, Header, HTTPException, Request, Response, status
from fastapi.responses import FileResponse
from sqlalchemy import delete, desc, func, or_, select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from .alerts import evaluate_telemetry_alerts
from .config import settings
from .db import get_db
from .models import AgentCommand, Alert, AuditLog, Company, Device, DeviceUpdateState, EnrollmentToken, PasswordResetToken, Telemetry, UpdatePolicy, User
from .meshcentral import (
    MeshCentralCommandError,
    MeshCentralTokenError,
    build_remote_desktop_url,
    create_login_token,
    meshcentral_client,
)
from .schemas import (
    CompanyCreate,
    CompanyDestroyRequest,
    CompanyRegistrationRequest,
    CompanyUpdate,
    DeviceInstallRequest,
    DeviceUpdate,
    EnrollmentRequest,
    LoginRequest,
    TelemetryRequest,
    UserCreate,
    UserUpdate,
)
from .update_service import maybe_enqueue_update_policy
from .security import (
    create_session_token,
    get_session_payload,
    hash_password,
    new_secret,
    sha256_text,
    verify_password,
)

router = APIRouter(prefix="/api")
Db = Annotated[Session, Depends(get_db)]


COMPONENT_DIR = Path(__file__).resolve().parent / "downloads"
DESKTOP_COMPONENTS = {
    "CoreControl.exe": "application/vnd.microsoft.portable-executable",
    "CoreControlAgent.exe": "application/vnd.microsoft.portable-executable",
    # Aliases legados mantidos para instaladores CoreTuner já distribuídos.
    "CoreTuner.exe": "application/vnd.microsoft.portable-executable",
    "CoreTunerAgent.exe": "application/vnd.microsoft.portable-executable",
}


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


def as_utc(value: datetime | None) -> datetime | None:
    if value is None:
        return None
    if value.tzinfo is None:
        return value.replace(tzinfo=timezone.utc)
    return value.astimezone(timezone.utc)


def iso(value: datetime | None) -> str | None:
    normalized = as_utc(value)
    return normalized.isoformat() if normalized else None


def slugify(value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return slug or "empresa"


def current_user(request: Request, db: Db) -> User:
    payload = get_session_payload(request)
    user = db.get(User, int(payload["sub"]))
    if not user or not user.active:
        raise HTTPException(status_code=401, detail="Usuário inválido")
    return user


CurrentUser = Annotated[User, Depends(current_user)]


def is_global_admin(user: User) -> bool:
    return user.role in {"global_admin", "platform_admin"}


def require_roles(user: User, *roles: str) -> None:
    # O Administrador Global herda todas as permissões administrativas da plataforma.
    if user.role == "global_admin":
        return
    if user.role not in roles:
        raise HTTPException(status_code=403, detail="Acesso não autorizado")


def assert_company_access(user: User, company_id: int) -> None:
    if is_global_admin(user):
        return
    if user.company_id != company_id:
        raise HTTPException(status_code=403, detail="Empresa não permitida")


def assert_device_access(user: User, device: Device) -> None:
    assert_company_access(user, device.company_id)


def serialize_company(company: Company, online: int = 0, total: int = 0, alerts: int = 0) -> dict:
    return {
        "id": company.id,
        "name": company.name,
        "slug": company.slug,
        "active": company.active,
        "created_at": iso(company.created_at),
        "devices_total": total,
        "devices_online": online,
        "alerts_open": alerts,
    }


def latest_telemetry(db: Session, device_id: int) -> Telemetry | None:
    return db.scalar(
        select(Telemetry).where(Telemetry.device_id == device_id).order_by(desc(Telemetry.recorded_at)).limit(1)
    )


def device_online(device: Device) -> bool:
    last_seen = as_utc(device.last_seen)
    return bool(last_seen and (utcnow() - last_seen).total_seconds() <= settings.offline_after_seconds)


def health_score(sample: Telemetry | None, online: bool) -> int:
    if not online:
        return 0
    if sample is None:
        return 50
    score = 100
    if sample.cpu_percent is not None:
        score -= max(0, int((sample.cpu_percent - 70) * 0.45))
    if sample.memory_percent is not None:
        score -= max(0, int((sample.memory_percent - 70) * 0.75))
    if sample.disk_percent is not None:
        score -= max(0, int((sample.disk_percent - 75) * 0.85))
    if sample.temperature_c is not None:
        score -= max(0, int((sample.temperature_c - 70) * 1.2))
    if sample.defender_active is False:
        score -= 18
    if sample.firewall_active is False:
        score -= 18
    return max(0, min(100, score))


def sample_extra(sample: Telemetry | None) -> dict:
    if not sample or not sample.raw_json:
        return {}
    try:
        value = json.loads(sample.raw_json)
        return value if isinstance(value, dict) else {}
    except (TypeError, ValueError):
        return {}


def remote_state(device: Device, sample: Telemetry | None) -> dict:
    extra = sample_extra(sample)
    installed = bool(extra.get("remote_agent_installed"))
    running = bool(extra.get("remote_agent_running"))
    checked_at = as_utc(device.remote_checked_at)
    verified_recently = bool(
        checked_at
        and (utcnow() - checked_at).total_seconds() <= settings.remote_status_stale_seconds
    )
    mesh_connected = bool(device.mesh_node_id and device.remote_online and verified_recently)
    enabled = bool(settings.remote_enabled and settings.remote_url)
    return {
        "enabled": enabled,
        "installed": installed,
        "running": running,
        "mesh_connected": mesh_connected,
        "mesh_node_id": device.mesh_node_id,
        "checked_at": iso(device.remote_checked_at),
        "last_seen": iso(device.remote_last_seen),
        "available": bool(enabled and running and mesh_connected),
        "service_name": extra.get("remote_service_name"),
    }


def serialize_sample(sample: Telemetry | None) -> dict | None:
    if not sample:
        return None
    extra = sample_extra(sample)
    activity = extra.get("activity") if isinstance(extra.get("activity"), dict) else None
    return {
        "recorded_at": iso(sample.recorded_at),
        "cpu_percent": sample.cpu_percent,
        "memory_percent": sample.memory_percent,
        "memory_used_gb": sample.memory_used_gb,
        "memory_total_gb": sample.memory_total_gb,
        "disk_percent": sample.disk_percent,
        "disk_free_gb": sample.disk_free_gb,
        "disk_total_gb": sample.disk_total_gb,
        "temperature_c": sample.temperature_c,
        "temperature_source": extra.get("temperature_source"),
        "gpu_name": extra.get("gpu_name"),
        "gpu_temperature_c": extra.get("gpu_temperature_c"),
        "gpu_usage_percent": extra.get("gpu_usage_percent"),
        "gpu_memory_used_mb": extra.get("gpu_memory_used_mb"),
        "gpu_memory_total_mb": extra.get("gpu_memory_total_mb"),
        "gpu_driver_version": extra.get("gpu_driver_version"),
        "uptime_seconds": sample.uptime_seconds,
        "ip_local": sample.ip_local,
        "network_name": sample.network_name,
        "defender_active": sample.defender_active,
        "firewall_active": sample.firewall_active,
        "remote_agent_installed": bool(extra.get("remote_agent_installed")),
        "remote_agent_running": bool(extra.get("remote_agent_running")),
        "remote_service_name": extra.get("remote_service_name"),
        "activity": activity,
    }


def serialize_device(db: Session, device: Device, include_sample: bool = True) -> dict:
    online = device_online(device)
    sample = latest_telemetry(db, device.id) if include_sample else None
    open_alerts = db.scalar(
        select(func.count(Alert.id)).where(Alert.device_id == device.id, Alert.status.in_(["open", "acknowledged"]))
    ) or 0
    return {
        "id": device.id,
        "company_id": device.company_id,
        "company_name": device.company.name if device.company else None,
        "device_uid": device.device_uid,
        "name": device.name,
        "hostname": device.hostname,
        "sector": device.sector,
        "location": device.location,
        "manufacturer": device.manufacturer,
        "model": device.model,
        "serial_number": device.serial_number,
        "os_name": device.os_name,
        "os_version": device.os_version,
        "agent_version": device.agent_version,
        "profile": device.profile,
        "active": device.active,
        "first_seen": iso(device.first_seen),
        "last_seen": iso(device.last_seen),
        "online": online,
        "health_score": health_score(sample, online),
        "alerts_open": int(open_alerts),
        "telemetry": serialize_sample(sample),
        "remote": remote_state(device, sample),
    }



def sync_company_remote_devices(
    db: Session,
    company: Company,
    *,
    force: bool = False,
) -> None:
    """Refresh exact MeshCentral connection state for all devices in one company."""
    if not meshcentral_client.provisioning_configured or not company.mesh_group_id:
        return
    remote_devices = meshcentral_client.list_group_devices(company.mesh_group_id, force=force)
    local_devices = list(
        db.scalars(
            select(Device).where(Device.company_id == company.id, Device.active.is_(True))
        ).all()
    )
    now = utcnow()
    for local in local_devices:
        local.remote_checked_at = now
        local.remote_online = False
        matched = meshcentral_client.match_device(local, remote_devices)
        if matched is None:
            # Um banco/grupo novo do MeshCentral gera IDs de nó novos. Não
            # mantenha o ID antigo no CoreControl, pois ele faria a Central abrir
            # uma sessão para um nó que já não existe nesse grupo.
            local.mesh_node_id = None
            continue
        local.mesh_node_id = matched.node_id
        local.remote_online = matched.connected
        if matched.connected:
            local.remote_last_seen = now
    db.commit()


def refresh_remote_for_devices(
    db: Session,
    devices: list[Device],
    *,
    force: bool = False,
    suppress_errors: bool = True,
) -> None:
    company_ids = sorted({device.company_id for device in devices})
    for company_id in company_ids:
        company = db.get(Company, company_id)
        if not company or not company.mesh_group_id:
            continue
        try:
            sync_company_remote_devices(db, company, force=force)
        except MeshCentralCommandError:
            if not suppress_errors:
                raise
            # A falha do serviço remoto não pode derrubar a telemetria/painel.
            continue


def prepare_remote_install(db: Session, company: Company, device: Device) -> tuple[dict | None, str | None]:
    if not settings.remote_enabled:
        return None, "O acesso remoto está desativado no servidor."
    if not meshcentral_client.provisioning_configured:
        return None, (
            "A automação remota não está completa. Configure CORETUNER_REMOTE_ADMIN_USER "
            "e reimplante o CoreControl com o MeshCtrl."
        )
    try:
        prepared = meshcentral_client.prepare_company_agent(company)
    except (MeshCentralCommandError, MeshCentralTokenError) as exc:
        return None, str(exc)
    company.mesh_group_id = prepared.mesh_group_id
    company.mesh_group_name = prepared.mesh_group_name
    company.mesh_group_synced_at = utcnow()
    db.commit()
    return (
        {
            "filename": prepared.filename,
            "url": f"/api/devices/{device.id}/remote-agent",
            "sha256": prepared.sha256,
            "size": prepared.size,
            "mesh_group_id": prepared.mesh_group_id,
            "mesh_group_hex": prepared.mesh_group_hex,
            "mesh_group_name": prepared.mesh_group_name,
            "server_url": prepared.server_url,
        },
        None,
    )

def sync_offline_alerts(db: Session, devices: list[Device]) -> None:
    now = utcnow()
    for device in devices:
        active = not device_online(device)
        current = db.scalar(
            select(Alert).where(
                Alert.device_id == device.id,
                Alert.alert_type == "device_offline",
                Alert.status.in_(["open", "acknowledged"]),
            )
        )
        if active and not current:
            last_seen = as_utc(device.last_seen)
            db.add(
                Alert(
                    company_id=device.company_id,
                    device_id=device.id,
                    alert_type="device_offline",
                    severity="critical",
                    title="Computador offline",
                    message=f"Sem comunicação desde {last_seen.strftime('%d/%m/%Y %H:%M:%S') if last_seen else 'horário desconhecido'}.",
                    status="open",
                    opened_at=now,
                    last_seen_at=now,
                )
            )
        elif active and current:
            current.last_seen_at = now
        elif not active and current:
            current.status = "resolved"
            current.resolved_at = now
            current.last_seen_at = now
    db.commit()


def unique_company_slug(db: Session, company_name: str) -> str:
    base = slugify(company_name)
    slug = base
    suffix = 2
    while db.scalar(select(Company.id).where(Company.slug == slug)):
        slug = f"{base}-{suffix}"
        suffix += 1
    return slug


def auth_payload(user: User, db: Session, token: str) -> dict:
    company = db.get(Company, user.company_id) if user.company_id else None
    return {
        "ok": True,
        "access_token": token,
        "token_type": "bearer",
        "expires_in_seconds": settings.token_minutes * 60,
        "user": {
            "id": user.id,
            "name": user.name,
            "email": user.email,
            "role": user.role,
            "company_id": user.company_id,
        },
        "company": serialize_company(company) if company else None,
    }


def set_session_cookie(response: Response, token: str) -> None:
    response.set_cookie(
        "coretuner_session",
        token,
        httponly=True,
        secure=settings.secure_cookies,
        samesite="strict",
        max_age=settings.token_minutes * 60,
        path="/",
    )


@router.get("/desktop/manifest")
def desktop_manifest(user: CurrentUser):
    files: dict[str, dict] = {}
    for filename in DESKTOP_COMPONENTS:
        path = COMPONENT_DIR / filename
        if not path.exists() or not path.is_file():
            raise HTTPException(status_code=503, detail=f"Componente indisponível: {filename}")
        files[filename] = {
            "filename": filename,
            "url": f"/api/desktop/components/{filename}",
            "sha256": file_sha256(path),
            "size": path.stat().st_size,
        }
    return {"version": "0.4.14", "files": files}


@router.get("/desktop/components/{filename}")
def desktop_component(filename: str, user: CurrentUser):
    media_type = DESKTOP_COMPONENTS.get(filename)
    if not media_type:
        raise HTTPException(status_code=404, detail="Componente não encontrado")
    path = COMPONENT_DIR / filename
    if not path.exists() or not path.is_file():
        raise HTTPException(status_code=404, detail="Componente indisponível")
    return FileResponse(
        path,
        media_type=media_type,
        filename=filename,
        headers={"Cache-Control": "no-store, private", "X-Content-Type-Options": "nosniff"},
    )


@router.get("/devices/{device_id}/remote-agent")
def download_remote_agent(device_id: int, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin", "technician")
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    company = db.get(Company, device.company_id)
    if not company or not company.active:
        raise HTTPException(status_code=404, detail="Empresa não encontrada")
    if not meshcentral_client.provisioning_configured:
        raise HTTPException(status_code=503, detail="A automação do acesso remoto não está configurada")
    try:
        prepared = meshcentral_client.prepare_company_agent(company)
    except (MeshCentralCommandError, MeshCentralTokenError) as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    company.mesh_group_id = prepared.mesh_group_id
    company.mesh_group_name = prepared.mesh_group_name
    company.mesh_group_synced_at = utcnow()
    db.commit()
    return FileResponse(
        prepared.path,
        media_type="application/vnd.microsoft.portable-executable",
        filename=prepared.filename,
        headers={"Cache-Control": "no-store, private", "X-Content-Type-Options": "nosniff"},
    )


@router.post("/auth/register-company", status_code=201)
def register_company(payload: CompanyRegistrationRequest, response: Response, db: Db):
    email = payload.email.lower().strip()
    if db.scalar(select(User.id).where(func.lower(User.email) == email)):
        raise HTTPException(status_code=409, detail="Este e-mail já está cadastrado")

    company = Company(name=payload.company_name.strip(), slug=unique_company_slug(db, payload.company_name))
    db.add(company)
    db.flush()

    owner = User(
        company_id=company.id,
        name=payload.responsible_name.strip(),
        email=email,
        password_hash=hash_password(payload.password),
        role="company_admin",
        active=True,
    )
    db.add(owner)
    db.flush()
    db.add(
        AuditLog(
            company_id=company.id,
            actor_user_id=owner.id,
            action="company.self_register",
            details=json.dumps({"company": company.name, "email": owner.email}, ensure_ascii=False),
        )
    )
    try:
        db.commit()
    except IntegrityError as exc:
        db.rollback()
        raise HTTPException(status_code=409, detail="Este e-mail ou nome de empresa já está cadastrado") from exc

    token = create_session_token(owner.id, owner.role, owner.company_id)
    set_session_cookie(response, token)
    return auth_payload(owner, db, token)


@router.post("/auth/login")
def login(payload: LoginRequest, response: Response, db: Db):
    user = db.scalar(select(User).where(func.lower(User.email) == payload.email.lower().strip()))
    if not user or not user.active or not verify_password(payload.password, user.password_hash):
        raise HTTPException(status_code=401, detail="E-mail ou senha inválidos")
    token = create_session_token(user.id, user.role, user.company_id)
    set_session_cookie(response, token)
    db.add(AuditLog(company_id=user.company_id, actor_user_id=user.id, action="auth.login", details="Login realizado"))
    db.commit()
    return auth_payload(user, db, token)


@router.post("/auth/logout")
def logout(response: Response):
    response.delete_cookie("coretuner_session", path="/")
    return {"ok": True}


@router.get("/auth/me")
def me(user: CurrentUser, db: Db):
    company = db.get(Company, user.company_id) if user.company_id else None
    return {
        "id": user.id,
        "name": user.name,
        "email": user.email,
        "role": user.role,
        "company_id": user.company_id,
        "company": serialize_company(company) if company else None,
    }


def _dashboard_log_details(raw: str | None):
    if not raw:
        return None
    try:
        value = json.loads(raw)
        return value
    except (TypeError, ValueError):
        return raw


def _dashboard_company_operations(user: CurrentUser, db: Session, devices: list[Device], companies: list[Company]) -> dict | None:
    # O administrador global continua com a visão consolidada da plataforma.
    # A Central de Operação abaixo é específica para usuários vinculados a uma empresa.
    if is_global_admin(user) or not user.company_id:
        return None

    company = next((item for item in companies if item.id == user.company_id), None)
    device_ids = [device.id for device in devices]
    since = utcnow() - timedelta(hours=24)

    def audit_count(action: str) -> int:
        return int(
            db.scalar(
                select(func.count(AuditLog.id)).where(
                    AuditLog.company_id == user.company_id,
                    AuditLog.created_at >= since,
                    AuditLog.action == action,
                )
            )
            or 0
        )

    update_states = []
    if device_ids:
        update_states = list(
            db.scalars(
                select(DeviceUpdateState).where(DeviceUpdateState.device_id.in_(device_ids))
            ).all()
        )
    updates_pending = sum(
        max(0, int(state.windows_pending or 0))
        + max(0, int(state.driver_pending or 0))
        + max(0, int(state.app_pending or 0))
        for state in update_states
    )
    reboot_required = sum(1 for state in update_states if state.reboot_required)

    recent_filters = [
        AuditLog.company_id == user.company_id,
        or_(
            AuditLog.action.in_([
                "remote.session.request",
                "alert.acknowledge",
                "agent.enroll",
                "device.update",
            ]),
            AuditLog.action.like("optimization.%.success"),
            AuditLog.action.like("optimization.%.failed"),
            AuditLog.action.like("updates.install.%"),
        ),
    ]
    recent_logs = list(
        db.scalars(
            select(AuditLog)
            .where(*recent_filters)
            .order_by(desc(AuditLog.created_at))
            .limit(12)
        ).all()
    )

    device_names = {device.id: device.name for device in devices}
    actor_ids = sorted({log.actor_user_id for log in recent_logs if log.actor_user_id})
    actors = {}
    if actor_ids:
        actors = {
            item.id: item.name
            for item in db.scalars(select(User).where(User.id.in_(actor_ids))).all()
        }

    recent_events = [
        {
            "id": log.id,
            "action": log.action,
            "details": _dashboard_log_details(log.details),
            "created_at": iso(log.created_at),
            "device_id": log.device_id,
            "device_name": device_names.get(log.device_id),
            "actor_name": actors.get(log.actor_user_id),
        }
        for log in recent_logs
    ]

    return {
        "company_name": company.name if company else None,
        "last_24h": {
            "optimizations": audit_count("optimization.apply.success"),
            "diagnostics": audit_count("optimization.diagnose.success"),
            "cleanups": audit_count("optimization.cleanup_temp.success"),
            "remote_sessions": audit_count("remote.session.request"),
            "update_installs": audit_count("updates.install.success"),
        },
        "updates": {
            "pending": updates_pending,
            "reboot_required": reboot_required,
        },
        "recent_events": recent_events,
    }


@router.get("/dashboard/summary")
def dashboard_summary(user: CurrentUser, db: Db):
    company_filter = [] if is_global_admin(user) else [Device.company_id == user.company_id]
    devices = list(db.scalars(select(Device).where(Device.active.is_(True), *company_filter)).all())
    sync_offline_alerts(db, devices)
    companies_stmt = select(Company).where(Company.active.is_(True))
    if not is_global_admin(user):
        companies_stmt = companies_stmt.where(Company.id == user.company_id)
    companies = list(db.scalars(companies_stmt.order_by(Company.name)).all())
    online = sum(1 for device in devices if device_online(device))
    alert_stmt = select(func.count(Alert.id)).where(Alert.status.in_(["open", "acknowledged"]))
    if not is_global_admin(user):
        alert_stmt = alert_stmt.where(Alert.company_id == user.company_id)
    open_alerts = int(db.scalar(alert_stmt) or 0)
    return {
        "companies": len(companies),
        "devices": len(devices),
        "online": online,
        "offline": len(devices) - online,
        "alerts_open": open_alerts,
        "operations": _dashboard_company_operations(user, db, devices, companies),
    }


@router.get("/companies")
def list_companies(user: CurrentUser, db: Db):
    stmt = select(Company).order_by(Company.name)
    if not is_global_admin(user):
        stmt = stmt.where(Company.id == user.company_id, Company.active.is_(True))
    companies = list(db.scalars(stmt).all())
    result = []
    for company in companies:
        devices_stmt = select(Device).where(Device.company_id == company.id)
        if not is_global_admin(user):
            devices_stmt = devices_stmt.where(Device.active.is_(True))
        devices = list(db.scalars(devices_stmt).all())
        online = sum(1 for d in devices if d.active and device_online(d))
        alerts = db.scalar(
            select(func.count(Alert.id)).where(
                Alert.company_id == company.id, Alert.status.in_(["open", "acknowledged"])
            )
        ) or 0
        result.append(serialize_company(company, online, len(devices), int(alerts)))
    return result


@router.post("/companies", status_code=201)
def create_company(payload: CompanyCreate, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin")
    company = Company(name=payload.name.strip(), slug=unique_company_slug(db, payload.name))
    db.add(company)
    db.flush()
    db.add(
        AuditLog(
            company_id=company.id,
            actor_user_id=user.id,
            action="company.create",
            details=json.dumps({"name": company.name}, ensure_ascii=False),
        )
    )
    db.commit()
    return serialize_company(company)


@router.patch("/companies/{company_id}")
def update_company(company_id: int, payload: CompanyUpdate, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin")
    company = db.get(Company, company_id)
    if not company:
        raise HTTPException(status_code=404, detail="Empresa não encontrada")

    changes = payload.model_dump(exclude_unset=True)
    before = {"name": company.name, "active": company.active}
    if "name" in changes:
        company.name = changes["name"].strip()
    if "active" in changes:
        company.active = bool(changes["active"])

    db.add(
        AuditLog(
            company_id=company.id,
            actor_user_id=user.id,
            action="company.update",
            details=json.dumps({"before": before, "after": {"name": company.name, "active": company.active}}, ensure_ascii=False),
        )
    )
    db.commit()
    db.refresh(company)
    return serialize_company(company)


@router.delete("/companies/{company_id}")
def destroy_company(company_id: int, payload: CompanyDestroyRequest, user: CurrentUser, db: Db):
    if user.role != "global_admin":
        raise HTTPException(status_code=403, detail="Somente o Administrador Global pode excluir uma empresa definitivamente")

    company = db.get(Company, company_id)
    if not company:
        raise HTTPException(status_code=404, detail="Empresa não encontrada")

    expected = f"EXCLUIR {company.name}"
    confirmation = " ".join(payload.confirmation.strip().split())
    if confirmation.casefold() != expected.casefold():
        raise HTTPException(
            status_code=400,
            detail=f'Digite exatamente "{expected}" para confirmar a exclusão definitiva',
        )

    company_name = company.name
    mesh_group_id = company.mesh_group_id
    device_ids = list(db.scalars(select(Device.id).where(Device.company_id == company_id)).all())
    company_user_ids = list(db.scalars(select(User.id).where(User.company_id == company_id)).all())

    if user.id in company_user_ids:
        raise HTTPException(status_code=400, detail="O Administrador Global não pode excluir a empresa à qual está vinculado")

    counts = {
        "devices": len(device_ids),
        "users": len(company_user_ids),
        "alerts": int(db.scalar(select(func.count(Alert.id)).where(Alert.company_id == company_id)) or 0),
        "enrollment_tokens": int(
            db.scalar(select(func.count(EnrollmentToken.id)).where(EnrollmentToken.company_id == company_id)) or 0
        ),
    }

    remote_cleanup = {"attempted": False, "removed": False, "warning": None}
    if mesh_group_id:
        if meshcentral_client.provisioning_configured:
            remote_cleanup["attempted"] = True
            try:
                meshcentral_client.remove_company_group(mesh_group_id)
                remote_cleanup["removed"] = True
            except MeshCentralCommandError:
                remote_cleanup["warning"] = (
                    "A empresa foi removida do CoreControl, mas o grupo do acesso remoto não pôde ser excluído automaticamente."
                )
        else:
            remote_cleanup["warning"] = (
                "A empresa possuía um grupo de acesso remoto, mas a integração administrativa não está disponível para removê-lo."
            )

    # Apaga primeiro os registros que possuem chaves estrangeiras para usuários,
    # computadores ou empresa. Tudo ocorre na mesma transação: se qualquer etapa
    # falhar, nenhuma exclusão parcial é confirmada.
    audit_filters = [AuditLog.company_id == company_id]
    if device_ids:
        audit_filters.append(AuditLog.device_id.in_(device_ids))
    if company_user_ids:
        audit_filters.append(AuditLog.actor_user_id.in_(company_user_ids))
        db.execute(delete(PasswordResetToken).where(PasswordResetToken.user_id.in_(company_user_ids)))
    db.execute(delete(AuditLog).where(or_(*audit_filters)))
    db.execute(delete(EnrollmentToken).where(EnrollmentToken.company_id == company_id))
    db.execute(delete(Alert).where(Alert.company_id == company_id))
    db.execute(delete(UpdatePolicy).where(UpdatePolicy.company_id == company_id))
    if device_ids:
        db.execute(delete(AgentCommand).where(AgentCommand.device_id.in_(device_ids)))
        db.execute(delete(DeviceUpdateState).where(DeviceUpdateState.device_id.in_(device_ids)))
        db.execute(delete(Telemetry).where(Telemetry.device_id.in_(device_ids)))
    db.execute(delete(Device).where(Device.company_id == company_id))
    db.execute(delete(User).where(User.company_id == company_id))
    db.execute(delete(Company).where(Company.id == company_id))

    db.add(
        AuditLog(
            company_id=None,
            actor_user_id=user.id,
            action="company.destroy",
            details=json.dumps(
                {
                    "company_id": company_id,
                    "company_name": company_name,
                    "mesh_group_id": mesh_group_id,
                    "deleted": counts,
                    "remote_cleanup": remote_cleanup,
                },
                ensure_ascii=False,
            ),
        )
    )
    try:
        db.commit()
    except IntegrityError as exc:
        db.rollback()
        raise HTTPException(
            status_code=409,
            detail="A empresa possui vínculos que impediram a exclusão definitiva",
        ) from exc

    return {
        "ok": True,
        "company_id": company_id,
        "company_name": company_name,
        "deleted": counts,
        "remote_cleanup": remote_cleanup,
    }


@router.get("/companies/{company_id}")
def get_company(company_id: int, user: CurrentUser, db: Db):
    assert_company_access(user, company_id)
    company = db.get(Company, company_id)
    if not company:
        raise HTTPException(status_code=404, detail="Empresa não encontrada")
    devices_stmt = select(Device).where(Device.company_id == company_id)
    if not is_global_admin(user):
        devices_stmt = devices_stmt.where(Device.active.is_(True))
    devices = list(db.scalars(devices_stmt.order_by(Device.name)).all())
    active_devices = [device for device in devices if device.active]
    sync_offline_alerts(db, active_devices)
    users_total = int(db.scalar(select(func.count(User.id)).where(User.company_id == company_id)) or 0)
    return {
        **serialize_company(company),
        "users_total": users_total,
        "devices": [serialize_device(db, d) for d in devices],
    }


INSTALL_CODE_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
VALID_ENROLLMENT_MINUTES = {30, 120, 1440}


def normalize_install_code(value: str) -> str:
    compact = re.sub(r"[^A-Z0-9]", "", str(value or "").upper())
    if len(compact) == 8:
        compact = "CC" + compact
    if len(compact) != 10 or not compact.startswith("CC"):
        return ""
    body = compact[2:]
    if any(char not in INSTALL_CODE_ALPHABET for char in body):
        return ""
    return f"CC-{body[:4]}-{body[4:]}"


def new_install_code(db: Session) -> str:
    for _ in range(20):
        body = "".join(secrets.choice(INSTALL_CODE_ALPHABET) for _ in range(8))
        code = f"CC-{body[:4]}-{body[4:]}"
        exists = db.scalar(
            select(EnrollmentToken.id).where(EnrollmentToken.code_hash == sha256_text(code))
        )
        if not exists:
            return code
    raise HTTPException(status_code=503, detail="Não foi possível gerar um código de instalação agora")


def get_valid_enrollment(db: Session, credential: str) -> tuple[EnrollmentToken, Company]:
    install_code = normalize_install_code(credential)
    if install_code:
        enrollment = db.scalar(
            select(EnrollmentToken).where(EnrollmentToken.code_hash == sha256_text(install_code))
        )
    else:
        enrollment = db.scalar(
            select(EnrollmentToken).where(EnrollmentToken.token_hash == sha256_text(credential))
        )

    now = utcnow()
    if not enrollment or enrollment.used_at is not None:
        raise HTTPException(status_code=410, detail="Código ou link de instalação inválido ou já utilizado")
    expires_at = as_utc(enrollment.expires_at)
    if not expires_at or expires_at < now:
        raise HTTPException(status_code=410, detail="Código ou link de instalação expirado")
    company = db.get(Company, enrollment.company_id)
    if not company or not company.active:
        raise HTTPException(status_code=404, detail="Empresa não encontrada ou desativada")
    return enrollment, company


@router.post("/companies/{company_id}/enrollment-token")
def create_enrollment_token(
    company_id: int,
    user: CurrentUser,
    db: Db,
    valid_minutes: int = 30,
):
    assert_company_access(user, company_id)
    require_roles(user, "platform_admin", "company_admin", "technician")
    company = db.get(Company, company_id)
    if not company or not company.active:
        raise HTTPException(status_code=404, detail="Empresa não encontrada ou desativada")
    if valid_minutes not in VALID_ENROLLMENT_MINUTES:
        raise HTTPException(status_code=422, detail="Validade permitida: 30 minutos, 2 horas ou 24 horas")

    raw = f"ctenr_{new_secret(32)}"
    install_code = new_install_code(db)
    expires = utcnow() + timedelta(minutes=valid_minutes)
    db.add(
        EnrollmentToken(
            company_id=company_id,
            token_hash=sha256_text(raw),
            code_hash=sha256_text(install_code),
            expires_at=expires,
            created_by=user.id,
        )
    )
    db.add(
        AuditLog(
            company_id=company_id,
            actor_user_id=user.id,
            action="agent.enrollment_token.create",
            details=(
                f"Código/link de instalação de uso único válido por {valid_minutes} minutos "
                f"até {expires.isoformat()}"
            ),
        )
    )
    db.commit()
    base = settings.public_url.rstrip("/")
    return {
        "token": raw,
        "installation_code": install_code,
        "installation_url": f"{base}/instalar/{raw}",
        "install_page_url": f"{base}/instalar",
        "setup_url": f"{base}/instalar/setup",
        "code_download_url": f"{base}/instalar/codigo/{install_code}",
        "qr_url": f"{base}/api/enrollment/{raw}/qr.svg",
        "expires_at": expires.isoformat(),
        "valid_minutes": valid_minutes,
        "single_use": True,
    }


@router.get("/enrollment/{credential}/info")
def enrollment_info(credential: str, db: Db):
    enrollment, company = get_valid_enrollment(db, credential)
    return {
        "ok": True,
        "company_id": company.id,
        "company_name": company.name,
        "expires_at": iso(enrollment.expires_at),
        "single_use": True,
    }


@router.get("/enrollment/{raw_token}/qr.svg")
def enrollment_qr(raw_token: str, db: Db):
    get_valid_enrollment(db, raw_token)
    try:
        import qrcode
        import qrcode.image.svg
    except ImportError as exc:
        raise HTTPException(status_code=503, detail="Gerador de QR Code indisponível") from exc

    target = f"{settings.public_url.rstrip('/')}/instalar/{raw_token}"
    image = qrcode.make(target, image_factory=qrcode.image.svg.SvgPathImage)
    buffer = io.BytesIO()
    image.save(buffer)
    return Response(
        content=buffer.getvalue(),
        media_type="image/svg+xml",
        headers={
            "Cache-Control": "no-store, private",
            "X-Content-Type-Options": "nosniff",
            "Referrer-Policy": "no-referrer",
        },
    )


@router.get("/enrollment/{credential}/manifest")
def enrollment_manifest(credential: str, db: Db):
    get_valid_enrollment(db, credential)
    files: dict[str, dict] = {}
    for filename in ("CoreControl.exe", "CoreControlAgent.exe"):
        path = COMPONENT_DIR / filename
        if not path.exists() or not path.is_file():
            raise HTTPException(status_code=503, detail=f"Componente indisponível: {filename}")
        files[filename] = {
            "filename": filename,
            "url": f"/api/enrollment/{credential}/components/{filename}",
            "sha256": file_sha256(path),
            "size": path.stat().st_size,
        }
    return {"version": "0.4.15", "files": files}


@router.get("/enrollment/{credential}/components/{filename}")
def enrollment_component(credential: str, filename: str, db: Db):
    get_valid_enrollment(db, credential)
    if filename not in {"CoreControl.exe", "CoreControlAgent.exe"}:
        raise HTTPException(status_code=404, detail="Componente não encontrado")
    path = COMPONENT_DIR / filename
    if not path.exists() or not path.is_file():
        raise HTTPException(status_code=404, detail="Componente indisponível")
    return FileResponse(
        path,
        media_type="application/vnd.microsoft.portable-executable",
        filename=filename,
        headers={"Cache-Control": "no-store, private", "X-Content-Type-Options": "nosniff"},
    )


@router.get("/devices")
def list_devices(user: CurrentUser, db: Db, company_id: int | None = None):
    stmt = select(Device)
    if is_global_admin(user):
        if company_id is not None:
            stmt = stmt.where(Device.company_id == company_id)
    else:
        stmt = stmt.where(Device.company_id == user.company_id, Device.active.is_(True))
    devices = list(db.scalars(stmt.order_by(Device.name)).all())
    active_devices = [device for device in devices if device.active]
    sync_offline_alerts(db, active_devices)
    refresh_remote_for_devices(db, active_devices)
    return [serialize_device(db, d) for d in devices]


@router.get("/devices/{device_id}")
def get_device(device_id: int, user: CurrentUser, db: Db):
    device = db.get(Device, device_id)
    if not device:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    refresh_remote_for_devices(db, [device])
    result = serialize_device(db, device)
    samples = list(
        db.scalars(
            select(Telemetry).where(Telemetry.device_id == device.id).order_by(desc(Telemetry.recorded_at)).limit(60)
        ).all()
    )
    result["history"] = [serialize_sample(sample) for sample in reversed(samples)]
    result["company_name"] = db.get(Company, device.company_id).name
    return result


@router.patch("/devices/{device_id}")
def update_device(device_id: int, payload: DeviceUpdate, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    device = db.get(Device, device_id)
    if not device:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)

    changes = payload.model_dump(exclude_unset=True)
    before = {
        "company_id": device.company_id,
        "name": device.name,
        "hostname": device.hostname,
        "sector": device.sector,
        "location": device.location,
        "manufacturer": device.manufacturer,
        "model": device.model,
        "serial_number": device.serial_number,
        "profile": device.profile,
        "active": device.active,
    }

    if "company_id" in changes:
        if not is_global_admin(user):
            raise HTTPException(status_code=403, detail="Somente administradores globais podem mover computadores entre empresas")
        target_company_id = changes["company_id"]
        if target_company_id is None:
            raise HTTPException(status_code=400, detail="Selecione uma empresa")
        target_company = db.get(Company, target_company_id)
        if not target_company:
            raise HTTPException(status_code=404, detail="Empresa de destino não encontrada")
        device.company_id = target_company_id

    for field in ("name", "hostname", "sector", "location", "manufacturer", "model", "serial_number", "profile"):
        if field in changes:
            value = changes[field]
            if isinstance(value, str):
                value = value.strip() or None
            if field in {"name", "hostname"} and not value:
                raise HTTPException(status_code=400, detail=f"{field} não pode ficar vazio")
            setattr(device, field, value)
    if "active" in changes:
        device.active = bool(changes["active"])

    after = {key: getattr(device, key) for key in before}
    db.add(
        AuditLog(
            company_id=device.company_id,
            actor_user_id=user.id,
            device_id=device.id,
            action="device.update",
            details=json.dumps({"before": before, "after": after}, ensure_ascii=False),
        )
    )
    try:
        db.commit()
    except IntegrityError as exc:
        db.rollback()
        raise HTTPException(status_code=409, detail="Este computador já está vinculado à empresa selecionada") from exc
    db.refresh(device)
    return serialize_device(db, device)


@router.get("/devices/{device_id}/remote-status")
def get_remote_status(device_id: int, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin", "technician")
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    sync_error = None
    try:
        refresh_remote_for_devices(db, [device], force=True, suppress_errors=False)
    except MeshCentralCommandError as exc:
        sync_error = str(exc)
    state = remote_state(device, latest_telemetry(db, device.id))
    return {
        "ok": True,
        "device_id": device.id,
        "hostname": device.hostname,
        "mesh_connected": state["mesh_connected"],
        "mesh_node_id": device.mesh_node_id,
        "service_running": state["running"],
        "available": state["available"],
        "checked_at": state["checked_at"],
        "warning": sync_error,
    }


@router.post("/devices/{device_id}/remote-session")
def create_remote_session(device_id: int, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin", "technician")
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    if not settings.remote_enabled or not settings.remote_url:
        raise HTTPException(status_code=503, detail="Acesso remoto ainda não foi configurado no servidor")
    if not settings.remote_token_configured:
        raise HTTPException(
            status_code=503,
            detail="Login automático do acesso remoto ainda não foi configurado",
        )

    try:
        refresh_remote_for_devices(db, [device], force=True, suppress_errors=False)
    except MeshCentralCommandError as exc:
        raise HTTPException(status_code=503, detail=f"MeshCentral indisponível: {exc}") from exc
    state = remote_state(device, latest_telemetry(db, device.id))
    if not state["running"]:
        raise HTTPException(status_code=409, detail="O serviço Mesh Agent não está rodando neste computador")
    if not state["mesh_connected"] or not device.mesh_node_id:
        raise HTTPException(
            status_code=409,
            detail="O computador ainda não apareceu online no MeshCentral. Reinstale pelo CoreControl Setup atualizado.",
        )

    try:
        login_token = create_login_token(
            login_token_key=settings.remote_login_token_key,
            username=settings.remote_login_user,
            domain=settings.remote_login_domain,
            expire_minutes=settings.remote_login_token_minutes,
        )
        remote_url = build_remote_desktop_url(
            base_url=settings.remote_url,
            login_token=login_token,
            node_id=device.mesh_node_id,
        )
    except MeshCentralTokenError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc

    db.add(
        AuditLog(
            company_id=device.company_id,
            actor_user_id=user.id,
            device_id=device.id,
            action="remote.session.request",
            details=json.dumps(
                {
                    "hostname": device.hostname,
                    "mesh_node_id": device.mesh_node_id,
                    "remote_user": settings.remote_login_user,
                    "embedded": True,
                },
                ensure_ascii=False,
            ),
        )
    )
    db.commit()
    return {
        "ok": True,
        "device_id": device.id,
        "device_name": device.name,
        "hostname": device.hostname,
        "url": remote_url,
        "embedded": True,
        "expires_in_seconds": settings.remote_login_token_minutes * 60,
    }


@router.get("/alerts")
def list_alerts(user: CurrentUser, db: Db, status_filter: str = "active"):
    stmt = select(Alert).order_by(desc(Alert.opened_at))
    if not is_global_admin(user):
        stmt = stmt.where(Alert.company_id == user.company_id)
    if status_filter == "active":
        stmt = stmt.where(Alert.status.in_(["open", "acknowledged"]))
    elif status_filter in {"open", "acknowledged", "resolved"}:
        stmt = stmt.where(Alert.status == status_filter)
    alerts = list(db.scalars(stmt.limit(300)).all())
    return [
        {
            "id": alert.id,
            "company_id": alert.company_id,
            "device_id": alert.device_id,
            "device_name": alert.device.name,
            "type": alert.alert_type,
            "severity": alert.severity,
            "title": alert.title,
            "message": alert.message,
            "status": alert.status,
            "opened_at": iso(alert.opened_at),
            "last_seen_at": iso(alert.last_seen_at),
            "resolved_at": iso(alert.resolved_at),
        }
        for alert in alerts
    ]


@router.post("/alerts/{alert_id}/ack")
def acknowledge_alert(alert_id: int, user: CurrentUser, db: Db):
    alert = db.get(Alert, alert_id)
    if not alert:
        raise HTTPException(status_code=404, detail="Alerta não encontrado")
    assert_company_access(user, alert.company_id)
    require_roles(user, "platform_admin", "company_admin", "technician")
    if alert.status == "open":
        alert.status = "acknowledged"
        alert.acknowledged_at = utcnow()
        db.add(
            AuditLog(
                company_id=alert.company_id,
                actor_user_id=user.id,
                device_id=alert.device_id,
                action="alert.acknowledge",
                details=f"Alerta {alert.id}: {alert.title}",
            )
        )
        db.commit()
    return {"ok": True, "status": alert.status}


@router.get("/users")
def list_users(user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    stmt = select(User).order_by(User.name)
    if not is_global_admin(user):
        stmt = stmt.where(User.company_id == user.company_id)
    users = list(db.scalars(stmt).all())
    return [
        {
            "id": item.id,
            "name": item.name,
            "email": item.email,
            "role": item.role,
            "company_id": item.company_id,
            "active": item.active,
        }
        for item in users
    ]


@router.post("/users", status_code=201)
def create_user(payload: UserCreate, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    valid_roles = {"platform_admin", "company_admin", "technician", "viewer"}
    if payload.role not in valid_roles:
        raise HTTPException(status_code=400, detail="Perfil inválido")
    if not is_global_admin(user):
        if payload.company_id not in (None, user.company_id):
            raise HTTPException(status_code=403, detail="Empresa não permitida")
        if payload.role == "platform_admin":
            raise HTTPException(status_code=403, detail="Perfil não permitido")
        company_id = user.company_id
    else:
        company_id = None if payload.role == "platform_admin" else payload.company_id
        if payload.role != "platform_admin":
            if company_id is None:
                raise HTTPException(status_code=400, detail="Selecione a empresa do usuário")
            if not db.get(Company, company_id):
                raise HTTPException(status_code=404, detail="Empresa não encontrada")
    if db.scalar(select(User.id).where(func.lower(User.email) == payload.email.lower())):
        raise HTTPException(status_code=409, detail="E-mail já cadastrado")
    new_user = User(
        name=payload.name.strip(),
        email=payload.email.lower().strip(),
        password_hash=hash_password(payload.password),
        role=payload.role,
        company_id=company_id,
    )
    db.add(new_user)
    db.flush()
    db.add(
        AuditLog(
            company_id=company_id,
            actor_user_id=user.id,
            action="user.create",
            details=json.dumps({"email": new_user.email, "role": new_user.role}, ensure_ascii=False),
        )
    )
    db.commit()
    return {"id": new_user.id, "name": new_user.name, "email": new_user.email, "role": new_user.role}


@router.patch("/users/{user_id}")
def update_user(user_id: int, payload: UserUpdate, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    target = db.get(User, user_id)
    if not target:
        raise HTTPException(status_code=404, detail="Usuário não encontrado")

    if not is_global_admin(user) and target.company_id != user.company_id:
        raise HTTPException(status_code=403, detail="Usuário não pertence à sua empresa")

    changes = payload.model_dump(exclude_unset=True)
    valid_roles = {"global_admin", "platform_admin", "company_admin", "technician", "viewer"}
    before = {
        "name": target.name,
        "email": target.email,
        "role": target.role,
        "company_id": target.company_id,
        "active": target.active,
    }

    # A proteção da conta global é baseada no papel, não em um e-mail hardcoded.
    is_primary_global_admin = target.role == "global_admin"
    if is_primary_global_admin and user.id != target.id and user.role != "global_admin":
        raise HTTPException(status_code=403, detail="Somente o Administrador Global pode alterar esta conta")
    if is_primary_global_admin:
        if changes.get("active") is False:
            raise HTTPException(status_code=400, detail="O Administrador Global não pode ser bloqueado")
        if "role" in changes and changes["role"] != "global_admin":
            raise HTTPException(status_code=400, detail="O Administrador Global deve manter acesso global")
        if "company_id" in changes and changes["company_id"] is not None:
            raise HTTPException(status_code=400, detail="O Administrador Global não pode ser vinculado a uma empresa")
        if (
            "email" in changes
            and settings.global_admin_email
            and str(changes["email"]).lower().strip() != settings.global_admin_email
        ):
            raise HTTPException(status_code=400, detail="O e-mail do Administrador Global é definido na configuração do servidor")

    if target.id == user.id:
        if changes.get("active") is False:
            raise HTTPException(status_code=400, detail="Você não pode bloquear o próprio acesso")
        if "role" in changes and changes["role"] != target.role:
            raise HTTPException(status_code=400, detail="Você não pode alterar o próprio perfil")
        if "company_id" in changes and changes["company_id"] != target.company_id:
            raise HTTPException(status_code=400, detail="Você não pode alterar a própria empresa")

    if "name" in changes:
        target.name = changes["name"].strip()
    if "email" in changes:
        new_email = str(changes["email"]).lower().strip()
        duplicate = db.scalar(select(User.id).where(func.lower(User.email) == new_email, User.id != target.id))
        if duplicate:
            raise HTTPException(status_code=409, detail="E-mail já cadastrado")
        target.email = new_email
    if "password" in changes and changes["password"]:
        target.password_hash = hash_password(changes["password"])
    if "role" in changes:
        new_role = changes["role"]
        if new_role not in valid_roles:
            raise HTTPException(status_code=400, detail="Perfil inválido")
        if new_role == "global_admin" and target.role != "global_admin":
            raise HTTPException(status_code=403, detail="O perfil Administrador Global é reservado à conta proprietária")
        if not is_global_admin(user) and new_role in {"platform_admin", "global_admin"}:
            raise HTTPException(status_code=403, detail="Perfil não permitido")
        target.role = new_role
    if "company_id" in changes:
        if not is_global_admin(user):
            if changes["company_id"] not in (None, user.company_id):
                raise HTTPException(status_code=403, detail="Empresa não permitida")
            target.company_id = user.company_id
        else:
            target.company_id = changes["company_id"]
    if target.role in {"platform_admin", "global_admin"}:
        target.company_id = None
    elif target.company_id is None:
        raise HTTPException(status_code=400, detail="Usuários da empresa precisam estar vinculados a uma empresa")
    elif not db.get(Company, target.company_id):
        raise HTTPException(status_code=404, detail="Empresa não encontrada")
    if "active" in changes:
        target.active = bool(changes["active"])

    after = {
        "name": target.name,
        "email": target.email,
        "role": target.role,
        "company_id": target.company_id,
        "active": target.active,
    }
    db.add(
        AuditLog(
            company_id=target.company_id,
            actor_user_id=user.id,
            action="user.update",
            details=json.dumps({"user_id": target.id, "before": before, "after": after}, ensure_ascii=False),
        )
    )
    db.commit()
    db.refresh(target)
    return {
        "id": target.id,
        "name": target.name,
        "email": target.email,
        "role": target.role,
        "company_id": target.company_id,
        "active": target.active,
    }


@router.post("/devices/install", status_code=201)
def install_device(payload: DeviceInstallRequest, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin", "technician")

    if is_global_admin(user):
        if payload.company_id is None:
            raise HTTPException(status_code=400, detail="Selecione a empresa para instalar este computador")
        company_id = payload.company_id
    else:
        if user.company_id is None:
            raise HTTPException(status_code=403, detail="Usuário sem empresa vinculada")
        if payload.company_id not in (None, user.company_id):
            raise HTTPException(status_code=403, detail="Empresa não permitida")
        company_id = user.company_id

    company = db.get(Company, company_id)
    if not company or not company.active:
        raise HTTPException(status_code=404, detail="Empresa não encontrada")

    now = utcnow()
    raw_secret = f"ctagt_{new_secret(36)}"
    device = db.scalar(
        select(Device).where(Device.company_id == company_id, Device.device_uid == payload.device_uid)
    )
    created = device is None
    if device is None:
        device = Device(
            company_id=company_id,
            device_uid=payload.device_uid,
            name=payload.name.strip(),
            hostname=payload.hostname.strip(),
            sector=payload.sector,
            location=payload.location,
            manufacturer=payload.manufacturer,
            model=payload.model,
            serial_number=payload.serial_number,
            os_name=payload.os_name,
            os_version=payload.os_version,
            agent_version=payload.agent_version,
            agent_secret_hash=sha256_text(raw_secret),
            first_seen=now,
            last_seen=now,
            active=True,
        )
        db.add(device)
        db.flush()
    else:
        device.name = payload.name.strip()
        device.hostname = payload.hostname.strip()
        device.sector = payload.sector
        device.location = payload.location
        device.manufacturer = payload.manufacturer
        device.model = payload.model
        device.serial_number = payload.serial_number
        device.os_name = payload.os_name
        device.os_version = payload.os_version
        device.agent_version = payload.agent_version
        device.agent_secret_hash = sha256_text(raw_secret)
        device.last_seen = now
        device.active = True

    db.add(
        AuditLog(
            company_id=company_id,
            actor_user_id=user.id,
            device_id=device.id,
            action="device.install" if created else "device.reinstall",
            details=json.dumps(
                {
                    "hostname": device.hostname,
                    "uid": device.device_uid,
                    "source": "CoreTunerSetup",
                    "install_remote": payload.install_remote,
                },
                ensure_ascii=False,
            ),
        )
    )
    db.commit()
    remote_agent = None
    remote_warning = None
    if payload.install_remote:
        remote_agent, remote_warning = prepare_remote_install(db, company, device)
    return {
        "ok": True,
        "created": created,
        "device_id": device.id,
        "company_id": company.id,
        "company_name": company.name,
        "agent_secret": raw_secret,
        "remote_agent": remote_agent,
        "remote_warning": remote_warning,
        "device": serialize_device(db, device),
    }


# ---------------- Agent API ----------------


def get_agent_secret(authorization: str | None) -> str:
    if not authorization or not authorization.lower().startswith("bearer "):
        raise HTTPException(status_code=401, detail="Credencial do agente necessária")
    return authorization.split(" ", 1)[1].strip()


@router.post("/agent/enroll", status_code=201)
def agent_enroll(payload: EnrollmentRequest, db: Db):
    enrollment, company = get_valid_enrollment(db, payload.enrollment_token)
    now = utcnow()
    existing = db.scalar(
        select(Device).where(Device.company_id == enrollment.company_id, Device.device_uid == payload.device_uid)
    )
    raw_secret = f"ctagt_{new_secret(36)}"
    if existing:
        device = existing
        device.name = payload.name
        device.hostname = payload.hostname
        device.sector = payload.sector
        device.location = payload.location
        device.manufacturer = payload.manufacturer
        device.model = payload.model
        device.serial_number = payload.serial_number
        device.os_name = payload.os_name
        device.os_version = payload.os_version
        device.agent_version = payload.agent_version
        device.agent_secret_hash = sha256_text(raw_secret)
        device.last_seen = now
        device.active = True
    else:
        device = Device(
            company_id=enrollment.company_id,
            device_uid=payload.device_uid,
            name=payload.name,
            hostname=payload.hostname,
            sector=payload.sector,
            location=payload.location,
            manufacturer=payload.manufacturer,
            model=payload.model,
            serial_number=payload.serial_number,
            os_name=payload.os_name,
            os_version=payload.os_version,
            agent_version=payload.agent_version,
            agent_secret_hash=sha256_text(raw_secret),
            first_seen=now,
            last_seen=now,
        )
        db.add(device)
        db.flush()
    enrollment.used_at = now
    db.add(
        AuditLog(
            company_id=device.company_id,
            actor_user_id=enrollment.created_by,
            device_id=device.id,
            action="agent.enroll",
            details=json.dumps({"hostname": device.hostname, "uid": device.device_uid, "source": "installation_authorization"}, ensure_ascii=False),
        )
    )
    db.commit()
    return {
        "device_id": device.id,
        "agent_secret": raw_secret,
        "company_id": device.company_id,
        "company_name": company.name,
        "remote_agent": None,
        "remote_warning": "Acesso remoto não é instalado pela autorização temporária de uso único.",
    }


@router.post("/agent/telemetry")
def agent_telemetry(
    payload: TelemetryRequest,
    db: Db,
    authorization: Annotated[str | None, Header()] = None,
):
    raw_secret = get_agent_secret(authorization)
    secret_hash = sha256_text(raw_secret)
    device = db.scalar(
        select(Device).where(
            Device.agent_secret_hash == secret_hash,
            Device.device_uid == payload.device_uid,
            Device.active.is_(True),
        )
    )
    if not device:
        raise HTTPException(status_code=401, detail="Agente não autorizado")
    now = utcnow()
    device.last_seen = now
    if payload.profile:
        device.profile = payload.profile
    agent_version = str((payload.extra or {}).get("agent_version") or "").strip()
    if agent_version:
        device.agent_version = agent_version[:40]
    sample = Telemetry(
        device_id=device.id,
        recorded_at=now,
        cpu_percent=payload.cpu_percent,
        memory_percent=payload.memory_percent,
        memory_used_gb=payload.memory_used_gb,
        memory_total_gb=payload.memory_total_gb,
        disk_percent=payload.disk_percent,
        disk_free_gb=payload.disk_free_gb,
        disk_total_gb=payload.disk_total_gb,
        temperature_c=payload.temperature_c,
        uptime_seconds=payload.uptime_seconds,
        ip_local=payload.ip_local,
        network_name=payload.network_name,
        defender_active=payload.defender_active,
        firewall_active=payload.firewall_active,
        raw_json=json.dumps(payload.extra or {}, ensure_ascii=False),
    )
    db.add(sample)
    db.flush()
    evaluate_telemetry_alerts(db, device, sample)
    maybe_enqueue_update_policy(db, device, now)
    db.commit()
    return {"ok": True, "server_time": now.isoformat(), "next_interval_seconds": 30}
