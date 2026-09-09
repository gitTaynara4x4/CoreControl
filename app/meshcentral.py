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


def _remote_session_marker(node_id: str) -> str:
    """Carrega o nó no parâmetro que o MeshCentral preserva após o login.

    O MeshCentral atual consome ``gotonode`` e também pode descartar parâmetros
    desconhecidos como ``ctnode`` ao autenticar com ``login``. Nos testes reais,
    ``coretuner`` é preservado. Por isso ele transporta, de forma URL-safe, o
    identificador curto do nó. O ID do nó não é uma credencial.
    """
    short_id = _short_node_id(node_id)
    encoded = base64.urlsafe_b64encode(short_id.encode("utf-8")).decode("ascii").rstrip("=")
    return f"1_{encoded}"


def build_remote_desktop_url(
    *,
    base_url: str,
    login_token: str,
    node_id: str,
) -> str:
    short_id = _short_node_id(node_id)
    query = urlencode(
        {
            "login": login_token,
            # O próprio MeshCentral usa gotonode antes/durante o login.
            "gotonode": short_id,
            "viewmode": "11",
            "hide": "63",
            # Compatibilidade com versões anteriores. Pode ser removido pelo
            # MeshCentral durante o login, por isso não é mais a fonte principal.
            "ctnode": short_id,
            # Este parâmetro sobrevive ao login por token. O custom.js v10.5
            # extrai daqui o nó exato e nunca escolhe o computador apenas pelo nome.
            "coretuner": _remote_session_marker(node_id),
        }
    )
    return f"{base_url.rstrip('/')}/?{query}"


def _mesh_id_to_hex(mesh_id: str) -> str:
    value = (mesh_id or "").strip()
    if not value:
        raise MeshCentralCommandError("O grupo remoto não possui identificador.")

    # O MeshCentral atual gera identificadores de grupo com 48 bytes
    # (96 caracteres hexadecimais). Instalações antigas podem usar 32 bytes
    # (64 caracteres hexadecimais). O MeshCtrl pode devolver qualquer um dos
    # formatos abaixo, por isso aceitamos ambos os tamanhos:
    # 1) 0x + hexadecimal;
    # 2) hexadecimal sem prefixo quando usado --hex;
    # 3) mesh/<domínio>/<Base64 modificado do MeshCentral>.
    candidate = value.rsplit("/", 1)[-1]
    if candidate.lower().startswith("0x"):
        candidate = candidate[2:]
    if len(candidate) in {64, 96} and all(ch in "0123456789abcdefABCDEF" for ch in candidate):
        return candidate.lower()

    encoded = candidate.replace("@", "+").replace("$", "/")
    encoded += "=" * ((4 - len(encoded) % 4) % 4)
    try:
        decoded = base64.b64decode(encoded, validate=True)
    except (ValueError, binascii.Error) as exc:
        raise MeshCentralCommandError("O identificador do grupo remoto é inválido.") from exc
    if len(decoded) not in {32, 48}:
        raise MeshCentralCommandError(
            f"O identificador do grupo remoto possui {len(decoded)} bytes; eram esperados 32 ou 48 bytes."
        )
    return decoded.hex()


def _mesh_id_for_download(mesh_id: str) -> str:
    """Return the MeshCentral URL identifier (modified Base64), never the hex display ID."""
    value = (mesh_id or "").strip()
    if not value:
        raise MeshCentralCommandError("O grupo remoto não possui identificador para baixar o agente.")

    candidate = value.rsplit("/", 1)[-1].strip()
    if candidate.lower().startswith("0x"):
        candidate = candidate[2:]

    # ListDeviceGroups --hex exposes _idhex for display, but /meshagents expects
    # the original group identifier in MeshCentral's modified Base64 format.
    if len(candidate) in {64, 96} and all(ch in "0123456789abcdefABCDEF" for ch in candidate):
        raw = bytes.fromhex(candidate)
    else:
        encoded = candidate.replace("@", "+").replace("$", "/")
        encoded += "=" * ((4 - len(encoded) % 4) % 4)
        try:
            raw = base64.b64decode(encoded, validate=True)
        except (ValueError, binascii.Error) as exc:
            raise MeshCentralCommandError("O identificador do grupo remoto é inválido para baixar o agente.") from exc

    if len(raw) not in {32, 48}:
        raise MeshCentralCommandError(
            f"O identificador do grupo remoto possui {len(raw)} bytes; eram esperados 32 ou 48 bytes."
        )
    return base64.b64encode(raw).decode("ascii").rstrip("=").replace("+", "@").replace("/", "$")


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
                f"MeshCtrl não encontrado em {meshctrl_path}. Reimplante o serviço CoreControl com o Dockerfile atualizado."
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
                raise MeshCentralCommandError("O Node.js não está instalado no serviço CoreControl.") from exc
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

    def device_power(self, node_id: str, action: str) -> str:
        """Send a power action to a MeshCentral-managed device.

        ``wake`` asks MeshCentral to emit Wake-on-LAN through available agents
        on the same network (or another supported out-of-band path). ``off``
        requests a remote power off for an online device.
        """
        clean_node = (node_id or "").strip()
        if not clean_node:
            raise MeshCentralCommandError("O computador não possui identificador remoto para controle de energia.")
        flags = {
            "wake": "--wake",
            "off": "--off",
        }
        flag = flags.get((action or "").strip().lower())
        if not flag:
            raise MeshCentralCommandError("A ação de energia solicitada não é permitida.")
        return self._meshctrl_command(
            "DevicePower",
            [flag, "--id", clean_node],
            timeout=max(20, settings.remote_command_timeout_seconds),
        )

    def device_wake_via_peer(self, peer_node_id: str, mac_address: str) -> str:
        """Emit a Magic Packet from an online Windows Mesh Agent on the LAN.

        MeshCentral's DevicePower --wake is kept as a fallback, but some
        networks do not relay that packet reliably. Running this tiny
        PowerShell snippet on another online machine in the same subnet makes
        the delivery deterministic without requiring the CoreControl Agent on
        that relay machine to be currently reporting telemetry.
        """
        clean_node = (peer_node_id or "").strip()
        clean_mac = "".join(ch for ch in str(mac_address or "") if ch in "0123456789abcdefABCDEF")
        if not clean_node:
            raise MeshCentralCommandError("O computador relay não possui identificador remoto.")
        if len(clean_mac) != 12:
            raise MeshCentralCommandError("O endereço MAC para Wake-on-LAN é inválido.")
        script = (
            "$ErrorActionPreference='Stop';"
            f"$mac='{clean_mac.upper()}';"
            "$bytes=New-Object byte[] 102;"
            "0..5|ForEach-Object{$bytes[$_]=0xFF};"
            "$macBytes=0..5|ForEach-Object{[Convert]::ToByte($mac.Substring($_*2,2),16)};"
            "for($i=1;$i -le 16;$i++){[Array]::Copy($macBytes,0,$bytes,$i*6,6)};"
            "$udp=New-Object System.Net.Sockets.UdpClient;"
            "$udp.EnableBroadcast=$true;"
            "foreach($port in @(9,7)){"
            "$ep=New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Broadcast,$port);"
            "1..4|ForEach-Object{[void]$udp.Send($bytes,$bytes.Length,$ep);Start-Sleep -Milliseconds 120}"
            "};"
            "$udp.Close();'WOL_SENT'"
        )
        return self._meshctrl_command(
            "RunCommand",
            ["--id", clean_node, "--run", script, "--powershell"],
            timeout=min(max(12, settings.remote_command_timeout_seconds), 25),
        )

    def device_prepare_wan_wake_route(
        self,
        node_id: str,
        external_port: int,
        *,
        broadcast_ip: str | None = None,
    ) -> dict[str, Any]:
        """Create/refresh a UPnP UDP mapping for single-PC Wake-on-WAN.

        When ``broadcast_ip`` is supplied, CoreControl first asks the router to
        forward the public UDP port directly to the LAN broadcast address. That
        route does not depend on another PC being online and does not require an
        ARP entry for the sleeping/off target. If the router rejects directed
        broadcast mappings, the caller can retry without ``broadcast_ip`` and
        use the unicast/S4-safe fallback instead.
        """
        clean_node = (node_id or "").strip()
        if not clean_node:
            raise MeshCentralCommandError("O computador não possui identificador remoto.")
        port = int(external_port or 0)
        if port < 40000 or port > 59999:
            raise MeshCentralCommandError("A porta da rota Wake-on-WAN é inválida.")

        broadcast_text = str(broadcast_ip or "").strip()
        broadcast_b64 = base64.b64encode(broadcast_text.encode("utf-8")).decode("ascii")

        # HNetCfg.NATUPnP talks to the customer's router through the normal
        # Windows UPnP stack. Running it via Mesh Agent means route preparation
        # still works even if the CoreControl Agent command queue is unhealthy.
        script = (
            "$ErrorActionPreference='Stop';"
            "$ProgressPreference='SilentlyContinue';"
            "try{Set-Service -Name SSDPSRV -StartupType Manual -ErrorAction SilentlyContinue;Start-Service SSDPSRV -ErrorAction SilentlyContinue}catch{};"
            "try{Set-Service -Name upnphost -StartupType Manual -ErrorAction SilentlyContinue;Start-Service upnphost -ErrorAction SilentlyContinue}catch{};"
            "$cfg=@(Get-NetIPConfiguration -ErrorAction Stop | Where-Object {$_.IPv4DefaultGateway -and $_.NetAdapter.Status -eq 'Up'} | Select-Object -First 1);"
            "if(-not $cfg){throw 'Nenhuma interface IPv4 ativa com gateway padrão foi encontrada.'};"
            "$ip=[string]$cfg.IPv4Address.IPAddress;"
            "$prefix=[int]$cfg.IPv4Address.PrefixLength;"
            "if([string]::IsNullOrWhiteSpace($ip)){throw 'Não foi possível determinar o IPv4 local.'};"
            f"$broadcast=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('{broadcast_b64}')).Trim();"
            "$target=$ip;$method='mesh_upnp_unicast';$description='CoreControl Wake-on-WAN';"
            "if(-not [string]::IsNullOrWhiteSpace($broadcast)){"
            "try{$parsed=[System.Net.IPAddress]::Parse($broadcast);if($parsed.AddressFamily -ne [System.Net.Sockets.AddressFamily]::InterNetwork){throw 'broadcast inválido'}}catch{throw 'O endereço de broadcast calculado para Wake-on-WAN é inválido.'};"
            "$target=$broadcast;$method='upnp_broadcast';$description='CoreControl Wake-on-WAN Broadcast'"
            "};"
            "$adapter=[string]$cfg.NetAdapter.Name;"
            "try{Set-NetAdapterPowerManagement -Name $adapter -WakeOnMagicPacket Enabled -ErrorAction SilentlyContinue | Out-Null}catch{};"
            "try{Set-NetAdapterPowerManagement -Name $adapter -ArpOffload Enabled -ErrorAction SilentlyContinue | Out-Null}catch{};"
            "try{$desc=[string]$cfg.NetAdapter.InterfaceDescription;if($desc){& powercfg.exe /deviceenablewake $desc 2>$null | Out-Null}}catch{};"
            "$nat=New-Object -ComObject HNetCfg.NATUPnP;"
            "$maps=$nat.StaticPortMappingCollection;"
            "if($null -eq $maps){throw 'UPnP indisponível ou desativado no roteador.'};"
            f"$ext={port};$int={port};"
            "$existing=$null;"
            "try{$existing=@($maps | Where-Object {$_.ExternalPort -eq $ext -and $_.Protocol -eq 'UDP'}) | Select-Object -First 1}catch{};"
            "if($existing -and ([string]$existing.InternalClient -ne $target -or [int]$existing.InternalPort -ne $int)){try{$maps.Remove($ext,'UDP')}catch{};$existing=$null};"
            "if(-not $existing){try{$existing=$maps.Add($ext,'UDP',$int,$target,$true,$description)}catch{throw ('O roteador não aceitou a rota Wake-on-WAN para '+$target+'.')}};"
            "if($null -eq $existing){throw 'O roteador não aceitou a regra UPnP de Wake-on-WAN.'};"
            "$public='';try{$public=[string]$existing.ExternalIPAddress}catch{};"
            # Alguns roteadores criam a regra corretamente, mas o COM do
            # Windows devolve ExternalIPAddress vazio/0.0.0.0. Descubra o IP
            # visto pela internet e deixe a VPS provar a rota de verdade antes
            # de confiar nela. Em CGNAT o probe externo simplesmente falhará.
            "if([string]::IsNullOrWhiteSpace($public) -or $public -eq '0.0.0.0'){"
            "foreach($uri in @('https://api.ipify.org','https://checkip.amazonaws.com')){try{$candidate=[string](Invoke-RestMethod -UseBasicParsing -Uri $uri -TimeoutSec 5);$candidate=$candidate.Trim();if($candidate){$public=$candidate;break}}catch{}}"
            "};"
            "$obj=[PSCustomObject]@{ok=$true;method=$method;external_ip=$public;external_port=[int]$existing.ExternalPort;internal_port=[int]$existing.InternalPort;internal_ip=[string]$existing.InternalClient;broadcast_ip=$broadcast;prefix_length=$prefix;adapter=$adapter};"
            "$obj|ConvertTo-Json -Compress"
        )
        output = self._meshctrl_command(
            "RunCommand",
            ["--id", clean_node, "--run", script, "--powershell"],
            timeout=min(max(15, settings.remote_command_timeout_seconds), 30),
        )
        value = _json_from_output(output)
        if not isinstance(value, dict) or not bool(value.get("ok")):
            raise MeshCentralCommandError("O roteador não devolveu uma rota Wake-on-WAN utilizável.")
        return value

    def device_wait_for_wan_probe(self, node_id: str, internal_port: int, probe_token: str) -> dict[str, Any]:
        """Wait briefly on the target PC for a UDP probe sent by the VPS."""
        clean_node = (node_id or "").strip()
        token = str(probe_token or "").strip()
        port = int(internal_port or 0)
        if not clean_node:
            raise MeshCentralCommandError("O computador não possui identificador remoto.")
        if port < 40000 or port > 59999 or len(token) < 12 or len(token) > 160:
            raise MeshCentralCommandError("Os dados de validação Wake-on-WAN são inválidos.")
        # Token contains urlsafe characters only, nevertheless encode it as
        # base64 so no user-controlled text is interpolated into PowerShell.
        token_b64 = base64.b64encode(token.encode("utf-8")).decode("ascii")
        rule = f"CoreControl Wake Probe {port}"
        script = (
            "$ErrorActionPreference='Stop';"
            f"$port={port};"
            f"$expected=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('{token_b64}'));"
            f"$rule='{rule}';"
            "try{& netsh advfirewall firewall delete rule name=$rule 2>$null | Out-Null}catch{};"
            "try{& netsh advfirewall firewall add rule name=$rule dir=in action=allow protocol=UDP localport=$port profile=any | Out-Null}catch{};"
            "$udp=$null;$received=$false;$remoteText='';"
            "try{"
            "$udp=New-Object System.Net.Sockets.UdpClient($port);"
            "$udp.Client.ReceiveTimeout=12000;"
            "$remote=New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any,0);"
            "$bytes=$udp.Receive([ref]$remote);"
            "$text=[Text.Encoding]::UTF8.GetString($bytes);"
            "if($text -eq $expected){$received=$true;$remoteText=[string]$remote.Address}"
            "}catch{}finally{if($udp){$udp.Close()};try{& netsh advfirewall firewall delete rule name=$rule 2>$null | Out-Null}catch{}};"
            "$obj=[PSCustomObject]@{received=$received;remote=$remoteText};$obj|ConvertTo-Json -Compress"
        )
        output = self._meshctrl_command(
            "RunCommand",
            ["--id", clean_node, "--run", script, "--powershell"],
            timeout=min(max(18, settings.remote_command_timeout_seconds), 28),
        )
        value = _json_from_output(output)
        if not isinstance(value, dict):
            raise MeshCentralCommandError("A confirmação Wake-on-WAN retornou resposta inválida.")
        return value

    def device_shutdown_for_wol(self, node_id: str) -> str:
        """Ask Windows to shut down and require an execution acknowledgement.

        MeshCtrl ``RunCommand`` without ``--reply`` only confirms that the
        command was accepted by MeshCentral; it does *not* prove that the
        remote Windows host executed it.  Power actions must therefore request
        the remote reply and validate an explicit marker emitted only after
        ``shutdown.exe`` itself returns success.
        """
        clean_node = (node_id or "").strip()
        if not clean_node:
            raise MeshCentralCommandError("O computador não possui identificador remoto para controle de energia.")
        marker = "CORECONTROL_SHUTDOWN_CONFIRMED"
        script = (
            "$ErrorActionPreference='Stop';"
            "$ProgressPreference='SilentlyContinue';"
            "$adapters=@(Get-NetAdapter -Physical -ErrorAction SilentlyContinue | Where-Object {$_.Status -eq 'Up'});"
            "foreach($a in $adapters){"
            "try{Set-NetAdapterPowerManagement -Name $a.Name -WakeOnMagicPacket Enabled -ErrorAction SilentlyContinue | Out-Null}catch{};"
            "try{Set-NetAdapterPowerManagement -Name $a.Name -ArpOffload Enabled -ErrorAction SilentlyContinue | Out-Null}catch{};"
            "$desc=[string]$a.InterfaceDescription;"
            "if($desc){try{& powercfg.exe /deviceenablewake $desc 2>$null | Out-Null}catch{}}"
            "};"
            "$shutdown=Join-Path $env:SystemRoot 'System32\\shutdown.exe';"
            "if(-not (Test-Path $shutdown)){throw 'shutdown.exe não foi encontrado no Windows.'};"
            "try{& $shutdown /a 2>$null | Out-Null}catch{};"
            "& $shutdown /s /f /t 15;"
            "$exit=[int]$LASTEXITCODE;"
            "if($exit -ne 0){throw ('shutdown.exe recusou o desligamento. Código: '+$exit)};"
            f"'{marker}'"
        )
        output = self._meshctrl_command(
            "RunCommand",
            ["--id", clean_node, "--run", script, "--powershell", "--reply"],
            timeout=min(max(20, settings.remote_command_timeout_seconds), 35),
        )
        if marker not in output:
            raise MeshCentralCommandError(
                "O MeshCentral aceitou o comando, mas o Windows não confirmou a execução do desligamento."
            )
        return output

    def device_enter_managed_off(self, node_id: str) -> str:
        """Enter CoreControl Off without powering down Windows.

        This is the software-only, router-independent power mode.  Windows and
        the management services stay alive, a dedicated SYSTEM process prevents
        automatic sleep, and interactive user sessions are disconnected.  The
        machine therefore remains reachable from the VPS for a deterministic
        later "Ligar" command even when it is the only PC on the LAN.
        """
        clean_node = (node_id or "").strip()
        if not clean_node:
            raise MeshCentralCommandError("O computador não possui identificador remoto para CoreControl Off.")
        marker = "CORECONTROL_MANAGED_OFF_CONFIRMED"
        # The keeper script uses SetThreadExecutionState from its own dedicated
        # process.  This avoids changing the customer's Windows power-plan
        # settings while still preventing an idle transition to sleep/S4/S5.
        script = (
            "$ErrorActionPreference='Stop';"
            "$ProgressPreference='SilentlyContinue';"
            "$dir=Join-Path $env:ProgramData 'CoreControl';"
            "New-Item -ItemType Directory -Path $dir -Force | Out-Null;"
            "$keeper=Join-Path $dir 'managed-off-keeper.ps1';"
            "$body=@'\n"
            "Add-Type -TypeDefinition @\"\n"
            "using System;\n"
            "using System.Runtime.InteropServices;\n"
            "public static class CoreControlPowerState {\n"
            "  [DllImport(\"kernel32.dll\")] public static extern uint SetThreadExecutionState(uint esFlags);\n"
            "}\n"
            "public static class CoreControlManagedSession {\n"
            "  [DllImport(\"Wtsapi32.dll\", SetLastError=true)] public static extern bool WTSDisconnectSession(IntPtr hServer, int sessionId, bool bWait);\n"
            "}\n"
            "\"@\n"
            "$ES_CONTINUOUS=0x80000000; $ES_SYSTEM_REQUIRED=0x00000001;\n"
            "try {\n"
            "  while($true){\n"
            "    [CoreControlPowerState]::SetThreadExecutionState($ES_CONTINUOUS -bor $ES_SYSTEM_REQUIRED) | Out-Null;\n"
            "    $sessions=@(Get-Process explorer -ErrorAction SilentlyContinue | Select-Object -ExpandProperty SessionId -Unique | Where-Object {$_ -gt 0});\n"
            "    foreach($sid in $sessions){try{[CoreControlManagedSession]::WTSDisconnectSession([IntPtr]::Zero,[int]$sid,$false) | Out-Null}catch{}};\n"
            "    Start-Sleep -Seconds 3;\n"
            "  }\n"
            "} finally {\n"
            "  [CoreControlPowerState]::SetThreadExecutionState($ES_CONTINUOUS) | Out-Null;\n"
            "}\n"
            "'@;"
            "Set-Content -Path $keeper -Value $body -Encoding UTF8 -Force;"
            # Do not spawn duplicates if the operator clicks twice.
            "$escaped=[Regex]::Escape($keeper);"
            "$existing=@(Get-CimInstance Win32_Process -Filter \"Name='powershell.exe'\" -ErrorAction SilentlyContinue | Where-Object {$_.CommandLine -match $escaped});"
            "if(-not $existing){"
            "  Start-Process -FilePath 'powershell.exe' -WindowStyle Hidden -ArgumentList @('-NoLogo','-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',$keeper) | Out-Null;"
            "};"
            # Disconnect every Explorer-backed interactive session. This is the
            # service-safe equivalent of locking the user's console session and
            # does not require a password or an interactive Mesh command.
            "$sessions=@(Get-Process explorer -ErrorAction SilentlyContinue | Select-Object -ExpandProperty SessionId -Unique | Where-Object {$_ -gt 0});"
            "if($sessions.Count -gt 0){"
            "$wts=@'\nusing System;\nusing System.Runtime.InteropServices;\npublic static class CoreControlWts {\n[DllImport(\"Wtsapi32.dll\", SetLastError=true)] public static extern bool WTSDisconnectSession(IntPtr hServer, int sessionId, bool bWait);\n}\n'@;"
            "Add-Type -TypeDefinition $wts -ErrorAction SilentlyContinue;"
            "foreach($sid in $sessions){try{[CoreControlWts]::WTSDisconnectSession([IntPtr]::Zero,[int]$sid,$false) | Out-Null}catch{}};"
            "};"
            f"'{marker}'"
        )
        output = self._meshctrl_command(
            "RunCommand",
            ["--id", clean_node, "--run", script, "--powershell", "--reply"],
            timeout=min(max(20, settings.remote_command_timeout_seconds), 35),
        )
        if marker not in output:
            raise MeshCentralCommandError(
                "O MeshCentral aceitou o CoreControl Off, mas o Windows não confirmou a preparação do modo gerenciado."
            )
        return output

    def device_exit_managed_off(self, node_id: str) -> str:
        """Leave CoreControl Off while keeping the machine online."""
        clean_node = (node_id or "").strip()
        if not clean_node:
            raise MeshCentralCommandError("O computador não possui identificador remoto para sair do CoreControl Off.")
        marker = "CORECONTROL_MANAGED_ON_CONFIRMED"
        script = (
            "$ErrorActionPreference='SilentlyContinue';"
            "$keeper=Join-Path (Join-Path $env:ProgramData 'CoreControl') 'managed-off-keeper.ps1';"
            "$escaped=[Regex]::Escape($keeper);"
            "$procs=@(Get-CimInstance Win32_Process -Filter \"Name='powershell.exe'\" -ErrorAction SilentlyContinue | Where-Object {$_.CommandLine -match $escaped});"
            "foreach($p in $procs){try{Stop-Process -Id ([int]$p.ProcessId) -Force -ErrorAction SilentlyContinue}catch{}};"
            "Remove-Item -Path $keeper -Force -ErrorAction SilentlyContinue;"
            # Reset any execution-state request left by the helper process.
            "$code='using System; using System.Runtime.InteropServices; public static class CCPowerReset { [DllImport(\"kernel32.dll\")] public static extern uint SetThreadExecutionState(uint e); }';"
            "try{Add-Type -TypeDefinition $code -ErrorAction SilentlyContinue;[CCPowerReset]::SetThreadExecutionState(0x80000000) | Out-Null}catch{};"
            f"'{marker}'"
        )
        output = self._meshctrl_command(
            "RunCommand",
            ["--id", clean_node, "--run", script, "--powershell", "--reply"],
            timeout=min(max(15, settings.remote_command_timeout_seconds), 30),
        )
        if marker not in output:
            raise MeshCentralCommandError(
                "O Windows não confirmou a saída do CoreControl Off."
            )
        return output

    def device_hibernate_for_wol(self, node_id: str) -> str:
        """Hibernate Windows after re-arming Wake-on-LAN.

        This is the conservative transition used when CoreControl has not yet
        verified an external/local Wake route. It avoids a hard S5 shutdown,
        which can leave some NIC/firmware combinations unable to wake remotely.
        """
        clean_node = (node_id or "").strip()
        if not clean_node:
            raise MeshCentralCommandError("O computador não possui identificador remoto para controle de energia.")
        script = (
            "$ErrorActionPreference='SilentlyContinue';"
            "$adapters=@(Get-NetAdapter -Physical -ErrorAction SilentlyContinue | Where-Object {$_.Status -eq 'Up'});"
            "foreach($a in $adapters){"
            "Set-NetAdapterPowerManagement -Name $a.Name -WakeOnMagicPacket Enabled -ErrorAction SilentlyContinue | Out-Null;Set-NetAdapterPowerManagement -Name $a.Name -ArpOffload Enabled -ErrorAction SilentlyContinue | Out-Null;"
            "$desc=[string]$a.InterfaceDescription;"
            "if($desc){& powercfg.exe /deviceenablewake $desc 2>$null | Out-Null}"
            "};"
            "& powercfg.exe /hibernate on 2>$null | Out-Null;"
            "Start-Sleep -Milliseconds 500;"
            "& shutdown.exe /h /f"
        )
        return self._meshctrl_command(
            "RunCommand",
            ["--id", clean_node, "--run", script, "--powershell"],
            timeout=max(20, settings.remote_command_timeout_seconds),
        )

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
                    f"Grupo automático da empresa {company.name} no CoreControl",
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
        mesh_hex_value = str(selected.get("_idhex") or selected.get("idhex") or "").strip()
        mesh_hex = _mesh_id_to_hex(mesh_hex_value or mesh_id)
        group_name = str(selected.get("name") or desired_name)

        # Mantém as regras de consentimento do grupo sincronizadas com a
        # configuração do CoreControl também para grupos já existentes.
        # Isso é importante porque AddDeviceGroup só aplica --consent na
        # criação; antes, alterar CORETUNER_REMOTE_GROUP_CONSENT não afetava
        # empresas/grupos que já estavam provisionados.
        desired_consent = int(settings.remote_group_consent)
        try:
            current_consent = int(selected.get("consent") or 0)
        except (TypeError, ValueError):
            current_consent = -1

        if current_consent != desired_consent:
            self._meshctrl_command(
                "EditDeviceGroup",
                [
                    "--id",
                    mesh_id,
                    "--consent",
                    str(desired_consent),
                ],
            )
            # Recarrega o grupo para que os dados em memória reflitam o
            # consentimento efetivamente salvo no MeshCentral.
            groups = self._list_groups()
            selected = next(
                (group for group in groups if str(group.get("_id") or group.get("id") or "") == mesh_id),
                selected,
            )

        integration_user_id = self.ensure_integration_user()

        # Sincronize SEMPRE as permissões do usuário técnico. O MeshCentral
        # aceita AddUserToDeviceGroup também para um vínculo já existente e,
        # nesse caso, substitui os direitos pelo valor informado. Versões
        # antigas do CoreControl podiam deixar o usuário com RemoteViewOnly
        # (bit 256) ou com um conjunto parcial de direitos; apenas verificar se
        # o usuário já estava no grupo mantinha esse estado para sempre.
        #
        # Direitos desejados: RemoteControl + esconder Terminal/Files/Registry/
        # AMT + eventos limitados. Deliberadamente NÃO usamos
        # --desktopviewonly nem --limiteddesktop.
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
        return mesh_id, mesh_hex.lower(), group_name

    def remove_company_group(self, mesh_id: str) -> None:
        """Remove o grupo remoto vinculado a uma empresa, quando configurado."""
        value = (mesh_id or "").strip()
        if not value:
            return
        self._meshctrl_command("RemoveDeviceGroup", ["--id", value])
        with self._cache_lock:
            self._device_cache.pop(value, None)

    def _agent_path(self, mesh_hex: str) -> Path:
        cache_root = Path(settings.remote_agent_cache_dir)
        cache_root.mkdir(parents=True, exist_ok=True)
        directory = cache_root / mesh_hex.lower()
        directory.mkdir(parents=True, exist_ok=True)
        return directory / settings.remote_agent_filename

    def _download_agent(self, mesh_id: str, target: Path) -> None:
        endpoint = f"{settings.remote_url.rstrip('/')}/meshagents"
        params = {
            "id": settings.remote_agent_type,
            "meshid": _mesh_id_for_download(mesh_id),
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
            self._download_agent(mesh_id, target)
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
        # Consultas de estado são usadas no polling de ligar/desligar. Não faz
        # sentido uma única leitura prender a interface pelo timeout geral de
        # comandos (45 s por padrão). Em leitura forçada falhamos rápido e a UI
        # tenta novamente; operações administrativas continuam usando o timeout
        # normal.
        status_timeout = min(settings.remote_command_timeout_seconds, 10) if force else None
        output = self._meshctrl_command("ListDevices", ["--id", mesh_id, "--json"], timeout=status_timeout)
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
                    # ``conn`` é um bitmask do MeshCentral. O bit 0 (valor
                    # 1) representa a conexão do Mesh Agent. Outros bits podem
                    # continuar ativos (ex.: Intel AMT/CIRA) mesmo quando o
                    # Windows já desligou. Usar ``conn > 0`` fazia o CoreControl
                    # enxergar o PC como ligado por vários minutos após o
                    # desligamento.
                    connected=bool(conn & 1),
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
    "_mesh_id_for_download",
    "MeshDevice",
    "PreparedRemoteAgent",
    "build_remote_desktop_url",
    "build_user_id",
    "create_login_token",
    "meshcentral_client",
]
