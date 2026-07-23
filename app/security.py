from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import secrets
from datetime import datetime, timedelta, timezone

from fastapi import HTTPException, Request, status

from .config import settings

PBKDF2_ROUNDS = 310_000


def hash_password(password: str) -> str:
    salt = os.urandom(16)
    digest = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, PBKDF2_ROUNDS)
    return f"pbkdf2_sha256${PBKDF2_ROUNDS}${base64.urlsafe_b64encode(salt).decode()}${base64.urlsafe_b64encode(digest).decode()}"


def verify_password(password: str, encoded: str) -> bool:
    try:
        algorithm, rounds, salt_b64, digest_b64 = encoded.split("$", 3)
        if algorithm != "pbkdf2_sha256":
            return False
        salt = base64.urlsafe_b64decode(salt_b64.encode())
        expected = base64.urlsafe_b64decode(digest_b64.encode())
        actual = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, int(rounds))
        return hmac.compare_digest(actual, expected)
    except (ValueError, TypeError):
        return False


def _b64(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def _unb64(value: str) -> bytes:
    return base64.urlsafe_b64decode(value + "=" * (-len(value) % 4))


def create_session_token(user_id: int, role: str, company_id: int | None) -> str:
    now = datetime.now(timezone.utc)
    payload = {
        "sub": user_id,
        "role": role,
        "company_id": company_id,
        "iat": int(now.timestamp()),
        "exp": int((now + timedelta(minutes=settings.token_minutes)).timestamp()),
        "nonce": secrets.token_hex(8),
    }
    body = _b64(json.dumps(payload, separators=(",", ":")).encode("utf-8"))
    sig = _b64(hmac.new(settings.secret_key.encode("utf-8"), body.encode("ascii"), hashlib.sha256).digest())
    return f"{body}.{sig}"


def decode_session_token(token: str) -> dict:
    try:
        body, sig = token.split(".", 1)
        expected = _b64(hmac.new(settings.secret_key.encode("utf-8"), body.encode("ascii"), hashlib.sha256).digest())
        if not hmac.compare_digest(sig, expected):
            raise ValueError("invalid signature")
        payload = json.loads(_unb64(body))
        if int(payload["exp"]) < int(datetime.now(timezone.utc).timestamp()):
            raise ValueError("expired")
        return payload
    except Exception as exc:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Sessão inválida ou expirada") from exc


def get_session_payload(request: Request) -> dict:
    token = request.cookies.get("coretuner_session")
    if not token:
        authorization = request.headers.get("authorization", "")
        if authorization.lower().startswith("bearer "):
            token = authorization.split(" ", 1)[1].strip()
    if not token:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Autenticação necessária")
    return decode_session_token(token)


def new_secret(length_bytes: int = 32) -> str:
    return secrets.token_urlsafe(length_bytes)


def sha256_text(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def create_download_token(filename: str) -> str:
    """Cria autorização curta e assinada para um único arquivo de download."""
    now = datetime.now(timezone.utc)
    payload = {
        "purpose": "protected_download",
        "filename": filename,
        "iat": int(now.timestamp()),
        "exp": int((now + timedelta(seconds=settings.download_token_seconds)).timestamp()),
        "nonce": secrets.token_hex(12),
    }
    body = _b64(json.dumps(payload, separators=(",", ":")).encode("utf-8"))
    signing_key = f"{settings.secret_key}:download".encode("utf-8")
    sig = _b64(hmac.new(signing_key, body.encode("ascii"), hashlib.sha256).digest())
    return f"{body}.{sig}"


def decode_download_token(token: str, expected_filename: str) -> dict:
    try:
        body, sig = token.split(".", 1)
        signing_key = f"{settings.secret_key}:download".encode("utf-8")
        expected_sig = _b64(hmac.new(signing_key, body.encode("ascii"), hashlib.sha256).digest())
        if not hmac.compare_digest(sig, expected_sig):
            raise ValueError("invalid signature")
        payload = json.loads(_unb64(body))
        if payload.get("purpose") != "protected_download":
            raise ValueError("invalid purpose")
        if payload.get("filename") != expected_filename:
            raise ValueError("invalid filename")
        if int(payload["exp"]) < int(datetime.now(timezone.utc).timestamp()):
            raise ValueError("expired")
        return payload
    except Exception as exc:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Autorização de download inválida ou expirada",
        ) from exc
