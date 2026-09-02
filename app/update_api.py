from __future__ import annotations
import json
from datetime import timedelta
from typing import Annotated
from fastapi import APIRouter, Depends, Header, HTTPException, Request
from sqlalchemy import desc, select
from sqlalchemy.orm import Session

from .api import (
    assert_device_access,
    current_user,
    device_online,
    get_agent_secret,
    is_global_admin,
    iso,
    require_roles,
)
from .db import get_db
from .models import AgentCommand, AuditLog, Company, Device, DeviceUpdateState, UpdatePolicy, User
from .schemas import (
    AgentCommandResultRequest,
    OptimizationApplyRequest,
    UpdateCheckRequest,
    UpdateInstallRequest,
    UpdatePolicyCreate,
    UpdatePolicyUpdate,
)
from .security import sha256_text
from .update_service import (
    active_update_command,
    apply_scan_result,
    build_install_payload,
    get_update_state,
    json_list,
    json_object,
    queue_agent_command,
    utcnow,
)

router = APIRouter(prefix="/api")
Db = Annotated[Session, Depends(get_db)]
CurrentUser = Annotated[User, Depends(current_user)]




def _version_tuple(value: str | None) -> tuple[int, int, int]:
    parts = []
    for piece in str(value or "").strip().lstrip("vV").split(".")[:3]:
        digits = "".join(character for character in piece if character.isdigit())
        parts.append(int(digits or 0))
    return tuple((parts + [0, 0, 0])[:3])


def _agent_supports_updates(device: Device) -> bool:
    return _version_tuple(device.agent_version) >= (0, 5, 0)

def _agent_device(db: Session, authorization: str | None, device_uid: str) -> Device:
    raw_secret = get_agent_secret(authorization)
    device = db.scalar(
        select(Device).where(
            Device.agent_secret_hash == sha256_text(raw_secret),
            Device.device_uid == device_uid,
            Device.active.is_(True),
        )
    )
    if not device:
        raise HTTPException(status_code=401, detail="Agente não autorizado")
    return device


def _latest_command(db: Session, device_id: int) -> AgentCommand | None:
    return db.scalar(
        select(AgentCommand)
        .where(
            AgentCommand.device_id == device_id,
            AgentCommand.command_type.in_(["updates.scan", "updates.install"]),
        )
        .order_by(desc(AgentCommand.created_at))
        .limit(1)
    )


def _command_public(command: AgentCommand | None) -> dict | None:
    if not command:
        return None
    return {
        "id": command.id,
        "type": command.command_type,
        "status": command.status,
        "created_at": iso(command.created_at),
        "claimed_at": iso(command.claimed_at),
        "finished_at": iso(command.finished_at),
        "error": command.error_text,
    }


def _state_public(db: Session, device: Device, *, include_items: bool = False) -> dict:
    state = get_update_state(db, device.id)
    command = _latest_command(db, device.id)
    if command and command.status in {"queued", "running"}:
        status = "installing" if command.command_type == "updates.install" and command.status == "running" else "scanning" if command.command_type == "updates.scan" and command.status == "running" else "queued"
    elif command and command.status == "failed":
        status = "error"
    elif state and state.last_scan_at:
        status = "ready"
    else:
        status = "not_scanned"

    windows_pending = int(state.windows_pending if state else 0)
    driver_pending = int(state.driver_pending if state else 0)
    app_pending = int(state.app_pending if state else 0)
    result = {
        "device_id": device.id,
        "device_name": device.name,
        "hostname": device.hostname,
        "company_id": device.company_id,
        "company_name": device.company.name if device.company else None,
        "online": device_online(device),
        "agent_version": device.agent_version,
        "agent_supports_updates": _agent_supports_updates(device),
        "status": status,
        "last_scan_at": iso(state.last_scan_at) if state else None,
        "last_install_at": iso(state.last_install_at) if state else None,
        "windows_pending": windows_pending,
        "driver_pending": driver_pending,
        "app_pending": app_pending,
        "pending_total": windows_pending + driver_pending + app_pending,
        "critical_pending": int(state.critical_pending if state else 0),
        "reboot_required": bool(state.reboot_required if state else False),
        "last_error": state.last_error if state else None,
        "command": _command_public(command),
    }
    if include_items:
        result["items"] = json_list(state.inventory_json if state else None)
    return result


def _accessible_devices(db: Session, user: User, device_ids: list[int] | None = None) -> list[Device]:
    stmt = select(Device).where(Device.active.is_(True)).order_by(Device.name)
    if not is_global_admin(user):
        if user.company_id is None:
            return []
        stmt = stmt.where(Device.company_id == user.company_id)
    if device_ids is not None:
        if not device_ids:
            return []
        stmt = stmt.where(Device.id.in_(device_ids))
    return list(db.scalars(stmt).all())


@router.get("/updates")
def updates_dashboard(user: CurrentUser, db: Db):
    devices = _accessible_devices(db, user)
    entries = [_state_public(db, device) for device in devices]
    return {
        "summary": {
            "devices": len(entries),
            "scanned": sum(1 for item in entries if item["last_scan_at"]),
            "pending": sum(item["pending_total"] for item in entries),
            "critical": sum(item["critical_pending"] for item in entries),
            "reboot_required": sum(1 for item in entries if item["reboot_required"]),
            "busy": sum(1 for item in entries if item["status"] in {"queued", "scanning", "installing"}),
        },
        "devices": entries,
    }


@router.get("/updates/devices/{device_id}")
def update_device_detail(device_id: int, user: CurrentUser, db: Db):
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    return _state_public(db, device, include_items=True)


@router.post("/updates/check")
def queue_update_check(payload: UpdateCheckRequest, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin", "technician")
    devices = _accessible_devices(db, user, payload.device_ids)
    if payload.device_ids is not None and len(devices) != len(set(payload.device_ids)):
        raise HTTPException(status_code=404, detail="Um ou mais computadores não foram encontrados ou não são permitidos")
    if not devices:
        raise HTTPException(status_code=400, detail="Nenhum computador disponível para verificar")

    unsupported_devices = [device for device in devices if not _agent_supports_updates(device)]
    if payload.device_ids is not None and unsupported_devices:
        raise HTTPException(
            status_code=409,
            detail="Atualize o CoreControl Agent deste computador para a versão 0.5.0 ou superior antes de usar Atualizações.",
        )

    queued = 0
    existing = 0
    unsupported = len(unsupported_devices)
    for device in devices:
        if not _agent_supports_updates(device):
            continue
        _, created = queue_agent_command(
            db,
            device,
            "updates.scan",
            {},
            created_by=user.id,
        )
        if created:
            queued += 1
        else:
            existing += 1
    db.add(
        AuditLog(
            company_id=user.company_id,
            actor_user_id=user.id,
            action="updates.scan.queue",
            details=json.dumps({"device_ids": [device.id for device in devices], "queued": queued}, ensure_ascii=False),
        )
    )
    db.commit()
    if queued == 0 and existing == 0 and unsupported:
        raise HTTPException(
            status_code=409,
            detail="Os computadores encontrados ainda usam um CoreControl Agent antigo. Reinstale/atualize o agente para a versão 0.5.0 ou superior.",
        )
    return {"ok": True, "queued": queued, "already_pending": existing, "unsupported": unsupported, "devices": len(devices)}


@router.post("/updates/install")
def queue_update_install(payload: UpdateInstallRequest, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin", "technician")
    device = db.get(Device, payload.device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    if not _agent_supports_updates(device):
        raise HTTPException(
            status_code=409,
            detail="Atualize o CoreControl Agent deste computador para a versão 0.5.0 ou superior.",
        )
    active = active_update_command(db, device.id)
    if active:
        raise HTTPException(status_code=409, detail="O computador já possui uma operação de atualização em andamento")
    state = get_update_state(db, device.id)
    if not state or not state.last_scan_at:
        raise HTTPException(status_code=409, detail="Verifique as atualizações antes de instalar")
    try:
        command_payload = build_install_payload(state, payload.item_keys)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    if not any(command_payload.values()):
        raise HTTPException(status_code=400, detail="Nenhuma atualização válida foi selecionada")

    command, _ = queue_agent_command(
        db,
        device,
        "updates.install",
        command_payload,
        created_by=user.id,
        deduplicate=False,
    )
    db.add(
        AuditLog(
            company_id=device.company_id,
            actor_user_id=user.id,
            device_id=device.id,
            action="updates.install.queue",
            details=json.dumps({"command_id": command.id, "items": payload.item_keys}, ensure_ascii=False),
        )
    )
    db.commit()
    return {"ok": True, "command_id": command.id, "status": command.status}


@router.get("/updates/history")
def update_history(user: CurrentUser, db: Db, limit: int = 100):
    limit = max(1, min(500, int(limit)))
    stmt = (
        select(AgentCommand)
        .where(AgentCommand.command_type.in_(["updates.scan", "updates.install"]))
        .order_by(desc(AgentCommand.created_at))
        .limit(limit)
    )
    if not is_global_admin(user):
        stmt = stmt.where(AgentCommand.company_id == user.company_id)
    commands = list(db.scalars(stmt).all())
    devices = {device.id: device for device in db.scalars(select(Device).where(Device.id.in_([c.device_id for c in commands]))).all()} if commands else {}
    return [
        {
            **_command_public(command),
            "device_id": command.device_id,
            "device_name": devices.get(command.device_id).name if devices.get(command.device_id) else f"Computador #{command.device_id}",
            "company_id": command.company_id,
        }
        for command in commands
    ]


def _policy_public(policy: UpdatePolicy, company_name: str | None = None) -> dict:
    try:
        days = [int(value) for value in policy.allowed_days.split(",") if value.strip()]
    except ValueError:
        days = []
    return {
        "id": policy.id,
        "company_id": policy.company_id,
        "company_name": company_name,
        "name": policy.name,
        "active": policy.active,
        "auto_scan": policy.auto_scan,
        "auto_install": policy.auto_install,
        "include_windows": policy.include_windows,
        "include_drivers": policy.include_drivers,
        "include_apps": policy.include_apps,
        "scan_interval_hours": policy.scan_interval_hours,
        "allowed_days": days,
        "start_hour": policy.start_hour,
        "end_hour": policy.end_hour,
        "timezone": policy.timezone,
        "last_auto_action_at": iso(policy.last_auto_action_at),
    }


@router.get("/updates/policies")
def list_update_policies(user: CurrentUser, db: Db):
    stmt = select(UpdatePolicy).order_by(UpdatePolicy.name)
    if not is_global_admin(user):
        stmt = stmt.where(UpdatePolicy.company_id == user.company_id)
    policies = list(db.scalars(stmt).all())
    companies = {company.id: company.name for company in db.scalars(select(Company)).all()}
    return [_policy_public(policy, companies.get(policy.company_id)) for policy in policies]


@router.post("/updates/policies", status_code=201)
def create_update_policy(payload: UpdatePolicyCreate, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    if is_global_admin(user):
        if payload.company_id is None:
            raise HTTPException(status_code=400, detail="Selecione uma empresa")
        company_id = payload.company_id
    else:
        if user.company_id is None:
            raise HTTPException(status_code=403, detail="Usuário sem empresa")
        if payload.company_id not in (None, user.company_id):
            raise HTTPException(status_code=403, detail="Empresa não permitida")
        company_id = user.company_id
    company = db.get(Company, company_id)
    if not company:
        raise HTTPException(status_code=404, detail="Empresa não encontrada")
    policy = UpdatePolicy(
        company_id=company_id,
        name=payload.name.strip(),
        active=payload.active,
        auto_scan=payload.auto_scan,
        auto_install=payload.auto_install,
        include_windows=payload.include_windows,
        include_drivers=payload.include_drivers,
        include_apps=payload.include_apps,
        scan_interval_hours=payload.scan_interval_hours,
        allowed_days=",".join(str(day) for day in payload.allowed_days),
        start_hour=payload.start_hour,
        end_hour=payload.end_hour,
        timezone=payload.timezone.strip(),
        created_by=user.id,
        updated_at=utcnow(),
    )
    db.add(policy)
    db.flush()
    db.add(AuditLog(company_id=company_id, actor_user_id=user.id, action="updates.policy.create", details=policy.name))
    db.commit()
    return _policy_public(policy, company.name)


@router.patch("/updates/policies/{policy_id}")
def update_update_policy(policy_id: int, payload: UpdatePolicyUpdate, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    policy = db.get(UpdatePolicy, policy_id)
    if not policy:
        raise HTTPException(status_code=404, detail="Política não encontrada")
    if not is_global_admin(user) and policy.company_id != user.company_id:
        raise HTTPException(status_code=403, detail="Empresa não permitida")
    changes = payload.model_dump(exclude_unset=True)
    if "allowed_days" in changes:
        changes["allowed_days"] = ",".join(str(day) for day in changes["allowed_days"])
    for field, value in changes.items():
        setattr(policy, field, value.strip() if field in {"name", "timezone"} and isinstance(value, str) else value)
    policy.updated_at = utcnow()
    db.add(AuditLog(company_id=policy.company_id, actor_user_id=user.id, action="updates.policy.update", details=policy.name))
    db.commit()
    company = db.get(Company, policy.company_id)
    return _policy_public(policy, company.name if company else None)


@router.delete("/updates/policies/{policy_id}")
def delete_update_policy(policy_id: int, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    policy = db.get(UpdatePolicy, policy_id)
    if not policy:
        raise HTTPException(status_code=404, detail="Política não encontrada")
    if not is_global_admin(user) and policy.company_id != user.company_id:
        raise HTTPException(status_code=403, detail="Empresa não permitida")
    company_id = policy.company_id
    name = policy.name
    db.delete(policy)
    db.add(AuditLog(company_id=company_id, actor_user_id=user.id, action="updates.policy.delete", details=name))
    db.commit()
    return {"ok": True}


@router.get("/agent/commands/next")
def agent_next_command(
    device_uid: str,
    db: Db,
    authorization: Annotated[str | None, Header()] = None,
):
    device = _agent_device(db, authorization, device_uid)
    now = utcnow()
    stale_before = now - timedelta(hours=2)
    stale = list(
        db.scalars(
            select(AgentCommand).where(
                AgentCommand.device_id == device.id,
                AgentCommand.status == "running",
                AgentCommand.claimed_at < stale_before,
            )
        ).all()
    )
    for command in stale:
        command.status = "failed"
        command.finished_at = now
        command.error_text = "O agente não concluiu a operação dentro do tempo esperado."

    command = db.scalar(
        select(AgentCommand)
        .where(AgentCommand.device_id == device.id, AgentCommand.status == "queued")
        .order_by(AgentCommand.created_at)
        .limit(1)
    )
    if command:
        command.status = "running"
        command.claimed_at = now
        db.commit()
        return {
            "command": {
                "id": command.id,
                "type": command.command_type,
                "payload": json_object(command.payload_json),
            }
        }
    if stale:
        db.commit()
    return {"command": None}


@router.post("/agent/commands/{command_id}/result")
def agent_command_result(
    command_id: int,
    payload: AgentCommandResultRequest,
    db: Db,
    authorization: Annotated[str | None, Header()] = None,
):
    device = _agent_device(db, authorization, payload.device_uid)
    command = db.get(AgentCommand, command_id)
    if not command or command.device_id != device.id:
        raise HTTPException(status_code=404, detail="Comando não encontrado")
    if command.status not in {"queued", "running"}:
        return {"ok": True, "status": command.status}

    now = utcnow()
    command.finished_at = now
    command.status = "succeeded" if payload.ok else "failed"
    command.result_json = json.dumps(payload.result or {}, ensure_ascii=False)
    command.error_text = (payload.error or "").strip()[:4000] or None

    if command.command_type == "updates.scan":
        if payload.ok:
            apply_scan_result(db, device, payload.result or {})
        else:
            state = get_update_state(db, device.id, create=True)
            state.last_error = command.error_text or "Falha ao verificar atualizações"
            state.updated_at = now
    elif command.command_type == "updates.install":
        state = get_update_state(db, device.id, create=True)
        state.last_install_at = now
        result = payload.result or {}
        if result.get("reboot_required"):
            state.reboot_required = True
        if not payload.ok:
            state.last_error = command.error_text or "Falha ao instalar atualizações"
        else:
            state.last_error = None
        # Uma nova verificação é segura e atualiza a lista mesmo quando houve falha parcial.
        queue_agent_command(db, device, "updates.scan", {"after_install": command.id}, created_by=command.created_by)
    elif command.command_type == "optimization.apply":
        result = payload.result or {}
        if payload.ok:
            active_profile = int(result.get("active_profile") or 0)
            active_name = str(result.get("active_profile_name") or "").strip()
            device.profile = active_name[:80] if active_profile > 0 and active_name else "Nenhum"

    db.add(
        AuditLog(
            company_id=device.company_id,
            actor_user_id=command.created_by,
            device_id=device.id,
            action=f"{command.command_type}.{'success' if payload.ok else 'failed'}",
            details=json.dumps(
                {"command_id": command.id, "error": command.error_text, "result": payload.result or {}},
                ensure_ascii=False,
            )[:12000],
        )
    )
    db.commit()
    return {"ok": True, "status": command.status}


def _agent_supports_activity(device: Device) -> bool:
    return _version_tuple(device.agent_version) >= (0, 6, 0)


def _latest_activity_command(db: Session, device_id: int) -> AgentCommand | None:
    return db.scalar(
        select(AgentCommand)
        .where(
            AgentCommand.device_id == device_id,
            AgentCommand.command_type == "activity.snapshot",
        )
        .order_by(desc(AgentCommand.created_at))
        .limit(1)
    )


def _latest_successful_activity_command(db: Session, device_id: int) -> AgentCommand | None:
    """Mantém a última coleta válida disponível enquanto uma nova coleta está em andamento."""
    return db.scalar(
        select(AgentCommand)
        .where(
            AgentCommand.device_id == device_id,
            AgentCommand.command_type == "activity.snapshot",
            AgentCommand.status == "succeeded",
        )
        .order_by(desc(AgentCommand.finished_at), desc(AgentCommand.created_at))
        .limit(1)
    )


def _activity_command_public(command: AgentCommand | None) -> dict | None:
    if not command:
        return None
    result = json_object(command.result_json) if command.result_json else {}
    return {
        "id": command.id,
        "status": command.status,
        "created_at": iso(command.created_at),
        "claimed_at": iso(command.claimed_at),
        "finished_at": iso(command.finished_at),
        "error": command.error_text,
        "result": result if command.status == "succeeded" else None,
    }


@router.post("/devices/{device_id}/activity/snapshot")
def request_activity_snapshot(device_id: int, user: CurrentUser, db: Db):
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    if not _agent_supports_activity(device):
        raise HTTPException(status_code=409, detail="Atualize o CoreControl Agent para a versão 0.6.0 ou superior")
    command, created = queue_agent_command(
        db,
        device,
        "activity.snapshot",
        {},
        created_by=user.id,
        deduplicate=True,
    )
    if created:
        db.add(
            AuditLog(
                company_id=device.company_id,
                actor_user_id=user.id,
                device_id=device.id,
                action="activity.snapshot.request",
                details="Consulta de aplicativos abertos solicitada pelo painel.",
            )
        )
    db.commit()
    return {
        "created": created,
        "agent_supports_activity": True,
        "command": _activity_command_public(command),
        "cached_command": _activity_command_public(_latest_successful_activity_command(db, device.id)),
    }


@router.get("/devices/{device_id}/activity/snapshot")
def get_activity_snapshot(device_id: int, user: CurrentUser, db: Db):
    device = db.get(Device, device_id)
    if not device:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    return {
        "agent_supports_activity": _agent_supports_activity(device),
        "command": _activity_command_public(_latest_activity_command(db, device.id)),
        "cached_command": _activity_command_public(_latest_successful_activity_command(db, device.id)),
    }


OPTIMIZATION_PROFILES = [
    {
        "id": 1,
        "name": "Conservador",
        "short": "Deixa o Windows mais leve sem mudar energia ou programas.",
        "actions": ["Reduz pequenas animações da interface", "Mantém o plano de energia atual", "Não altera prioridade de programas"],
    },
    {
        "id": 2,
        "name": "Equilibrado",
        "short": "Reduz efeitos visuais e mantém desempenho e consumo equilibrados.",
        "actions": ["Reduz animações de janelas e menus", "Ativa o plano Equilibrado", "Não altera prioridade de programas"],
    },
    {
        "id": 3,
        "name": "Modo Atendimento",
        "short": "Prepara o PC para navegador, WhatsApp, CRM e discador.",
        "actions": ["Reduz animações", "Mantém energia em Equilibrado", "Prioriza moderadamente aplicativos de atendimento abertos"],
    },
    {
        "id": 4,
        "name": "Alto Desempenho",
        "short": "Entrega mais resposta quando o computador está ligado à tomada.",
        "actions": ["Reduz animações", "Prioriza aplicativos de atendimento", "Usa Alto desempenho na tomada e Equilibrado na bateria"],
    },
    {
        "id": 5,
        "name": "Desativar otimização",
        "short": "Restaura o estado salvo antes da primeira otimização.",
        "actions": ["Restaura animações", "Restaura plano de energia", "Restaura prioridades ainda aplicáveis"],
    },
]


def _agent_supports_optimization(device: Device) -> bool:
    return _version_tuple(device.agent_version) >= (0, 9, 0)


def _agent_supports_optimization_insights(device: Device) -> bool:
    return _version_tuple(device.agent_version) >= (0, 9, 1)


def _latest_optimization_command(db: Session, device_id: int) -> AgentCommand | None:
    return db.scalar(
        select(AgentCommand)
        .where(
            AgentCommand.device_id == device_id,
            AgentCommand.command_type == "optimization.apply",
        )
        .order_by(desc(AgentCommand.created_at))
        .limit(1)
    )


def _latest_optimization_diagnostic_command(db: Session, device_id: int) -> AgentCommand | None:
    return db.scalar(
        select(AgentCommand)
        .where(
            AgentCommand.device_id == device_id,
            AgentCommand.command_type.in_(["optimization.diagnose", "optimization.apply", "optimization.cleanup_temp"]),
            AgentCommand.status == "succeeded",
        )
        .order_by(desc(AgentCommand.finished_at), desc(AgentCommand.created_at))
        .limit(1)
    )


def _latest_optimization_insight_operation(db: Session, device_id: int) -> AgentCommand | None:
    return db.scalar(
        select(AgentCommand)
        .where(
            AgentCommand.device_id == device_id,
            AgentCommand.command_type.in_(["optimization.diagnose", "optimization.cleanup_temp"]),
        )
        .order_by(desc(AgentCommand.created_at))
        .limit(1)
    )


def _optimization_diagnostics_from_command(command: AgentCommand | None) -> dict | None:
    if not command or not command.result_json:
        return None
    result = json_object(command.result_json)
    if command.command_type == "optimization.diagnose":
        diagnostics = result
    else:
        diagnostics = result.get("diagnostics_after") if isinstance(result, dict) else None
    return diagnostics if isinstance(diagnostics, dict) and diagnostics else None


def _optimization_insight_command_public(command: AgentCommand | None) -> dict | None:
    if not command:
        return None
    result = json_object(command.result_json) if command.result_json else {}
    return {
        "id": command.id,
        "type": command.command_type,
        "status": command.status,
        "created_at": iso(command.created_at),
        "claimed_at": iso(command.claimed_at),
        "finished_at": iso(command.finished_at),
        "error": command.error_text,
        "result": result if command.result_json else None,
    }


def _optimization_command_public(command: AgentCommand | None) -> dict | None:
    if not command:
        return None
    result = json_object(command.result_json) if command.result_json else {}
    payload = json_object(command.payload_json) if command.payload_json else {}
    return {
        "id": command.id,
        "profile": int(payload.get("profile") or 0),
        "status": command.status,
        "created_at": iso(command.created_at),
        "claimed_at": iso(command.claimed_at),
        "finished_at": iso(command.finished_at),
        "error": command.error_text,
        "result": result if command.result_json else None,
    }


@router.get("/devices/{device_id}/optimization")
def get_device_optimization(device_id: int, user: CurrentUser, db: Db):
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    current = str(device.profile or "").strip()
    if not current or current.casefold() == "nenhum":
        current = None
    diagnostic_source = _latest_optimization_diagnostic_command(db, device.id)
    return {
        "agent_supports_optimization": _agent_supports_optimization(device),
        "agent_supports_optimization_insights": _agent_supports_optimization_insights(device),
        "online": device_online(device),
        "active_profile_name": current,
        "profiles": OPTIMIZATION_PROFILES,
        "command": _optimization_command_public(_latest_optimization_command(db, device.id)),
        "diagnostics": _optimization_diagnostics_from_command(diagnostic_source),
        "diagnostics_source": _optimization_insight_command_public(diagnostic_source),
        "insight_command": _optimization_insight_command_public(_latest_optimization_insight_operation(db, device.id)),
    }


@router.post("/devices/{device_id}/optimization/diagnose")
def diagnose_device_optimization(device_id: int, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    if not _agent_supports_optimization_insights(device):
        raise HTTPException(status_code=409, detail="Atualize o CoreControl Agent para a versão 0.9.1 ou superior para usar o diagnóstico inteligente")
    if not device_online(device):
        raise HTTPException(status_code=409, detail="O computador precisa estar online para executar o diagnóstico")
    command, created = queue_agent_command(db, device, "optimization.diagnose", {}, created_by=user.id, deduplicate=True)
    if created:
        db.add(AuditLog(company_id=device.company_id, actor_user_id=user.id, device_id=device.id, action="optimization.diagnose.request", details="Diagnóstico inteligente solicitado pelo painel."))
    db.commit()
    return {"created": created, "agent_supports_optimization_insights": True, "command": _optimization_insight_command_public(command)}


@router.post("/devices/{device_id}/optimization/cleanup-temp")
def cleanup_device_optimization_temp(device_id: int, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    if not _agent_supports_optimization_insights(device):
        raise HTTPException(status_code=409, detail="Atualize o CoreControl Agent para a versão 0.9.1 ou superior para usar a limpeza segura")
    if not device_online(device):
        raise HTTPException(status_code=409, detail="O computador precisa estar online para executar a limpeza")
    command, created = queue_agent_command(db, device, "optimization.cleanup_temp", {}, created_by=user.id, deduplicate=True)
    if created:
        db.add(AuditLog(company_id=device.company_id, actor_user_id=user.id, device_id=device.id, action="optimization.cleanup_temp.request", details="Limpeza segura de arquivos temporários antigos solicitada pelo painel."))
    db.commit()
    return {"created": created, "agent_supports_optimization_insights": True, "command": _optimization_insight_command_public(command)}


@router.post("/devices/{device_id}/optimization")
def apply_device_optimization(device_id: int, payload: OptimizationApplyRequest, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin")
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    if not _agent_supports_optimization(device):
        raise HTTPException(status_code=409, detail="Atualize o CoreControl Agent para a versão 0.9.0 ou superior para otimizar pelo painel")
    if not device_online(device):
        raise HTTPException(status_code=409, detail="O computador precisa estar online para aplicar uma otimização")

    command, created = queue_agent_command(
        db,
        device,
        "optimization.apply",
        {"profile": payload.profile},
        created_by=user.id,
        deduplicate=True,
    )
    if created:
        profile = next((item for item in OPTIMIZATION_PROFILES if item["id"] == payload.profile), None)
        db.add(
            AuditLog(
                company_id=device.company_id,
                actor_user_id=user.id,
                device_id=device.id,
                action="optimization.apply.request",
                details=json.dumps(
                    {"profile": payload.profile, "profile_name": (profile or {}).get("name")},
                    ensure_ascii=False,
                ),
            )
        )
    db.commit()
    return {
        "created": created,
        "agent_supports_optimization": True,
        "command": _optimization_command_public(command),
    }
