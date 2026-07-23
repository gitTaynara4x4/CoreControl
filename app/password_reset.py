from __future__ import annotations

import json
import logging
import threading
import time
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from typing import Annotated
from urllib.parse import urlencode

from fastapi import APIRouter, Depends, HTTPException, Request, Response, status
from sqlalchemy import func, select, update
from sqlalchemy.orm import Session

from .config import settings
from .db import get_db
from .email_service import EmailDeliveryError, send_password_reset_email
from .models import AuditLog, PasswordResetToken, User
from .schemas import PasswordResetConfirmRequest, PasswordResetRequest
from .security import hash_password, new_secret, sha256_text

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/api/auth/password-reset", tags=["password-reset"])
Db = Annotated[Session, Depends(get_db)]
GENERIC_MESSAGE = "Se o e-mail estiver cadastrado, enviaremos as instruções de recuperação."


@dataclass
class AttemptState:
    attempts: list[float] = field(default_factory=list)


_attempts: dict[str, AttemptState] = defaultdict(AttemptState)
_attempt_lock = threading.Lock()


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


def as_utc(value: datetime) -> datetime:
    if value.tzinfo is None:
        return value.replace(tzinfo=timezone.utc)
    return value.astimezone(timezone.utc)


def client_ip(request: Request) -> str:
    forwarded = request.headers.get("x-forwarded-for", "")
    if forwarded:
        return forwarded.split(",", 1)[0].strip()[:120]
    if request.client:
        return request.client.host[:120]
    return "unknown"


def check_rate_limit(key: str) -> None:
    now = time.monotonic()
    start = now - settings.password_reset_window_seconds
    with _attempt_lock:
        state = _attempts[key]
        state.attempts = [item for item in state.attempts if item >= start]
        if len(state.attempts) >= settings.password_reset_max_attempts:
            raise HTTPException(
                status_code=status.HTTP_429_TOO_MANY_REQUESTS,
                detail="Muitas solicitações. Aguarde alguns minutos antes de tentar novamente.",
            )
        state.attempts.append(now)


def build_reset_url(raw_token: str) -> str:
    query = urlencode({"reset_token": raw_token})
    return f"{settings.public_url}/?{query}"


@router.post("/request")
def request_password_reset(payload: PasswordResetRequest, request: Request, db: Db):
    if not settings.smtp_configured:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="A recuperação por e-mail ainda não foi configurada no servidor.",
        )

    email = payload.email.lower().strip()
    ip = client_ip(request)
    check_rate_limit(f"{ip}:{sha256_text(email)[:16]}")

    user = db.scalar(select(User).where(func.lower(User.email) == email))
    if not user or not user.active:
        return {"ok": True, "message": GENERIC_MESSAGE}

    now = utcnow()
    db.execute(
        update(PasswordResetToken)
        .where(PasswordResetToken.user_id == user.id, PasswordResetToken.used_at.is_(None))
        .values(used_at=now)
    )

    raw_token = new_secret(32)
    reset_token = PasswordResetToken(
        user_id=user.id,
        token_hash=sha256_text(raw_token),
        expires_at=now + timedelta(minutes=settings.password_reset_minutes),
        requested_ip=ip,
    )
    db.add(reset_token)
    db.add(
        AuditLog(
            company_id=user.company_id,
            actor_user_id=user.id,
            action="auth.password_reset.requested",
            details=json.dumps({"email": user.email, "ip": ip}, ensure_ascii=False),
        )
    )
    db.commit()

    try:
        send_password_reset_email(
            recipient_name=user.name,
            recipient_email=user.email,
            reset_url=build_reset_url(raw_token),
        )
    except EmailDeliveryError:
        logger.exception("Falha ao enviar recuperação de senha para user_id=%s", user.id)
        reset_token.used_at = utcnow()
        db.add(
            AuditLog(
                company_id=user.company_id,
                actor_user_id=user.id,
                action="auth.password_reset.email_failed",
                details=json.dumps({"email": user.email, "ip": ip}, ensure_ascii=False),
            )
        )
        db.commit()

    return {"ok": True, "message": GENERIC_MESSAGE}


@router.post("/confirm")
def confirm_password_reset(payload: PasswordResetConfirmRequest, response: Response, db: Db):
    token_hash = sha256_text(payload.token.strip())
    reset_token = db.scalar(
        select(PasswordResetToken).where(PasswordResetToken.token_hash == token_hash)
    )
    now = utcnow()
    if (
        not reset_token
        or reset_token.used_at is not None
        or as_utc(reset_token.expires_at) < now
    ):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="O link de recuperação é inválido, expirou ou já foi utilizado.",
        )

    user = db.get(User, reset_token.user_id)
    if not user or not user.active:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="O link de recuperação é inválido, expirou ou já foi utilizado.",
        )

    user.password_hash = hash_password(payload.password)
    db.execute(
        update(PasswordResetToken)
        .where(PasswordResetToken.user_id == user.id, PasswordResetToken.used_at.is_(None))
        .values(used_at=now)
    )
    db.add(
        AuditLog(
            company_id=user.company_id,
            actor_user_id=user.id,
            action="auth.password_reset.completed",
            details="Senha redefinida por link de recuperação.",
        )
    )
    db.commit()
    response.delete_cookie("coretuner_session", path="/")
    return {"ok": True, "message": "Senha redefinida com sucesso. Entre usando a nova senha."}
