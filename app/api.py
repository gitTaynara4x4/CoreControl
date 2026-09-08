from __future__ import annotations

import hashlib
import io
import ipaddress
import json
import re
import secrets
import socket
import threading
import time
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
from .update_service import maybe_enqueue_update_policy, queue_agent_command
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


MAC_ADDRESS_RE = re.compile(r"^[0-9a-f]{2}(?::[0-9a-f]{2}){5}$", re.IGNORECASE)


def normalize_mac(value: object) -> str:
    raw = str(value or "").strip().lower().replace("-", ":")
    return raw if MAC_ADDRESS_RE.fullmatch(raw) else ""


def device_wol_info(db: Session, device: Device) -> dict:
    sample = latest_telemetry(db, device.id)
    extra = sample_extra(sample)
    capability = extra.get("wol_capability")
    capability = capability if isinstance(capability, dict) else {}
    return {
        "mac_address": normalize_mac(capability.get("mac_address")) or normalize_mac(extra.get("primary_mac")),
        "network_cidr": str(extra.get("network_cidr") or "").strip(),
        "relay_capable": bool(extra.get("wol_relay_capable")),
        "ip_local": sample.ip_local if sample else None,
        "capability_checked": bool(capability.get("checked")),
        "capability_checked_at": str(capability.get("checked_at") or "").strip() or None,
        "adapter_name": str(capability.get("adapter_name") or "").strip() or None,
        "interface_description": str(capability.get("interface_description") or "").strip() or None,
        "link_type": str(capability.get("link_type") or "").strip().lower() or "unknown",
        "magic_packet_supported": bool(capability.get("magic_packet_supported")),
        "magic_packet_enabled": bool(capability.get("magic_packet_enabled")),
        "wake_programmable": bool(capability.get("wake_programmable")),
        "wake_armed": bool(capability.get("wake_armed")),
        "s5_driver_hint": bool(capability.get("s5_driver_hint")),
        "intel_amt_detected": bool(capability.get("intel_amt_detected")),
        "auto_configured": bool(capability.get("auto_configured")),
        "windows_prepared": bool(capability.get("windows_prepared")),
        "firmware_needs_check": bool(capability.get("firmware_needs_check", True)),
        "capability_reason": str(capability.get("reason") or "").strip(),
        "capability_error": str(capability.get("error") or "").strip(),
    }


def find_wake_relays(db: Session, target: Device) -> list[Device]:
    target_info = device_wol_info(db, target)
    network_cidr = target_info.get("network_cidr") or ""
    mac_address = target_info.get("mac_address") or ""
    if not network_cidr or not mac_address:
        return []

    peers = list(
        db.scalars(
            select(Device).where(
                Device.company_id == target.company_id,
                Device.active.is_(True),
                Device.id != target.id,
            )
        ).all()
    )
    relays: list[Device] = []
    for peer in peers:
        if not device_online(peer):
            continue
        peer_info = device_wol_info(db, peer)
        if not peer_info.get("relay_capable"):
            continue
        if peer_info.get("network_cidr") != network_cidr:
            continue
        relays.append(peer)
    return relays


def _same_wol_network(target_info: dict, peer_info: dict) -> bool:
    """Conservative same-LAN check for MeshCentral WOL relays.

    Prefer the network CIDR reported by the Agent. Older samples may not have
    it, so fall back to IPv4 membership when one side does provide a network.
    We deliberately avoid broadcasting through arbitrary company devices that
    may be installed at another physical site.
    """
    target_cidr = str(target_info.get("network_cidr") or "").strip()
    peer_cidr = str(peer_info.get("network_cidr") or "").strip()
    if target_cidr and peer_cidr:
        try:
            return ipaddress.ip_network(target_cidr, strict=False) == ipaddress.ip_network(peer_cidr, strict=False)
        except ValueError:
            return False

    try:
        target_ip = ipaddress.ip_address(str(target_info.get("ip_local") or "").strip())
        peer_ip = ipaddress.ip_address(str(peer_info.get("ip_local") or "").strip())
    except ValueError:
        return False
    if target_ip.version != 4 or peer_ip.version != 4 or not target_ip.is_private or not peer_ip.is_private:
        return False
    if target_cidr:
        try:
            return peer_ip in ipaddress.ip_network(target_cidr, strict=False)
        except ValueError:
            return False
    if peer_cidr:
        try:
            return target_ip in ipaddress.ip_network(peer_cidr, strict=False)
        except ValueError:
            return False
    # Last-resort compatibility for old telemetry: only /24 private peers.
    return int(target_ip) >> 8 == int(peer_ip) >> 8


def find_mesh_wake_relays(db: Session, target: Device) -> list[Device]:
    """Online Mesh Agents in the same company/LAN usable as WOL relays."""
    target_info = device_wol_info(db, target)
    if not target_info.get("mac_address"):
        return []
    peers = list(
        db.scalars(
            select(Device).where(
                Device.company_id == target.company_id,
                Device.active.is_(True),
                Device.id != target.id,
                Device.mesh_node_id.is_not(None),
            )
        ).all()
    )
    relays: list[Device] = []
    for peer in peers:
        if not bool(peer.remote_online):
            continue
        peer_info = device_wol_info(db, peer)
        if _same_wol_network(target_info, peer_info):
            relays.append(peer)
    return relays


WAKE_ROUTE_VALID_FOR = timedelta(days=7)


def _command_json(value: str | None) -> dict:
    try:
        parsed = json.loads(value or "{}")
        return parsed if isinstance(parsed, dict) else {}
    except (TypeError, ValueError):
        return {}


def _version_at_least(value: str | None, minimum: tuple[int, int, int]) -> bool:
    parts: list[int] = []
    for piece in str(value or "").strip().lstrip("vV").split(".")[:3]:
        digits = "".join(character for character in piece if character.isdigit())
        parts.append(int(digits or 0))
    current = tuple((parts + [0, 0, 0])[:3])
    return current >= minimum


def latest_wan_wake_route(db: Session, device: Device) -> dict:
    probe = db.scalar(
        select(AgentCommand)
        .where(AgentCommand.device_id == device.id, AgentCommand.command_type == "power.route_probe")
        .order_by(desc(AgentCommand.created_at), desc(AgentCommand.id))
        .limit(1)
    )
    if not probe:
        return {"verified": False, "status": "idle", "message": "A rota externa ainda não foi testada."}

    probe_payload = _command_json(probe.payload_json)
    probe_result = _command_json(probe.result_json)
    token = str(probe_payload.get("probe_token") or probe_result.get("probe_token") or "").strip()

    confirms = list(
        db.scalars(
            select(AgentCommand)
            .where(AgentCommand.device_id == device.id, AgentCommand.command_type == "power.route_probe_confirm")
            .order_by(desc(AgentCommand.created_at), desc(AgentCommand.id))
            .limit(12)
        ).all()
    )
    confirm: AgentCommand | None = None
    for item in confirms:
        payload = _command_json(item.payload_json)
        result = _command_json(item.result_json)
        item_token = str(payload.get("probe_token") or result.get("probe_token") or "").strip()
        if token and item_token == token:
            confirm = item
            break

    if probe.status in {"queued", "running"}:
        return {"verified": False, "status": "testing", "message": "Testando UPnP e preparando a rota de Wake-on-LAN..."}
    if probe.status == "failed":
        # O erro técnico completo permanece registrado em AgentCommand.error_text
        # para diagnóstico, mas nunca é exposto na interface do cliente.
        return {
            "verified": False,
            "status": "failed",
            "message": "Não foi possível configurar automaticamente a rota pelo roteador.",
        }
    if confirm is None:
        return {"verified": False, "status": "verifying", "message": "Rota criada. Aguardando o teste externo da VPS..."}
    if confirm.status in {"queued", "running"}:
        return {"verified": False, "status": "verifying", "message": "A VPS enviou o teste. Aguardando confirmação do PC..."}
    if confirm.status != "succeeded":
        # Mantém detalhes técnicos somente nos logs/comandos internos.
        return {
            "verified": False,
            "status": "failed",
            "message": "Não foi possível confirmar automaticamente a rota de religamento.",
        }

    result = _command_json(confirm.result_json)
    if not bool(result.get("verified")):
        return {"verified": False, "status": "failed", "message": "O teste externo não confirmou a rota de Wake-on-LAN."}

    finished_at = as_utc(confirm.finished_at or confirm.created_at)
    method = str(result.get("method") or "upnp_broadcast").strip() or "upnp_broadcast"
    # Mapeamento UPnP para o IP do próprio PC é mais compatível com roteadores
    # domésticos, mas depende do lease/NAT/ARP do equipamento. Revalide com mais
    # frequência do que a rota broadcast antiga.
    valid_for = timedelta(hours=24) if method == "mesh_upnp_unicast" else WAKE_ROUTE_VALID_FOR
    route_expired = bool(not finished_at or utcnow() - finished_at > valid_for)

    external_ip = str(result.get("external_ip") or "").strip()
    try:
        parsed_ip = ipaddress.ip_address(external_ip)
        external_ip_ok = parsed_ip.version == 4 and parsed_ip.is_global
    except ValueError:
        external_ip_ok = False
    external_port = int(result.get("external_port") or 0)
    if not external_ip_ok or not (40000 <= external_port <= 59999):
        return {"verified": False, "status": "failed", "message": "A rota confirmada devolveu endereço externo inválido."}

    # Se o PC já está offline, não desative o único caminho de Wake só porque
    # venceu a janela de revalidação. A rota continua sendo tentável e não pode
    # ser revalidada até a máquina acordar. Quando o PC estiver online, aí sim
    # exigimos uma nova prova antes do próximo desligamento.
    if route_expired and device_power_currently_on(device):
        return {
            "verified": False,
            "status": "expired",
            "message": "A última rota confirmada expirou. O CoreControl renovará a rota antes do próximo desligamento.",
        }

    stale_offline = bool(route_expired)
    return {
        "verified": True,
        "status": "verified_stale" if stale_offline else "verified",
        "message": (
            "Usando a última rota Wake-on-WAN confirmada para tentar religar o computador offline."
            if stale_offline
            else "Rota externa confirmada pela VPS."
        ),
        "stale": stale_offline,
        "method": method,
        "external_ip": external_ip,
        "external_port": external_port,
        "internal_port": int(result.get("internal_port") or 0),
        "internal_ip": str(result.get("internal_ip") or "").strip() or None,
        "broadcast_ip": str(result.get("broadcast_ip") or "").strip() or None,
        "verified_at": iso(finished_at),
        "valid_for_seconds": int(valid_for.total_seconds()),
    }


def _send_wan_magic_packet(route: dict, mac_text: str) -> None:
    mac = normalize_mac(mac_text)
    if not mac:
        raise ValueError("MAC inválido para Wake-on-LAN")
    raw_mac = bytes.fromhex(mac.replace(":", ""))
    packet = b"\xff" * 6 + raw_mac * 16
    external_ip = str(route.get("external_ip") or "").strip()
    external_port = int(route.get("external_port") or 0)
    parsed_ip = ipaddress.ip_address(external_ip)
    if parsed_ip.version != 4 or not parsed_ip.is_global or not (40000 <= external_port <= 59999):
        raise ValueError("Rota WAN de Wake-on-LAN inválida")
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        sock.settimeout(2.0)
        for _ in range(3):
            sock.sendto(packet, (external_ip, external_port))
    finally:
        sock.close()



def _wake_route_port(mac_text: str) -> int:
    normalized = normalize_mac(mac_text) or str(mac_text or "").strip().lower()
    digest = int(hashlib.sha256(normalized.encode("utf-8")).hexdigest()[:8], 16)
    return 40000 + (digest % 19000)


def _route_public_ipv4(value: str | None) -> str | None:
    text = str(value or "").strip()
    try:
        parsed = ipaddress.ip_address(text)
    except ValueError:
        return None
    if parsed.version != 4 or not parsed.is_global:
        return None
    return text


def _network_broadcast(cidr: str | None) -> str | None:
    try:
        return str(ipaddress.ip_network(str(cidr or "").strip(), strict=False).broadcast_address)
    except ValueError:
        return None


def _record_verified_wan_route(
    db: Session,
    device: Device,
    *,
    created_by: int | None,
    token: str,
    method: str,
    external_ip: str,
    external_port: int,
    internal_port: int,
    internal_ip: str | None,
    network_cidr: str | None,
) -> dict:
    now = utcnow()
    broadcast_ip = _network_broadcast(network_cidr)
    probe_payload = {
        "probe_token": token,
        "external_port": external_port,
        "method": method,
    }
    probe_result = {
        "mapping_created": True,
        "probe_token": token,
        "method": method,
        "external_ip": external_ip,
        "external_port": external_port,
        "internal_port": internal_port,
        "internal_ip": internal_ip,
        "broadcast_ip": broadcast_ip,
    }
    confirm_payload = {
        "probe_token": token,
        "method": method,
        "external_ip": external_ip,
        "external_port": external_port,
        "internal_port": internal_port,
        "internal_ip": internal_ip,
        "broadcast_ip": broadcast_ip,
        "probe_sent": True,
    }
    confirm_result = {
        "verified": True,
        "probe_token": token,
        "method": method,
        "external_ip": external_ip,
        "external_port": external_port,
        "internal_port": internal_port,
        "internal_ip": internal_ip,
        "broadcast_ip": broadcast_ip,
        "received_at": iso(now),
    }
    db.add(
        AgentCommand(
            device_id=device.id,
            company_id=device.company_id,
            created_by=created_by,
            command_type="power.route_probe",
            payload_json=json.dumps(probe_payload, ensure_ascii=False),
            status="succeeded",
            created_at=now,
            claimed_at=now,
            finished_at=now,
            result_json=json.dumps(probe_result, ensure_ascii=False),
        )
    )
    db.add(
        AgentCommand(
            device_id=device.id,
            company_id=device.company_id,
            created_by=created_by,
            command_type="power.route_probe_confirm",
            payload_json=json.dumps(confirm_payload, ensure_ascii=False),
            status="succeeded",
            created_at=now,
            claimed_at=now,
            finished_at=now,
            result_json=json.dumps(confirm_result, ensure_ascii=False),
        )
    )
    db.flush()
    return {
        "verified": True,
        "status": "verified",
        "message": "Rota externa confirmada pela VPS.",
        "method": method,
        "external_ip": external_ip,
        "external_port": external_port,
        "internal_port": internal_port,
        "internal_ip": internal_ip,
        "broadcast_ip": broadcast_ip,
        "verified_at": iso(now),
    }


def _try_mesh_wan_route(
    db: Session,
    device: Device,
    *,
    created_by: int | None,
    target_info: dict | None = None,
) -> tuple[dict | None, str | None]:
    """Configure and prove a single-PC Wake-on-WAN route through Mesh Agent.

    The preferred route is UDP -> LAN broadcast. Once verified from the VPS,
    that route can wake a fully shut-down PC without relying on another machine
    inside the customer's network and without relying on a stale router ARP
    entry. Consumer routers that reject broadcast UPnP mappings fall back to a
    direct unicast mapping; that fallback is kept only for the S4/hibernate
    power-off path where the NIC can preserve ARP offload.
    """
    if not (
        settings.remote_enabled
        and meshcentral_client.provisioning_configured
        and device.mesh_node_id
    ):
        return None, "Acesso remoto não está disponível para preparar a rota externa."
    if not device_power_currently_on(device):
        return None, "O computador precisa estar ligado para preparar a rota externa."

    info = target_info or device_wol_info(db, device)
    mac_address = str(info.get("mac_address") or "").strip()
    if not normalize_mac(mac_address):
        return None, "O endereço MAC ainda não está disponível para preparar a rota externa."

    external_port = _wake_route_port(mac_address)
    broadcast_ip = _network_broadcast(str(info.get("network_cidr") or "").strip())
    candidates: list[tuple[str, str | None]] = []
    if broadcast_ip:
        candidates.append(("upnp_broadcast", broadcast_ip))
    candidates.append(("mesh_upnp_unicast", None))

    errors: list[str] = []
    for expected_method, candidate_broadcast in candidates:
        try:
            prepared = meshcentral_client.device_prepare_wan_wake_route(
                device.mesh_node_id,
                external_port,
                broadcast_ip=candidate_broadcast,
            )
        except MeshCentralCommandError as exc:
            errors.append(str(exc))
            continue

        external_ip = _route_public_ipv4(prepared.get("external_ip"))
        if not external_ip:
            errors.append("O roteador não possui um IPv4 público diretamente alcançável (possível CGNAT).")
            continue
        try:
            mapped_external_port = int(prepared.get("external_port") or 0)
            internal_port = int(prepared.get("internal_port") or 0)
        except (TypeError, ValueError):
            errors.append("O roteador devolveu portas inválidas para a rota externa.")
            continue
        if not (40000 <= mapped_external_port <= 59999 and 40000 <= internal_port <= 59999):
            errors.append("O roteador devolveu portas inválidas para a rota externa.")
            continue

        token = secrets.token_urlsafe(24)
        listener_result: dict[str, object] = {}

        def wait_for_probe() -> None:
            try:
                listener_result["value"] = meshcentral_client.device_wait_for_wan_probe(
                    device.mesh_node_id,
                    internal_port,
                    token,
                )
            except Exception as exc:  # detalhes ficam apenas no servidor
                listener_result["error"] = str(exc)

        thread = threading.Thread(target=wait_for_probe, name=f"corecontrol-wan-probe-{device.id}", daemon=True)
        thread.start()

        # RunCommand precisa de um instante para abrir o listener. O teste é
        # repetido para eliminar a corrida entre a VPS e o Windows remoto.
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        try:
            sock.settimeout(1.0)
            for _ in range(10):
                time.sleep(0.65)
                try:
                    sock.sendto(token.encode("utf-8"), (external_ip, mapped_external_port))
                except OSError:
                    pass
                if not thread.is_alive():
                    break
        finally:
            sock.close()

        thread.join(timeout=8.0)
        value = listener_result.get("value")
        if thread.is_alive() or not isinstance(value, dict) or not bool(value.get("received")):
            if expected_method == "upnp_broadcast":
                errors.append("O roteador criou a rota broadcast, mas o pacote externo não chegou pela LAN.")
            else:
                errors.append("A VPS não conseguiu confirmar que a rota externa chega até este computador.")
            continue

        prepared_method = str(prepared.get("method") or expected_method).strip() or expected_method
        # Nunca transforme uma rota unicast em garantia de S5 por engano.
        if candidate_broadcast:
            prepared_method = "upnp_broadcast"
        else:
            prepared_method = "mesh_upnp_unicast"

        route = _record_verified_wan_route(
            db,
            device,
            created_by=created_by,
            token=token,
            method=prepared_method,
            external_ip=external_ip,
            external_port=mapped_external_port,
            internal_port=internal_port,
            internal_ip=str(prepared.get("internal_ip") or "").strip() or None,
            network_cidr=str(info.get("network_cidr") or "").strip() or None,
        )
        return route, None

    clean_errors = [item.strip() for item in errors if str(item).strip()]
    return None, (clean_errors[-1] if clean_errors else "Não foi possível preparar uma rota externa de Wake-on-LAN.")


def _try_agent_wan_route_sync(
    db: Session,
    device: Device,
    *,
    created_by: int | None,
    target_info: dict | None = None,
    timeout_seconds: float = 34.0,
) -> tuple[dict | None, str | None]:
    """Try the Agent's native UPnP/SSDP route probe and wait for VPS proof.

    This is intentionally a second, independent implementation of route
    preparation.  Some routers work through the Agent's direct SSDP/SOAP
    discovery but not through Windows HNetCfg.NATUPnP (or vice-versa).  The
    target PC itself prepares the mapping while it is still online; no second
    computer inside the customer's LAN is required.
    """
    info = target_info or device_wol_info(db, device)
    mac_address = str(info.get("mac_address") or "").strip()
    network_cidr = str(info.get("network_cidr") or "").strip()
    if not normalize_mac(mac_address) or not network_cidr:
        return None, "O Agent ainda não informou MAC e sub-rede suficientes para preparar a rota externa."
    if not device_online(device):
        return None, "O CoreControl Agent precisa estar online por alguns segundos para preparar a rota alternativa."

    token = secrets.token_urlsafe(24)
    external_port = _wake_route_port(mac_address)
    command, _created = queue_agent_command(
        db,
        device,
        "power.route_probe",
        {
            "mac_address": mac_address,
            "network_cidr": network_cidr,
            "probe_token": token,
            "external_port": external_port,
        },
        created_by=created_by,
        # O token precisa ser único. Um teste anterior pendente nunca deve
        # impedir o preflight do desligamento atual.
        deduplicate=False,
    )
    db.commit()

    deadline = time.monotonic() + max(8.0, float(timeout_seconds))
    last_message = "Aguardando o Agent preparar a rota alternativa pelo roteador."
    while time.monotonic() < deadline:
        time.sleep(0.75)
        db.expire_all()
        current = db.get(AgentCommand, command.id)
        route = latest_wan_wake_route(db, device)
        if route.get("verified"):
            return route, None

        if current and current.status == "failed":
            # Não exponha stack/erro cru do Agent na UI. latest_wan_wake_route
            # já transforma o resultado em texto seguro para o operador.
            return None, str(route.get("message") or "O roteador não aceitou a rota alternativa de Wake-on-WAN.")

        route_status = str(route.get("status") or "").strip()
        if route_status == "failed" and current and current.status == "succeeded":
            return None, str(route.get("message") or "A VPS não conseguiu confirmar a rota alternativa de Wake-on-WAN.")
        if route.get("message"):
            last_message = str(route["message"])

    return None, f"{last_message} O teste automático excedeu o tempo de confirmação."


POWER_WAKE_PENDING_FOR = timedelta(minutes=5)
POWER_OFF_PENDING_FOR = timedelta(minutes=3)


def device_managed_off_state(db: Session, device: Device) -> dict:
    """Return the software-only CoreControl Off state for this device.

    The marker lives in the audit trail, so no schema migration is required.
    In this mode Windows and the Mesh/CoreControl services remain running; the
    user session is disconnected and the machine is kept reachable so a later
    remote "Ligar" never depends on Wake-on-LAN, router settings or another
    PC on the LAN.
    """
    last = db.scalar(
        select(AuditLog)
        .where(
            AuditLog.device_id == device.id,
            AuditLog.action.in_([
                "power.managed_off.entered",
                "power.managed_off.exited",
            ]),
        )
        .order_by(desc(AuditLog.created_at), desc(AuditLog.id))
        .limit(1)
    )
    active = bool(last and last.action == "power.managed_off.entered")
    return {
        "active": active,
        "since": iso(as_utc(last.created_at)) if active and last else None,
    }


def device_effectively_online(db: Session, device: Device) -> bool:
    """Operator-facing state: managed-off machines appear desligado."""
    return bool(device_online(device) and not device_managed_off_state(db, device)["active"])


def device_power_currently_on(device: Device) -> bool:
    """Return the best server-side power state currently known.

    When MeshCentral is provisioned for the device, use its recently checked
    connection state because it continues to work while the CoreControl Agent
    itself is starting. Otherwise fall back to normal Agent heartbeat state.
    """
    mesh_ready = bool(
        settings.remote_enabled
        and meshcentral_client.provisioning_configured
        and device.mesh_node_id
    )
    checked_at = as_utc(device.remote_checked_at)
    mesh_recent = bool(
        checked_at
        and (utcnow() - checked_at).total_seconds() <= settings.remote_status_stale_seconds
    )
    if mesh_ready and checked_at and mesh_recent:
        return bool(device.remote_online)
    return device_online(device)


def device_power_pending_state(db: Session, device: Device, *, currently_on: bool | None = None) -> dict:
    """Persist UI transition state using the power audit trail.

    No schema migration is required: a successful power dispatch already
    creates ``power.wake.sent`` / ``power.off.sent`` in ``audit_logs``.  The
    most recent dispatch remains "pending" until MeshCentral observes the
    requested state or the conservative timeout expires. This makes
    "Ligando..." / "Desligando..." survive F5, another browser and page
    navigation.
    """
    last = db.scalar(
        select(AuditLog)
        .where(
            AuditLog.device_id == device.id,
            AuditLog.action.in_([
                "power.wake.pending", "power.wake.sent", "power.wake.failed",
                "power.off.pending", "power.off.sent", "power.off.failed",
            ]),
        )
        .order_by(desc(AuditLog.created_at), desc(AuditLog.id))
        .limit(1)
    )
    empty = {
        "pending_action": None,
        "pending_since": None,
        "pending_expires_at": None,
        "pending_seconds_remaining": 0,
    }
    if not last:
        return empty

    if last.action.endswith(".failed"):
        return empty
    action = "wake" if ".wake." in last.action else "off"

    # CoreControl Off é um estado lógico: o PC permanece fisicamente online
    # para garantir religamento por software. Assim que o marcador de managed
    # off foi gravado, a transição de desligamento já terminou mesmo que o
    # MeshCentral continue conectado.
    managed_off = device_managed_off_state(db, device)["active"]
    if action == "off" and managed_off:
        return empty
    sent_at = as_utc(last.created_at)
    if sent_at is None:
        return empty

    if currently_on is None:
        currently_on = device_power_currently_on(device)

    # The requested state has already been observed, so this transition is done.
    if action == "wake" and currently_on:
        return empty
    if action == "off" and not currently_on:
        return empty

    # If MeshCentral saw this machine after a wake request, the wake did work at
    # least once. Do not resurrect an old "Ligando..." if the PC was switched
    # off again manually a moment later.
    if action == "wake":
        remote_last_seen = as_utc(device.remote_last_seen)
        if remote_last_seen and remote_last_seen > sent_at:
            return empty

    ttl = POWER_WAKE_PENDING_FOR if action == "wake" else POWER_OFF_PENDING_FOR
    expires_at = sent_at + ttl
    remaining = int(max(0, (expires_at - utcnow()).total_seconds()))
    if remaining <= 0:
        return empty

    return {
        "pending_action": action,
        "pending_since": iso(sent_at),
        "pending_expires_at": iso(expires_at),
        "pending_seconds_remaining": remaining,
    }


def device_power_readiness(db: Session, device: Device) -> dict:
    target_info = device_wol_info(db, device)
    relays = find_wake_relays(db, device)
    wan_route = latest_wan_wake_route(db, device)
    mesh_fallback = bool(
        settings.remote_enabled
        and meshcentral_client.provisioning_configured
        and device.mesh_node_id
    )

    # "PC preparado" e "rota confirmada" são coisas diferentes. O Agent pode
    # habilitar/validar a placa local, mas um PC totalmente desligado ainda
    # precisa receber o Magic Packet pela rede ou possuir gerenciamento
    # out-of-band realmente provisionado. Não marcamos isso como garantido só
    # porque o MeshCentral aceita o comando --wake.
    pc_wol_prepared = bool(target_info.get("windows_prepared"))
    amt_detected = bool(target_info.get("intel_amt_detected"))
    wake_verified = bool(relays or wan_route.get("verified"))
    wan_method = str(wan_route.get("method") or "").strip()
    windows_device = "windows" in str(device.os_name or "").lower()
    managed_off = device_managed_off_state(db, device)
    managed_mode_available = bool(windows_device and mesh_fallback)

    # Wake-on-LAN continua disponível como recurso adicional para máquinas que
    # realmente podem desligar em S5. Porém o modo padrão software-only não
    # depende disso: o PC permanece alcançável e o operador vê estado desligado.
    full_shutdown_safe = bool(relays or (wan_route.get("verified") and wan_method == "upnp_broadcast"))
    wake_available = bool(managed_off["active"] and managed_mode_available) or bool(wake_verified)
    off_available = bool(managed_mode_available or mesh_fallback)
    safe_to_power_off = bool(managed_mode_available or (off_available and (wake_verified or not settings.power_require_verified_wake)))

    if managed_mode_available:
        reason = (
            "Modo CoreControl Off disponível. Este PC pode ser colocado em estado desligado pelo painel "
            "sem depender de Wake-on-LAN, roteador, IP público ou outro computador na rede."
        )
    elif not target_info.get("mac_address"):
        reason = "O Agent ainda não informou o endereço MAC deste computador."
    elif not target_info.get("capability_checked"):
        reason = "Aguardando o Agent 0.9.7 concluir o diagnóstico automático de Wake-on-LAN."
    elif not pc_wol_prepared:
        reason = target_info.get("capability_reason") or "A placa de rede ainda não ficou preparada para Wake-on-LAN no Windows."
    elif relays:
        reason = "PC preparado e existe uma rota Wake-on-LAN verificada dentro da rede local."
    elif wan_route.get("verified"):
        if wan_method == "mesh_upnp_unicast":
            reason = (
                "PC preparado e a VPS confirmou uma rota externa direta pelo roteador. "
                "Para não depender de outro PC na rede, o CoreControl usará modo seguro/hibernação ao desligar."
            )
        else:
            reason = (
                "PC preparado e a VPS confirmou Wake-on-WAN por broadcast. "
                "Este computador pode ser desligado totalmente e ligado novamente sem depender de outro PC na rede."
            )
    elif amt_detected:
        reason = (
            "O PC parece possuir Intel AMT/vPro e está preparado para WOL, mas o CoreControl ainda não confirmou o gerenciamento "
            "out-of-band deste equipamento. O desligamento permanece protegido até essa rota ser provisionada."
        )
    elif target_info.get("link_type") == "wifi":
        reason = (
            "O Windows está preparado para wake, mas este PC está usando Wi-Fi. Wake após desligamento total por Wi-Fi depende do "
            "hardware/firmware e ainda não há uma rota externa confirmada."
        )
    elif target_info.get("s5_driver_hint"):
        reason = (
            "O PC está preparado para Magic Packet e o driver anuncia recurso relacionado a wake após desligamento, mas ainda não "
            "existe uma rota externa confirmada para entregar o pacote quando este for o único PC ligado na rede."
        )
    else:
        reason = (
            "O PC está preparado para Wake-on-LAN no Windows, mas ainda não existe uma rota externa confirmada para entregar o "
            "Magic Packet depois que ele ficar totalmente desligado."
        )

    pending = device_power_pending_state(db, device, currently_on=device_power_currently_on(device))

    return {
        "mac_known": bool(target_info.get("mac_address")),
        "network_cidr": target_info.get("network_cidr") or None,
        "adapter_name": target_info.get("adapter_name"),
        "interface_description": target_info.get("interface_description"),
        "link_type": target_info.get("link_type") or "unknown",
        "capability_checked": bool(target_info.get("capability_checked")),
        "capability_checked_at": target_info.get("capability_checked_at"),
        "magic_packet_supported": bool(target_info.get("magic_packet_supported")),
        "magic_packet_enabled": bool(target_info.get("magic_packet_enabled")),
        "wake_programmable": bool(target_info.get("wake_programmable")),
        "wake_armed": bool(target_info.get("wake_armed")),
        "pc_wol_prepared": pc_wol_prepared,
        "s5_driver_hint": bool(target_info.get("s5_driver_hint")),
        "intel_amt_detected": amt_detected,
        "auto_configured": bool(target_info.get("auto_configured")),
        "firmware_needs_check": bool(target_info.get("firmware_needs_check")),
        "capability_reason": target_info.get("capability_reason") or None,
        "capability_error": target_info.get("capability_error") or None,
        "relay_available": bool(relays),
        "relay_count": len(relays),
        "relay_names": [relay.name for relay in relays[:5]],
        "wan_route_verified": bool(wan_route.get("verified")),
        "wan_route_status": wan_route.get("status") or "idle",
        "wan_route_message": wan_route.get("message"),
        "wan_route_method": wan_route.get("method"),
        "wan_route_verified_at": wan_route.get("verified_at"),
        "mesh_fallback": mesh_fallback,
        "wake_available": wake_available,
        "wake_verified": wake_verified,
        "full_shutdown_safe": full_shutdown_safe,
        "managed_mode_available": managed_mode_available,
        "managed_off_active": bool(managed_off["active"]),
        "managed_off_since": managed_off.get("since"),
        "software_only_power": managed_mode_available,
        "power_off_mode": "managed" if managed_mode_available else ("shutdown" if full_shutdown_safe else ("hibernate" if wake_verified else "blocked")),
        "off_available": off_available,
        "safe_to_power_off": safe_to_power_off,
        "requires_verified_wake": settings.power_require_verified_wake,
        "reason": reason,
        **pending,
    }


def remote_state(device: Device, sample: Telemetry | None) -> dict:
    extra = sample_extra(sample)
    checked_at = as_utc(device.remote_checked_at)
    verified_recently = bool(
        checked_at
        and (utcnow() - checked_at).total_seconds() <= settings.remote_status_stale_seconds
    )
    mesh_connected = bool(device.mesh_node_id and device.remote_online and verified_recently)
    # Uma conexão ativa no MeshCentral é evidência mais forte que uma amostra de
    # telemetria anterior: se o nó está conectado, o Mesh Agent está instalado e
    # rodando agora. Isso evita o falso "Não instalado" logo após o Setup.
    installed = bool(extra.get("remote_agent_installed") or device.mesh_node_id)
    running = bool(extra.get("remote_agent_running") or mesh_connected)
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
    actual_online = device_online(device)
    managed_off = device_managed_off_state(db, device)
    online = bool(actual_online and not managed_off["active"])
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
        "actual_online": actual_online,
        "managed_off": bool(managed_off["active"]),
        "health_score": health_score(sample, online),
        "alerts_open": int(open_alerts),
        "telemetry": serialize_sample(sample),
        "remote": remote_state(device, sample),
        "power": device_power_readiness(db, device),
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
            AuditLog.action.like("power.%"),
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
    online = sum(1 for device in devices if device_effectively_online(db, device))
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
        online = sum(1 for d in devices if d.active and device_effectively_online(db, d))
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


@router.post("/devices/{device_id}/reinstall-token")
def create_device_reinstall_token(
    device_id: int,
    user: CurrentUser,
    db: Db,
    valid_minutes: int = 30,
):
    """Create a single-use installation authorization bound to one device.

    The frontend has exposed this action for a while; keeping the authorization
    tied to the device UID prevents a reinstall link copied to another PC from
    enrolling a second machine by mistake.
    """
    require_roles(user, "platform_admin", "company_admin", "technician")
    device = db.get(Device, device_id)
    if not device:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    if not device.active:
        raise HTTPException(status_code=409, detail="Computador está desativado")
    if valid_minutes not in VALID_ENROLLMENT_MINUTES:
        raise HTTPException(status_code=422, detail="Validade permitida: 30 minutos, 2 horas ou 24 horas")

    company = db.get(Company, device.company_id)
    if not company or not company.active:
        raise HTTPException(status_code=404, detail="Empresa não encontrada ou desativada")

    # Only the newest unused reinstall authorization for a device remains valid.
    db.execute(
        delete(EnrollmentToken).where(
            EnrollmentToken.device_id == device.id,
            EnrollmentToken.used_at.is_(None),
        )
    )

    raw = f"ctenr_{new_secret(32)}"
    install_code = new_install_code(db)
    expires = utcnow() + timedelta(minutes=valid_minutes)
    db.add(
        EnrollmentToken(
            company_id=device.company_id,
            device_id=device.id,
            token_hash=sha256_text(raw),
            code_hash=sha256_text(install_code),
            expires_at=expires,
            created_by=user.id,
        )
    )
    db.add(
        AuditLog(
            company_id=device.company_id,
            actor_user_id=user.id,
            device_id=device.id,
            action="agent.reinstall_token.create",
            details=(
                f"Código/link de reinstalação de uso único válido por {valid_minutes} minutos "
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
        "reinstall": True,
        "device_id": device.id,
        "device_name": device.name,
    }


@router.get("/enrollment/{credential}/info")
def enrollment_info(credential: str, db: Db):
    enrollment, company = get_valid_enrollment(db, credential)
    device = db.get(Device, enrollment.device_id) if enrollment.device_id is not None else None
    return {
        "ok": True,
        "company_id": company.id,
        "company_name": company.name,
        "expires_at": iso(enrollment.expires_at),
        "single_use": True,
        "reinstall": enrollment.device_id is not None,
        "device_id": enrollment.device_id,
        "device_name": device.name if device else None,
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
    managed_off = device_managed_off_state(db, device)
    actual_on = device_power_currently_on(device)
    return {
        "ok": True,
        "device_id": device.id,
        "hostname": device.hostname,
        "mesh_connected": state["mesh_connected"],
        "mesh_node_id": device.mesh_node_id,
        "service_running": state["running"],
        "available": state["available"],
        "checked_at": state["checked_at"],
        "managed_off_active": bool(managed_off["active"]),
        "power_on": bool(actual_on and not managed_off["active"]),
        "actual_power_reachable": bool(actual_on),
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

    # Antes de cada sessão, reconcilia o grupo e os direitos do usuário
    # técnico. Assim uma empresa provisionada por versões antigas não fica
    # presa em RemoteViewOnly e não exige reinstalar o Agent no PC.
    company = db.get(Company, device.company_id)
    if not company:
        raise HTTPException(status_code=404, detail="Empresa do computador não encontrada")
    try:
        mesh_id, _mesh_hex, group_name = meshcentral_client.ensure_company_group(company)
        if company.mesh_group_id != mesh_id or company.mesh_group_name != group_name:
            company.mesh_group_id = mesh_id
            company.mesh_group_name = group_name
            company.mesh_group_synced_at = utcnow()
            db.commit()
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


@router.get("/devices/{device_id}/power-readiness")
def device_power_readiness_endpoint(device_id: int, user: CurrentUser, db: Db):
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    return {"device_id": device.id, **device_power_readiness(db, device)}


@router.post("/devices/{device_id}/wake-route-test")
def test_device_wake_route(device_id: int, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin", "technician")
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)
    if settings.remote_enabled and meshcentral_client.provisioning_configured and device.mesh_node_id:
        try:
            refresh_remote_for_devices(db, [device], force=True, suppress_errors=False)
        except MeshCentralCommandError:
            pass
    if not device_power_currently_on(device):
        raise HTTPException(status_code=409, detail="O computador precisa estar ligado para testar a rota de ligamento.")

    target_info = device_wol_info(db, device)
    if not target_info.get("windows_prepared"):
        raise HTTPException(
            status_code=409,
            detail=target_info.get("capability_reason") or "Prepare o Wake-on-LAN deste computador antes de testar a rota externa.",
        )

    # v10.28: primeiro tenta a rota mais compatível para clientes com um único
    # PC. O Mesh Agent configura UPnP para o IPv4 real da máquina e a VPS prova
    # a rota de fora para dentro antes de marcá-la como válida.
    route, route_error = _try_mesh_wan_route(
        db,
        device,
        created_by=user.id,
        target_info=target_info,
    )
    if route:
        db.add(
            AuditLog(
                company_id=device.company_id,
                actor_user_id=user.id,
                device_id=device.id,
                action="power.route_verified",
                details=json.dumps(
                    {
                        "method": route.get("method"),
                        "external_port": route.get("external_port"),
                        "verified_at": route.get("verified_at"),
                    },
                    ensure_ascii=False,
                ),
            )
        )
        db.commit()
        route_method = str(route.get("method") or "").strip()
        route_message = (
            "Wake-on-WAN por broadcast confirmado. Este computador pode ser desligado totalmente e ligado novamente sem depender de outro PC na rede."
            if route_method == "upnp_broadcast"
            else "Rota Wake-on-WAN direta confirmada. O CoreControl usará modo seguro ao desligar para que este PC possa ser ligado novamente sem depender de outro computador na rede."
        )
        return {
            "ok": True,
            "created": True,
            "status": "verified",
            "verified": True,
            "method": route_method,
            "message": route_message,
        }

    # Segunda implementação automática: usa o próprio CoreControl Agent da
    # máquina alvo para descobrir o roteador via SSDP/SOAP e pede à VPS que
    # confirme a rota de fora para dentro. Continua sem depender de outro PC.
    agent_route, agent_route_error = _try_agent_wan_route_sync(
        db,
        device,
        created_by=user.id,
        target_info=target_info,
    )
    if agent_route:
        db.add(
            AuditLog(
                company_id=device.company_id,
                actor_user_id=user.id,
                device_id=device.id,
                action="power.route_verified",
                details=json.dumps(
                    {
                        "method": agent_route.get("method"),
                        "external_port": agent_route.get("external_port"),
                        "verified_at": agent_route.get("verified_at"),
                        "preparer": "corecontrol_agent",
                    },
                    ensure_ascii=False,
                ),
            )
        )
        db.commit()
        return {
            "ok": True,
            "created": True,
            "status": "verified",
            "verified": True,
            "method": agent_route.get("method"),
            "message": "Rota Wake-on-WAN confirmada pela VPS usando o próprio PC. Não depende de outro computador na rede.",
        }

    detail = agent_route_error or route_error or "Não foi possível confirmar uma rota externa de Wake-on-WAN."
    raise HTTPException(status_code=409, detail=detail)


@router.post("/devices/{device_id}/power")
def control_device_power(device_id: int, action: str, user: CurrentUser, db: Db):
    require_roles(user, "platform_admin", "company_admin", "technician")
    device = db.get(Device, device_id)
    if not device or not device.active:
        raise HTTPException(status_code=404, detail="Computador não encontrado")
    assert_device_access(user, device)

    requested = (action or "").strip().lower()
    if requested not in {"wake", "off"}:
        raise HTTPException(status_code=400, detail="Ação de energia inválida")

    mesh_ready = bool(
        settings.remote_enabled
        and meshcentral_client.provisioning_configured
        and device.mesh_node_id
    )
    if mesh_ready:
        try:
            refresh_remote_for_devices(db, [device], force=True, suppress_errors=False)
        except MeshCentralCommandError as exc:
            if requested == "off":
                raise HTTPException(status_code=503, detail=f"MeshCentral indisponível: {exc}") from exc

    currently_on = device_power_currently_on(device)
    managed_state = device_managed_off_state(db, device)
    managed_off_active = bool(managed_state["active"])
    existing_pending = device_power_pending_state(db, device, currently_on=currently_on)
    is_retry = bool(requested == "wake" and existing_pending.get("pending_action") == "wake" and not managed_off_active)

    # Desligamento não deve ser disparado duas vezes. Wake é diferente: Magic
    # Packet é idempotente e pode precisar de novas tentativas até a placa acordar.
    if requested == "off" and existing_pending.get("pending_action") == "off":
        return {
            "ok": True,
            "device_id": device.id,
            "device_name": device.name,
            "action": requested,
            "status": "pending",
            **existing_pending,
            "message": "O desligamento já está em andamento.",
        }

    if requested == "off" and managed_off_active:
        return {
            "ok": True,
            "device_id": device.id,
            "device_name": device.name,
            "action": requested,
            "status": "off",
            "methods": ["corecontrol_managed_off"],
            "wake_verified": True,
            "managed_off_active": True,
            "message": "O computador já está desligado pelo CoreControl.",
        }
    if requested == "off" and not currently_on:
        raise HTTPException(status_code=409, detail="O computador já aparece desligado/offline.")
    if requested == "wake" and currently_on and not managed_off_active:
        return {
            "ok": True,
            "device_id": device.id,
            "device_name": device.name,
            "action": requested,
            "status": "online",
            "methods": [],
            "wake_verified": True,
            "managed_off_active": False,
            "message": "O computador já está online.",
        }

    readiness = device_power_readiness(db, device)
    wan_route = latest_wan_wake_route(db, device)

    # Antes de desligar um PC que pode estar sozinho na rede, tente criar ou
    # melhorar a rota Wake-on-WAN enquanto a máquina AINDA está ligada. Mesmo
    # quando já existe unicast verificado, tentamos promover para broadcast,
    # pois broadcast confirmado é seguro para desligamento total/S5.
    route_preflight_error: str | None = None
    if (
        requested == "off"
        and "windows" in str(device.os_name or "").lower()
        and readiness.get("pc_wol_prepared")
        and not readiness.get("managed_mode_available")
        and not readiness.get("full_shutdown_safe")
        and mesh_ready
    ):
        preflight_info = device_wol_info(db, device)
        auto_route, mesh_route_error = _try_mesh_wan_route(
            db,
            device,
            created_by=user.id,
            target_info=preflight_info,
        )

        # HNetCfg.NATUPnP não funciona em todos os roteadores/Windows. Se ele
        # falhar, tente automaticamente a implementação SSDP/SOAP do Agent da
        # PRÓPRIA máquina antes de bloquear o desligamento. Nenhum relay ou
        # segundo computador dentro da LAN é necessário.
        if not auto_route:
            auto_route, agent_route_error = _try_agent_wan_route_sync(
                db,
                device,
                created_by=user.id,
                target_info=preflight_info,
            )
            route_preflight_error = agent_route_error or mesh_route_error

        if auto_route:
            db.commit()
            readiness = device_power_readiness(db, device)
            wan_route = latest_wan_wake_route(db, device)
            route_preflight_error = None
        elif not route_preflight_error:
            route_preflight_error = mesh_route_error

    methods: list[str] = []
    relay_ids: list[int] = []

    # Só o primeiro despacho cria o marcador que sustenta "Ligando..." após F5.
    # Retentativas de Wake não reiniciam o relógio de 5 minutos.
    if not is_retry:
        db.add(
            AuditLog(
                company_id=device.company_id,
                actor_user_id=user.id,
                device_id=device.id,
                action=f"power.{requested}.pending",
                details=json.dumps(
                    {
                        "hostname": device.hostname,
                        "mesh_node_id": device.mesh_node_id,
                        "requested_action": requested,
                        "phase": "dispatching",
                    },
                    ensure_ascii=False,
                ),
            )
        )
        db.commit()

    try:
        if requested == "off":
            if not mesh_ready:
                raise HTTPException(
                    status_code=503,
                    detail="O desligamento remoto exige o vínculo MeshCentral deste computador.",
                )

            # Nunca coloque o único PC da rede em um estado do qual o próprio
            # CoreControl não saiba trazê-lo de volta. Se a política exige rota
            # verificada, bloqueie antes de enviar qualquer comando de energia.
            if settings.power_require_verified_wake and not readiness.get("safe_to_power_off"):
                detail_reason = route_preflight_error or str(
                    readiness.get("reason") or "Não foi possível confirmar a rota Wake-on-WAN enquanto o computador estava ligado."
                )
                raise HTTPException(
                    status_code=409,
                    detail=(
                        "Desligamento bloqueado por segurança após tentar automaticamente as rotas disponíveis. "
                        + detail_reason
                    ),
                )

            if "windows" in str(device.os_name or "").lower():
                if readiness.get("managed_mode_available"):
                    # Modo padrão do produto para instalação zero-config: não
                    # desliga eletricamente o Windows. Mantém apenas os serviços
                    # de gerenciamento vivos e desconecta a sessão do usuário.
                    # Assim o botão Ligar funciona de forma determinística sem
                    # roteador, WOL, CGNAT, BIOS ou outro computador na LAN.
                    try:
                        meshcentral_client.device_enter_managed_off(device.mesh_node_id)
                        methods.append("corecontrol_managed_off")
                        db.add(
                            AuditLog(
                                company_id=device.company_id,
                                actor_user_id=user.id,
                                device_id=device.id,
                                action="power.managed_off.entered",
                                details=json.dumps(
                                    {
                                        "hostname": device.hostname,
                                        "mesh_node_id": device.mesh_node_id,
                                        "mode": "software_only",
                                    },
                                    ensure_ascii=False,
                                ),
                            )
                        )
                        db.commit()
                    except MeshCentralCommandError as exc:
                        raise HTTPException(
                            status_code=503,
                            detail=f"Não foi possível colocar o computador em CoreControl Off: {exc}",
                        ) from exc
                elif readiness.get("full_shutdown_safe"):
                    # Caminho legado opcional para ambientes onde S5 realmente
                    # foi comprovado por uma rota de wake fora do Windows.
                    shutdown_error: MeshCentralCommandError | None = None
                    try:
                        meshcentral_client.device_shutdown_for_wol(device.mesh_node_id)
                        methods.append("meshcentral_windows_wol_shutdown")
                    except MeshCentralCommandError as exc:
                        shutdown_error = exc

                    if not methods:
                        try:
                            meshcentral_client.device_power(device.mesh_node_id, "off")
                            methods.append("meshcentral_off_fallback")
                        except MeshCentralCommandError as exc:
                            detail = shutdown_error or exc
                            raise HTTPException(status_code=503, detail=f"Não foi possível desligar o Windows: {detail}") from exc
                else:
                    try:
                        meshcentral_client.device_hibernate_for_wol(device.mesh_node_id)
                        methods.append("meshcentral_windows_wol_hibernate")
                    except MeshCentralCommandError as exc:
                        raise HTTPException(
                            status_code=503,
                            detail=f"Não foi possível colocar o Windows no modo seguro para religamento: {exc}",
                        ) from exc
            else:
                try:
                    meshcentral_client.device_power(device.mesh_node_id, "off")
                    methods.append("meshcentral_off")
                except MeshCentralCommandError as exc:
                    raise HTTPException(status_code=503, detail=f"Não foi possível enviar o comando de energia: {exc}") from exc
        else:
            if managed_off_active:
                if not mesh_ready:
                    raise HTTPException(
                        status_code=503,
                        detail="O CoreControl Off perdeu o vínculo remoto deste computador.",
                    )
                try:
                    meshcentral_client.device_exit_managed_off(device.mesh_node_id)
                    methods.append("corecontrol_managed_on")
                    db.add(
                        AuditLog(
                            company_id=device.company_id,
                            actor_user_id=user.id,
                            device_id=device.id,
                            action="power.managed_off.exited",
                            details=json.dumps(
                                {
                                    "hostname": device.hostname,
                                    "mesh_node_id": device.mesh_node_id,
                                    "mode": "software_only",
                                },
                                ensure_ascii=False,
                            ),
                        )
                    )
                    db.commit()
                except MeshCentralCommandError as exc:
                    raise HTTPException(
                        status_code=503,
                        detail=f"Não foi possível ligar o computador pelo CoreControl Off: {exc}",
                    ) from exc
            else:
                target_info = device_wol_info(db, device)
                mac_address = target_info.get("mac_address") or ""

                # Mantém as rotas antigas apenas como fallback para um PC que
                # esteja fisicamente offline e não tenha entrado em managed off.
                # é que uma tentativa pendente não bloqueia novo Magic Packet.
                if mac_address and wan_route.get("verified"):
                    try:
                        _send_wan_magic_packet(wan_route, mac_address)
                        methods.append("corecontrol_wan_upnp")
                    except (OSError, ValueError):
                        pass

                relays = find_wake_relays(db, device) if mac_address else []
                for relay in relays[:3]:
                    queue_agent_command(
                        db,
                        relay,
                        "power.wake_peer",
                        {
                            "mac_address": mac_address,
                            "target_device_id": device.id,
                            "target_name": device.name,
                        },
                        created_by=user.id,
                        deduplicate=False,
                    )
                    relay_ids.append(relay.id)
                if relay_ids:
                    methods.append("corecontrol_lan_relay")

                # v10.27: não dependa apenas do CoreControl Agent do relay. Se outro
                # Mesh Agent estiver online na mesma LAN, execute o Magic Packet
                # diretamente nele. Isso cobre PCs antigos cujo Agent ainda não
                # iniciou sessão e redes onde DevicePower --wake não entrega o
                # broadcast de forma confiável.
                mesh_relay_ids: list[int] = []
                if mac_address:
                    for relay in find_mesh_wake_relays(db, device)[:3]:
                        try:
                            meshcentral_client.device_wake_via_peer(relay.mesh_node_id, mac_address)
                            mesh_relay_ids.append(relay.id)
                        except MeshCentralCommandError:
                            continue
                    if mesh_relay_ids:
                        methods.append("meshcentral_lan_relay")
                        for relay_id in mesh_relay_ids:
                            if relay_id not in relay_ids:
                                relay_ids.append(relay_id)

                if mesh_ready:
                    try:
                        meshcentral_client.device_power(device.mesh_node_id, "wake")
                        methods.append("meshcentral_wake")
                    except MeshCentralCommandError:
                        if not methods:
                            raise HTTPException(status_code=503, detail="Não foi possível enviar o Wake-on-LAN pelo MeshCentral.")

                if not methods:
                    raise HTTPException(
                        status_code=409,
                        detail=(
                            "Não existe uma rota disponível para Wake-on-LAN. "
                            + readiness["reason"]
                        ),
                    )
    except Exception as exc:
        # Falha de uma RETENTATIVA não encerra a tentativa original; o polling
        # pode continuar e tentar outra rota. Só o primeiro despacho falho limpa
        # o estado pendente.
        failed_action = "power.wake.retry_failed" if is_retry else f"power.{requested}.failed"
        db.add(
            AuditLog(
                company_id=device.company_id,
                actor_user_id=user.id,
                device_id=device.id,
                action=failed_action,
                details=json.dumps(
                    {
                        "hostname": device.hostname,
                        "requested_action": requested,
                        "error": str(getattr(exc, "detail", exc)),
                    },
                    ensure_ascii=False,
                ),
            )
        )
        db.commit()
        raise

    action_name = "power.wake.retry" if is_retry else ("power.wake.sent" if requested == "wake" else "power.off.sent")
    db.add(
        AuditLog(
            company_id=device.company_id,
            actor_user_id=user.id,
            device_id=device.id,
            action=action_name,
            details=json.dumps(
                {
                    "hostname": device.hostname,
                    "mesh_node_id": device.mesh_node_id,
                    "requested_action": requested,
                    "methods": methods,
                    "relay_device_ids": relay_ids,
                    "wake_verified": readiness["wake_verified"],
                    "full_shutdown_safe": readiness.get("full_shutdown_safe"),
                    "power_off_mode": readiness.get("power_off_mode"),
                    "retry": is_retry,
                },
                ensure_ascii=False,
            ),
        )
    )
    db.commit()
    pending = device_power_pending_state(db, device, currently_on=currently_on)

    if requested == "off":
        if "corecontrol_managed_off" in methods:
            message = (
                "Computador desligado pelo CoreControl. O serviço remoto permanece ativo em segundo plano para garantir que o botão Ligar funcione sem configuração de rede."
            )
        elif "meshcentral_windows_wol_hibernate" in methods:
            message = (
                "Modo seguro de energia enviado. O computador ficará aparente como desligado, mas continuará preparado "
                "para ser ligado pela rota Wake-on-WAN sem depender de outro PC na rede."
            )
        else:
            message = (
                "Comando para desligar enviado. A rota Wake-on-WAN para religamento está verificada e o CoreControl acompanhará "
                "até o computador ficar offline."
            )
    elif "corecontrol_managed_on" in methods:
        message = "Computador ligado pelo CoreControl. A máquina continua acessível sem depender de Wake-on-LAN ou do roteador."
    elif is_retry:
        message = "Novo Wake-on-LAN enviado. Continuando a aguardar o computador voltar online."
    elif relay_ids:
        message = "Wake-on-LAN enviado por um computador online da mesma rede e pelos fallbacks disponíveis. O CoreControl acompanhará até o computador voltar online."
    else:
        message = "Sinal para ligar enviado. O CoreControl acompanhará o computador até ele voltar online."

    return {
        "ok": True,
        "device_id": device.id,
        "device_name": device.name,
        "action": requested,
        "status": "pending" if pending.get("pending_action") else "sent",
        "methods": methods,
        "wake_verified": readiness["wake_verified"],
        "full_shutdown_safe": readiness.get("full_shutdown_safe"),
        "power_off_mode": readiness.get("power_off_mode"),
        "managed_off_active": "corecontrol_managed_off" in methods,
        **pending,
        "message": message,
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
        incoming_name = payload.name.strip()
        incoming_hostname = payload.hostname.strip()
        existing_name = (device.name or "").strip()
        existing_hostname = (device.hostname or "").strip()

        # Reinstalar/atualizar nunca deve apagar o nome amigável escolhido pela empresa.
        # O Setup costuma reenviar o hostname como nome padrão. Só substituímos o nome
        # existente quando o instalador recebeu explicitamente um nome amigável diferente.
        incoming_is_friendly = bool(incoming_name and incoming_hostname and incoming_name.casefold() != incoming_hostname.casefold())
        existing_is_friendly = bool(existing_name and existing_hostname and existing_name.casefold() != existing_hostname.casefold())
        if incoming_is_friendly or not existing_is_friendly:
            device.name = incoming_name or existing_name or incoming_hostname
        # caso contrário preserva device.name exatamente como foi definido no painel

        device.hostname = incoming_hostname
        if payload.sector is not None:
            device.sector = payload.sector
        if payload.location is not None:
            device.location = payload.location
        device.manufacturer = payload.manufacturer
        device.model = payload.model
        device.serial_number = payload.serial_number
        device.os_name = payload.os_name
        device.os_version = payload.os_version
        # A versão do Agent é atualizada pela telemetria do próprio Agent.
        # O instalador possui uma versão própria (ex.: 0.4.x) e não deve
        # sobrescrever a versão real já observada do CoreControlAgent.
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


def get_agent_device_by_secret(db: Session, authorization: str | None) -> Device:
    raw_secret = get_agent_secret(authorization)
    secret_hash = sha256_text(raw_secret)
    device = db.scalar(
        select(Device).where(
            Device.agent_secret_hash == secret_hash,
            Device.active.is_(True),
        )
    )
    if not device:
        raise HTTPException(status_code=401, detail="Agente não autorizado")
    return device


@router.post("/agent/enroll", status_code=201)
def agent_enroll(payload: EnrollmentRequest, db: Db):
    enrollment, company = get_valid_enrollment(db, payload.enrollment_token)
    now = utcnow()
    existing = db.scalar(
        select(Device).where(Device.company_id == enrollment.company_id, Device.device_uid == payload.device_uid)
    )

    if enrollment.device_id is not None:
        target = db.get(Device, enrollment.device_id)
        if not target or target.company_id != enrollment.company_id:
            raise HTTPException(status_code=410, detail="Autorização de reinstalação não é mais válida")
        if target.device_uid != payload.device_uid:
            raise HTTPException(
                status_code=409,
                detail="Este link/código de reinstalação pertence a outro computador",
            )
        existing = target
    raw_secret = f"ctagt_{new_secret(36)}"
    if existing:
        device = existing
        incoming_name = (payload.name or "").strip()
        incoming_hostname = (payload.hostname or "").strip()
        existing_name = (device.name or "").strip()
        existing_hostname = (device.hostname or "").strip()

        # Re-enrollment/atualização do Agent preserva o nome amigável cadastrado.
        incoming_is_friendly = bool(incoming_name and incoming_hostname and incoming_name.casefold() != incoming_hostname.casefold())
        existing_is_friendly = bool(existing_name and existing_hostname and existing_name.casefold() != existing_hostname.casefold())
        if incoming_is_friendly or not existing_is_friendly:
            device.name = incoming_name or existing_name or incoming_hostname
        device.hostname = incoming_hostname
        if payload.sector is not None:
            device.sector = payload.sector
        if payload.location is not None:
            device.location = payload.location
        device.manufacturer = payload.manufacturer
        device.model = payload.model
        device.serial_number = payload.serial_number
        device.os_name = payload.os_name
        device.os_version = payload.os_version
        # Preserve a versão real do Agent até a próxima telemetria do binário novo.
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

    # A autorização temporária vincula apenas este computador à empresa. Depois
    # do vínculo, o Agent recebe uma credencial própria e pode baixar somente o
    # agente remoto pertencente à mesma empresa. Isso permite que a instalação
    # por código configure diagnóstico + acesso remoto em um único fluxo, sem
    # expor login/senha da empresa e sem reutilizar o token de uso único.
    remote_agent, remote_warning = prepare_remote_install(db, company, device)
    if remote_agent is not None:
        remote_agent = dict(remote_agent)
        remote_agent["url"] = "/api/agent/remote-agent"

    return {
        "device_id": device.id,
        "agent_secret": raw_secret,
        "company_id": device.company_id,
        "company_name": company.name,
        "remote_agent": remote_agent,
        "remote_warning": remote_warning,
    }


@router.get("/agent/remote-agent")
def download_agent_remote_agent(
    db: Db,
    authorization: Annotated[str | None, Header()] = None,
):
    device = get_agent_device_by_secret(db, authorization)
    company = db.get(Company, device.company_id)
    if not company or not company.active:
        raise HTTPException(status_code=404, detail="Empresa não encontrada")
    if not settings.remote_enabled:
        raise HTTPException(status_code=503, detail="O acesso remoto está desativado no servidor")
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


@router.get("/agent/remote-status")
def get_agent_remote_status(
    db: Db,
    authorization: Annotated[str | None, Header()] = None,
):
    device = get_agent_device_by_secret(db, authorization)
    sync_error = None
    try:
        refresh_remote_for_devices(db, [device], force=True, suppress_errors=False)
    except MeshCentralCommandError as exc:
        sync_error = str(exc)
    sample = latest_telemetry(db, device.id)
    state = remote_state(device, sample)
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
