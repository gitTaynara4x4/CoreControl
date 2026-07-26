from __future__ import annotations

import base64
import binascii
import hashlib
import json
import os
import secrets
import subprocess
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path
from threading import Lock
from typing import Any
from urllib.parse import urlencode, urlparse, urlunparse

import httpx
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from .config import settings


class MeshCentralTokenError(RuntimeError):
    """Raised when the MeshCentral integration secret is invalid."""


class MeshCentralCommandError(RuntimeError):
    """Raised when MeshCtrl or the MeshCentral agent endpoint fails."""


@dataclass(frozen=True)
class MeshDevice:
    node_id: str
    mesh_id: str
    name: str
    real_name: str
    hostname: str
    connected: bool
    raw: dict[str, Any]


@dataclass(frozen=True)
class PreparedRemoteAgent:
    path: Path
    filename: str
    sha256: str
    size: int
    mesh_group_id: str
    mesh_group_hex: str
    mesh_group_name: str
    server_url: str


def _key_bytes(login_token_key: str) -> bytes:
    raw = (login_token_key or "").strip()
    if not raw:
        raise MeshCentralTokenError("A chave de login do MeshCentral não foi configurada.")
    try:
        key = bytes.fromhex(raw)
    except ValueError as exc:
        raise MeshCentralTokenError("A chave de login do MeshCentral precisa estar em hexadecimal.") from exc
    if len(key) < 32:
        raise MeshCentralTokenError("A chave de login do MeshCentral precisa ter pelo menos 32 bytes.")
    return key


def build_user_id(username: str, domain: str = "") -> str:
    clean_user = (username or "").strip().lower()
    clean_domain = (domain or "").strip().lower()
    if not clean_user:
        raise MeshCentralTokenError("O usuário de integração do MeshCentral não foi configurado.")
    if any(char in clean_user for char in ("/", "\\", "?", "#")):
        raise MeshCentralTokenError("O usuário de integração do MeshCentral é inválido.")
    if any(char in clean_domain for char in ("/", "\\", "?", "#")):
        raise MeshCentralTokenError("O domínio de integração do MeshCentral é inválido.")
    return f"user/{clean_domain}/{clean_user}"


def create_login_token(
    *,
    login_token_key: str,
    username: str,
    domain: str = "",
    expire_minutes: int = 2,
) -> str:
    """Create a MeshCentral AES-256-GCM login cookie compatible token."""
    key = _key_bytes(login_token_key)
    minutes = max(1, min(int(expire_minutes), 10))
    payload = {
        "u": build_user_id(username, domain),
        "a": 3,
        "expire": minutes,
        "once": secrets.token_urlsafe(24),
        "time": int(time.time()),
    }
    plaintext = json.dumps(payload, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    iv = secrets.token_bytes(12)
    encrypted = AESGCM(key[:32]).encrypt(iv, plaintext, None)
    ciphertext, tag = encrypted[:-16], encrypted[-16:]
    token = base64.b64encode(iv + tag + ciphertext).decode("ascii")
    return token.replace("+", "@").replace("/", "$")


def _short_node_id(node_id: str) -> str:
    value = (node_id or "").strip()
    if not value:
        raise MeshCentralTokenError("O computador não possui um identificador remoto válido.")
    if "/" in value:
        value = value.rsplit("/", 1)[-1]
    if not value or any(char in value for char in ("?", "#", "\\")):
        raise MeshCentralTokenError("O identificador remoto do computador é inválido.")
    return value


def build_remote_desktop_url(
    *,
    base_url: str,
    login_token: str,
    node_id: str,
) -> str:
    query = urlencode(
        {
            "login": login_token,
            "gotonode": _short_node_id(node_id),
            "viewmode": "11",
            "hide": "63",
        }
    )
    return f"{base_url.rstrip('/')}/?{query}"


def _mesh_id_to_hex(mesh_id: str) -> str:
    value = (mesh_id or "").strip()
    if not value:
        raise MeshCentralCommandError("O grupo remoto não possui identificador.")

    # O MeshCtrl pode devolver o identificador em três formatos:
    # 1) 0x + 64 caracteres hexadecimais;
    # 2) 64 caracteres hexadecimais sem prefixo quando usado --hex;
    # 3) mesh// + identificador Base64 modificado do MeshCentral.
    candidate = value.rsplit("/", 1)[-1]
    if candidate.lower().startswith("0x"):
        candidate = candidate[2:]
    if len(candidate) == 64 and all(ch in "0123456789abcdefABCDEF" for ch in candidate):
        return candidate.lower()

    encoded = candidate.replace("@", "+").replace("$", "/")
    encoded += "=" * ((4 - len(encoded) % 4) % 4)
    try:
        decoded = base64.b64decode(encoded, validate=True)
    except (ValueError, binascii.Error) as exc:
        raise MeshCentralCommandError("O identificador do grupo remoto é inválido.") from exc
    if len(decoded) != 32:
        raise MeshCentralCommandError("O identificador do grupo remoto não possui 32 bytes.")
    return decoded.hex()


def _safe_group_name(company_name: str, company_slug: str, company_id: int) -> str:
    label = " ".join((company_name or "").strip().split()) or company_slug or f"Empresa {company_id}"
    suffix = f" [{company_slug or company_id}-{company_id}]"
    prefix = "CoreTuner - "
    max_label = max(12, 160 - len(prefix) - len(suffix))
    return f"{prefix}{label[:max_label]}{suffix}"


def _json_from_output(output: str) -> Any:
    text = (output or "").strip()
    if not text:
        raise MeshCentralCommandError("O MeshCentral não retornou dados.")
    decoder = json.JSONDecoder()
    positions = [idx for idx, char in enumerate(text) if char in "[{"]
    for position in positions:
        try:
            value, _ = decoder.raw_decode(text[position:])
            return value
        except json.JSONDecodeError:
            continue
    raise MeshCentralCommandError("A resposta do MeshCentral não está em formato JSON válido.")


def _normalize_name(value: str | None) -> str:
    return " ".join((value or "").strip().lower().split())


class MeshCentralClient:
    def __init__(self) -> None:
        self._cache_lock = Lock()
        self._device_cache: dict[str, tuple[float, list[MeshDevice]]] = {}

    @property
    def provisioning_configured(self) -> bool:
        return settings.remote_provisioning_configured

    def _websocket_url(self) -> str:
        parsed = urlparse(settings.remote_url)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc:
            raise MeshCentralCommandError("CORETUNER_REMOTE_URL é inválida.")
        scheme = "wss" if parsed.scheme == "https" else "ws"
        path = parsed.path.rstrip("/")
        return urlunparse((scheme, parsed.netloc, path, "", "", ""))

    def _meshctrl_command(
        self,
        action: str,
        args: list[str] | None = None,
        *,
        login_user: str | None = None,
        timeout: int | None = None,
    ) -> str:
        if not self.provisioning_configured:
            raise MeshCentralCommandError(
                "A automação remota exige CORETUNER_REMOTE_ADMIN_USER e o MeshCtrl instalado no servidor."
            )
        meshctrl_path = Path(settings.remote_meshctrl_path)
        if not meshctrl_path.is_file():
            raise MeshCentralCommandError(
                f"MeshCtrl não encontrado em {meshctrl_path}. Reimplante o serviço CoreTuner com o Dockerfile atualizado."
            )
        node_path = settings.remote_node_path or "node"
        auth_user = (login_user or settings.remote_admin_user).strip()
        if not auth_user:
            raise MeshCentralCommandError("CORETUNER_REMOTE_ADMIN_USER não foi configurado.")

        key = (settings.remote_login_token_key or "").strip()
        _key_bytes(key)
        with tempfile.TemporaryDirectory(prefix="coretuner-meshctrl-") as temporary:
            key_file = Path(temporary) / "login-key.txt"
            key_file.write_text(key, encoding="ascii")
            try:
                os.chmod(key_file, 0o600)
            except OSError:
                pass
            command = [
                node_path,
                str(meshctrl_path),
                action,
                "--url",
                self._websocket_url(),
                "--loginuser",
                auth_user,
                "--loginkeyfile",
                str(key_file),
            ]
            if settings.remote_login_domain:
                command.extend(["--logindomain", settings.remote_login_domain])
            command.extend(args or [])
            try:
                result = subprocess.run(
                    command,
                    capture_output=True,
                    text=True,
                    timeout=timeout or settings.remote_command_timeout_seconds,
                    check=False,
                    cwd=temporary,
                    env={**os.environ, "NO_COLOR": "1"},
                )
            except FileNotFoundError as exc:
                raise MeshCentralCommandError("O Node.js não está instalado no serviço CoreTuner.") from exc
            except subprocess.TimeoutExpired as exc:
                raise MeshCentralCommandError(f"O MeshCentral não respondeu ao comando {action} dentro do prazo.") from exc

        output = "\n".join(part.strip() for part in (result.stdout, result.stderr) if part and part.strip()).strip()
        lowered = output.lower()
        known_errors = (
            "invalid login",
            "authentication token required",
            "unable to connect",
            "download error",
            "missing module",
            "error:",
        )
        if result.returncode not in (0, None) or any(marker in lowered for marker in known_errors):
            safe_output = output.replace(key, "[chave oculta]")
            raise MeshCentralCommandError(safe_output or f"O comando {action} falhou.")
        return output

    def _list_users(self) -> list[dict[str, Any]]:
        output = self._meshctrl_command("ListUsers", ["--json"])
        value = _json_from_output(output)
        if not isinstance(value, list):
            raise MeshCentralCommandError("A lista de usuários do MeshCentral é inválida.")
        return [item for item in value if isinstance(item, dict)]

    def ensure_integration_user(self) -> str:
        expected_id = build_user_id(settings.remote_login_user, settings.remote_login_domain)
        users = self._list_users()
        for user in users:
            user_id = str(user.get("_id") or "")
            username = str(user.get("name") or user_id.rsplit("/", 1)[-1])
            if user_id == expected_id or username.lower() == settings.remote_login_user.lower():
                return user_id or expected_id

        args = [
            "--user",
            settings.remote_login_user,
            "--randompass",
            "--rights",
            "none",
            "--realname",
            "CoreTuner Integração",
        ]
        if settings.remote_login_domain:
            args.extend(["--domain", settings.remote_login_domain])
        self._meshctrl_command("AddUser", args)
        users = self._list_users()
        for user in users:
            user_id = str(user.get("_id") or "")
            if user_id == expected_id or user_id.rsplit("/", 1)[-1].lower() == settings.remote_login_user.lower():
                return user_id or expected_id
        raise MeshCentralCommandError("O usuário coretuner-integracao não pôde ser criado no MeshCentral.")

    def _list_groups(self) -> list[dict[str, Any]]:
        output = self._meshctrl_command("ListDeviceGroups", ["--json", "--hex"])
        value = _json_from_output(output)
        if not isinstance(value, list):
            raise MeshCentralCommandError("A lista de grupos do MeshCentral é inválida.")
        return [item for item in value if isinstance(item, dict)]

    def ensure_company_group(self, company: Any) -> tuple[str, str, str]:
        desired_name = _safe_group_name(company.name, company.slug, company.id)
        groups = self._list_groups()
        selected: dict[str, Any] | None = None
        saved_id = (getattr(company, "mesh_group_id", None) or "").strip()
        if saved_id:
            selected = next((group for group in groups if str(group.get("_id") or group.get("id") or "") == saved_id), None)
        if selected is None:
            selected = next((group for group in groups if str(group.get("name") or "") == desired_name), None)
        if selected is None:
            self._meshctrl_command(
                "AddDeviceGroup",
                [
                    "--name",
                    desired_name,
                    "--desc",
                    f"Grupo automático da empresa {company.name} no CoreTuner",
                    "--features",
                    str(settings.remote_group_features),
                    "--consent",
                    str(settings.remote_group_consent),
                ],
            )
            groups = self._list_groups()
            selected = next((group for group in groups if str(group.get("name") or "") == desired_name), None)
        if selected is None:
            raise MeshCentralCommandError("O grupo remoto da empresa não pôde ser criado.")

        mesh_id = str(selected.get("_id") or selected.get("id") or "").strip()
        mesh_hex = str(selected.get("_idhex") or selected.get("idhex") or "").strip()
        if mesh_hex.lower().startswith("0x"):
            mesh_hex = mesh_hex[2:]
        if len(mesh_hex) != 64:
            mesh_hex = _mesh_id_to_hex(mesh_id)
        group_name = str(selected.get("name") or desired_name)

        integration_user_id = self.ensure_integration_user()
        links = selected.get("links")
        already_linked = isinstance(links, dict) and integration_user_id in links
        if not already_linked:
            try:
                self._meshctrl_command(
                    "AddUserToDeviceGroup",
                    [
                        "--id",
                        mesh_id,
                        "--userid",
                        integration_user_id,
                        "--remotecontrol",
                        "--noterminal",
                        "--nofiles",
                        "--noregistry",
                        "--noamt",
                        "--limitedevents",
                    ],
                )
            except MeshCentralCommandError as exc:
                message = str(exc).lower()
                if "already" not in message and "já" not in message:
                    raise
        return mesh_id, mesh_hex.lower(), group_name

    def _agent_path(self, mesh_hex: str) -> Path:
        cache_root = Path(settings.remote_agent_cache_dir)
        cache_root.mkdir(parents=True, exist_ok=True)
        directory = cache_root / mesh_hex.lower()
        directory.mkdir(parents=True, exist_ok=True)
        return directory / settings.remote_agent_filename

    def _download_agent(self, mesh_hex: str, target: Path) -> None:
        endpoint = f"{settings.remote_url.rstrip('/')}/meshagents"
        params = {
            "id": settings.remote_agent_type,
            "meshid": mesh_hex,
            "installflags": settings.remote_agent_install_flags,
        }
        try:
            with httpx.Client(follow_redirects=True, timeout=settings.remote_agent_download_timeout_seconds) as client:
                response = client.get(endpoint, params=params)
                response.raise_for_status()
        except httpx.HTTPError as exc:
            raise MeshCentralCommandError(f"Falha ao baixar o agente dinâmico do MeshCentral: {exc}") from exc
        raw = response.content
        if len(raw) < 100_000 or raw[:2] != b"MZ":
            content_type = response.headers.get("content-type", "desconhecido")
            raise MeshCentralCommandError(
                f"O MeshCentral não retornou um executável Windows válido (tipo {content_type}, {len(raw)} bytes)."
            )
        temporary = target.with_suffix(target.suffix + ".tmp")
        temporary.write_bytes(raw)
        os.replace(temporary, target)

    def prepare_company_agent(self, company: Any) -> PreparedRemoteAgent:
        mesh_id, mesh_hex, group_name = self.ensure_company_group(company)
        target = self._agent_path(mesh_hex)
        refresh = True
        if target.is_file():
            age = time.time() - target.stat().st_mtime
            refresh = age > settings.remote_agent_cache_seconds
        if refresh:
            self._download_agent(mesh_hex, target)
        raw_header = target.read_bytes()[:2]
        if raw_header != b"MZ":
            target.unlink(missing_ok=True)
            raise MeshCentralCommandError("O agente remoto armazenado no servidor está corrompido.")
        digest = hashlib.sha256()
        with target.open("rb") as handle:
            for chunk in iter(lambda: handle.read(1024 * 1024), b""):
                digest.update(chunk)
        return PreparedRemoteAgent(
            path=target,
            filename=settings.remote_agent_filename,
            sha256=digest.hexdigest(),
            size=target.stat().st_size,
            mesh_group_id=mesh_id,
            mesh_group_hex=mesh_hex,
            mesh_group_name=group_name,
            server_url=settings.remote_url,
        )

    def list_group_devices(self, mesh_id: str, *, force: bool = False) -> list[MeshDevice]:
        now = time.monotonic()
        with self._cache_lock:
            cached = self._device_cache.get(mesh_id)
            if not force and cached and (now - cached[0]) <= settings.remote_status_cache_seconds:
                return list(cached[1])
        output = self._meshctrl_command("ListDevices", ["--id", mesh_id, "--json"])
        value = _json_from_output(output)
        if not isinstance(value, list):
            raise MeshCentralCommandError("A lista de computadores do MeshCentral é inválida.")
        devices: list[MeshDevice] = []
        for item in value:
            if not isinstance(item, dict):
                continue
            node_id = str(item.get("_id") or item.get("id") or "").strip()
            if not node_id:
                continue
            try:
                conn = int(item.get("conn") or 0)
            except (TypeError, ValueError):
                conn = 0
            devices.append(
                MeshDevice(
                    node_id=node_id,
                    mesh_id=str(item.get("meshid") or mesh_id),
                    name=str(item.get("name") or ""),
                    real_name=str(item.get("rname") or ""),
                    hostname=str(item.get("host") or item.get("hostname") or ""),
                    connected=conn > 0,
                    raw=item,
                )
            )
        with self._cache_lock:
            self._device_cache[mesh_id] = (now, list(devices))
        return devices

    @staticmethod
    def match_device(device: Any, remote_devices: list[MeshDevice]) -> MeshDevice | None:
        saved_node = (getattr(device, "mesh_node_id", None) or "").strip()
        if saved_node:
            for remote in remote_devices:
                if remote.node_id == saved_node:
                    return remote
        targets = {
            _normalize_name(getattr(device, "hostname", None)),
            _normalize_name(getattr(device, "name", None)),
        }
        targets.discard("")
        matches: list[MeshDevice] = []
        for remote in remote_devices:
            candidates = {
                _normalize_name(remote.name),
                _normalize_name(remote.real_name),
                _normalize_name(remote.hostname),
            }
            if targets.intersection(candidates):
                matches.append(remote)
        if len(matches) == 1:
            return matches[0]
        connected = [item for item in matches if item.connected]
        return connected[0] if len(connected) == 1 else None


meshcentral_client = MeshCentralClient()


__all__ = [
    "MeshCentralCommandError",
    "MeshCentralTokenError",
    "MeshDevice",
    "PreparedRemoteAgent",
    "build_remote_desktop_url",
    "build_user_id",
    "create_login_token",
    "meshcentral_client",
]
