from __future__ import annotations

import argparse
import base64
import getpass
from dataclasses import dataclass

from sqlalchemy import func, select

from app.config import settings
from app.db import Base, SessionLocal, apply_runtime_migrations, engine
from app.models import AuditLog, User
from app.security import hash_password


@dataclass(frozen=True)
class Credentials:
    email: str
    password_hash: str


def _valid_password_hash(value: str) -> bool:
    try:
        algorithm, rounds, salt_b64, digest_b64 = value.split("$", 3)
        if algorithm != "pbkdf2_sha256" or int(rounds) < 100_000:
            return False
        salt = base64.urlsafe_b64decode(salt_b64.encode("ascii"))
        digest = base64.urlsafe_b64decode(digest_b64.encode("ascii"))
        return len(salt) >= 16 and len(digest) >= 32
    except (ValueError, TypeError, UnicodeError):
        return False


def _credentials_from_env() -> Credentials:
    email = settings.global_admin_email.strip().lower()
    encoded = settings.global_admin_password_hash.strip()
    if not email:
        raise SystemExit("ERRO: CORECONTROL_GLOBAL_ADMIN_EMAIL não foi definido no .env.")
    if not encoded:
        raise SystemExit("ERRO: CORECONTROL_GLOBAL_ADMIN_PASSWORD_HASH não foi definido no .env.")
    if not _valid_password_hash(encoded):
        raise SystemExit(
            "ERRO: CORECONTROL_GLOBAL_ADMIN_PASSWORD_HASH não contém um hash PBKDF2 válido. "
            "Não coloque a senha normal nesse campo."
        )
    return Credentials(email=email, password_hash=encoded)


def _credentials_interactive() -> Credentials:
    suggested = settings.global_admin_email.strip().lower()
    prompt = f"E-mail do Administrador Global [{suggested}]: " if suggested else "E-mail do Administrador Global: "
    email = input(prompt).strip().lower() or suggested
    if not email or "@" not in email:
        raise SystemExit("ERRO: informe um e-mail válido.")

    password = getpass.getpass("Nova senha: ")
    confirmation = getpass.getpass("Confirme a nova senha: ")
    if password != confirmation:
        raise SystemExit("ERRO: as senhas não conferem.")
    if len(password) < 12:
        raise SystemExit("ERRO: a senha precisa ter pelo menos 12 caracteres.")
    return Credentials(email=email, password_hash=hash_password(password))


def create_global_admin(credentials: Credentials, name: str) -> int:
    Base.metadata.create_all(bind=engine)
    apply_runtime_migrations()

    with SessionLocal() as db:
        existing_global = db.scalar(select(User).where(User.role == "global_admin"))
        if existing_global:
            raise SystemExit(
                f"ERRO: já existe um Administrador Global cadastrado ({existing_global.email}). "
                "Nenhuma alteração foi feita."
            )

        duplicate = db.scalar(select(User).where(func.lower(User.email) == credentials.email))
        if duplicate:
            raise SystemExit(
                f"ERRO: o e-mail {credentials.email} já pertence a outro usuário. Nenhuma alteração foi feita."
            )

        user = User(
            name=name.strip() or "Administrador Global",
            email=credentials.email,
            password_hash=credentials.password_hash,
            role="global_admin",
            company_id=None,
            active=True,
        )
        db.add(user)
        db.flush()
        db.add(
            AuditLog(
                company_id=None,
                actor_user_id=user.id,
                action="system.global_admin.manual_create.v1",
                details="Administrador Global criado por provisionamento manual",
            )
        )
        db.commit()
        return user.id


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Cria explicitamente a primeira conta de Administrador Global do CoreControl."
    )
    parser.add_argument(
        "--from-env",
        action="store_true",
        help="usa CORECONTROL_GLOBAL_ADMIN_EMAIL e CORECONTROL_GLOBAL_ADMIN_PASSWORD_HASH do .env",
    )
    parser.add_argument("--name", default="Administrador Global", help="nome exibido para a conta")
    args = parser.parse_args()

    credentials = _credentials_from_env() if args.from_env else _credentials_interactive()
    user_id = create_global_admin(credentials, args.name)
    print(f"OK - Administrador Global criado com ID {user_id}: {credentials.email}")
    print("O servidor não recria esta conta automaticamente se ela for apagada.")


if __name__ == "__main__":
    main()
