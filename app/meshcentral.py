from __future__ import annotations

import base64
import json
import secrets
import time
from urllib.parse import urlencode

from cryptography.hazmat.primitives.ciphers.aead import AESGCM


class MeshCentralTokenError(RuntimeError):
    """Raised when the MeshCentral integration secret is invalid."""


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
    """Create a MeshCentral AES-256-GCM login cookie compatible token.

    MeshCentral stores the generated 80-byte key as hexadecimal. The current
    server implementation uses the first 32 bytes as an AES-256-GCM key and
    serializes the token as IV + authentication tag + ciphertext.
    """
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


def build_remote_desktop_url(
    *,
    base_url: str,
    login_token: str,
    hostname: str,
) -> str:
    clean_hostname = (hostname or "").strip()
    if not clean_hostname:
        raise MeshCentralTokenError("O computador não possui um hostname válido.")
    query = urlencode(
        {
            "login": login_token,
            "gotodevicername": clean_hostname,
            "viewmode": "11",
            "hide": "63",
        }
    )
    return f"{base_url.rstrip('/')}/?{query}"
