from __future__ import annotations

import hmac
import threading
import time
from collections import defaultdict
from dataclasses import dataclass, field
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Request, status
from sqlalchemy.orm import Session

from .config import settings
from .db import get_db
from .models import AuditLog, User
from .schemas import DownloadUnlockRequest
from .security import create_download_token, get_session_payload

router = APIRouter(prefix="/api/public", tags=["public"])
DOWNLOAD_FILENAME = "CoreTunerSetup.exe"
Db = Annotated[Session, Depends(get_db)]


@dataclass
class AttemptState:
    failures: list[float] = field(default_factory=list)
    blocked_until: float = 0.0


_attempts: dict[str, AttemptState] = defaultdict(AttemptState)
_attempt_lock = threading.Lock()


def _client_key(request: Request) -> str:
    forwarded = request.headers.get("x-forwarded-for", "")
    if forwarded:
        return forwarded.split(",", 1)[0].strip()[:120]
    if request.client:
        return request.client.host[:120]
    return "unknown"


def _check_rate_limit(key: str) -> None:
    now = time.monotonic()
    with _attempt_lock:
        state = _attempts[key]
        if state.blocked_until > now:
            retry_after = max(1, int(state.blocked_until - now))
            raise HTTPException(
                status_code=status.HTTP_429_TOO_MANY_REQUESTS,
                detail="Muitas tentativas. Aguarde antes de tentar novamente.",
                headers={"Retry-After": str(retry_after)},
            )
        window_start = now - settings.download_attempt_window_seconds
        state.failures = [attempt for attempt in state.failures if attempt >= window_start]


def _record_failure(key: str) -> None:
    now = time.monotonic()
    with _attempt_lock:
        state = _attempts[key]
        window_start = now - settings.download_attempt_window_seconds
        state.failures = [attempt for attempt in state.failures if attempt >= window_start]
        state.failures.append(now)
        if len(state.failures) >= settings.download_max_attempts:
            state.blocked_until = now + settings.download_block_seconds
            state.failures.clear()


def _clear_failures(key: str) -> None:
    with _attempt_lock:
        _attempts.pop(key, None)


def _authenticated_user(request: Request, db: Session) -> User:
    payload = get_session_payload(request)
    user = db.get(User, int(payload["sub"]))
    if not user or not user.active:
        raise HTTPException(status_code=401, detail="Faça login para liberar o download")
    return user


@router.get("/download-status")
def download_status(request: Request, db: Db):
    authenticated = False
    user_name = None
    try:
        user = _authenticated_user(request, db)
        authenticated = True
        user_name = user.name
    except HTTPException:
        pass
    return {
        "enabled": bool(settings.download_password),
        "filename": DOWNLOAD_FILENAME,
        "version": "0.4.9",
        "protected": True,
        "requires_login": True,
        "authenticated": authenticated,
        "user_name": user_name,
    }


@router.post("/download-ticket")
def create_ticket(payload: DownloadUnlockRequest, request: Request, db: Db):
    user = _authenticated_user(request, db)

    if not settings.download_password:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Download temporariamente indisponível. Configure CORETUNER_DOWNLOAD_PASSWORD no servidor.",
        )

    key = f"{user.id}:{_client_key(request)}"
    _check_rate_limit(key)

    submitted = payload.password.encode("utf-8")
    configured = settings.download_password.encode("utf-8")
    if not hmac.compare_digest(submitted, configured):
        _record_failure(key)
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Senha de download inválida")

    _clear_failures(key)
    token = create_download_token(DOWNLOAD_FILENAME)
    db.add(
        AuditLog(
            company_id=user.company_id,
            actor_user_id=user.id,
            action="download.setup.unlock",
            details=f"CoreTunerSetup liberado para {user.email}",
        )
    )
    db.commit()
    return {
        "ok": True,
        "download_url": f"/downloads/{DOWNLOAD_FILENAME}?token={token}",
        "expires_in_seconds": settings.download_token_seconds,
        "filename": DOWNLOAD_FILENAME,
    }
