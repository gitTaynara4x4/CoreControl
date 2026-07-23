from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv

PROJECT_ROOT = Path(__file__).resolve().parent.parent
load_dotenv(PROJECT_ROOT / ".env", override=False)


def _int_env(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None:
        return default
    try:
        return int(raw)
    except ValueError as exc:
        raise RuntimeError(f"A variável {name} precisa ser um número inteiro.") from exc


def _bool_env(name: str, default: bool) -> bool:
    raw = os.getenv(name)
    if raw is None:
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on", "sim"}


def _database_url() -> str:
    raw = os.getenv(
        "CORETUNER_DATABASE_URL",
        f"sqlite:///{Path(os.getenv('CORETUNER_DATA_DIR', './data')).resolve() / 'coretuner.db'}",
    ).strip()
    # EasyPanel e alguns provedores entregam postgres://. SQLAlchemy 2 usa
    # postgresql+psycopg:// para o driver psycopg 3.
    if raw.startswith("postgres://"):
        return "postgresql+psycopg://" + raw[len("postgres://") :]
    if raw.startswith("postgresql://") and "+" not in raw.split("://", 1)[0]:
        return "postgresql+psycopg://" + raw[len("postgresql://") :]
    return raw


@dataclass(frozen=True)
class Settings:
    app_name: str = "CoreTuner Central"
    environment: str = os.getenv("CORETUNER_ENV", "development")
    secret_key: str = os.getenv("CORETUNER_SECRET_KEY", "change-this-in-production")
    database_url: str = _database_url()
    public_url: str = os.getenv("CORETUNER_PUBLIC_URL", "http://127.0.0.1:8002").rstrip("/")
    admin_email: str = os.getenv("CORETUNER_ADMIN_EMAIL", "admin@coretuner.com.br")
    admin_password: str = os.getenv("CORETUNER_ADMIN_PASSWORD", "TroqueAgora123!")
    download_password: str = os.getenv("CORETUNER_DOWNLOAD_PASSWORD", "")
    download_token_seconds: int = _int_env("CORETUNER_DOWNLOAD_TOKEN_SECONDS", 180)
    download_attempt_window_seconds: int = _int_env("CORETUNER_DOWNLOAD_ATTEMPT_WINDOW_SECONDS", 600)
    download_max_attempts: int = _int_env("CORETUNER_DOWNLOAD_MAX_ATTEMPTS", 5)
    download_block_seconds: int = _int_env("CORETUNER_DOWNLOAD_BLOCK_SECONDS", 900)
    token_minutes: int = _int_env("CORETUNER_TOKEN_MINUTES", 720)
    offline_after_seconds: int = _int_env("CORETUNER_OFFLINE_AFTER_SECONDS", 180)
    telemetry_retention_days: int = _int_env("CORETUNER_TELEMETRY_RETENTION_DAYS", 30)
    remote_enabled: bool = _bool_env("CORETUNER_REMOTE_ENABLED", False)
    remote_url: str = os.getenv("CORETUNER_REMOTE_URL", "").strip().rstrip("/")
    remote_agent_filename: str = os.getenv("CORETUNER_REMOTE_AGENT_FILENAME", "CoreTunerRemoteAgent.exe").strip() or "CoreTunerRemoteAgent.exe"
    smtp_host: str = os.getenv("CORETUNER_SMTP_HOST", "smtp.gmail.com").strip()
    smtp_port: int = _int_env("CORETUNER_SMTP_PORT", 587)
    smtp_user: str = os.getenv("CORETUNER_SMTP_USER", "").strip()
    smtp_password: str = os.getenv("CORETUNER_SMTP_PASSWORD", "").strip()
    smtp_from_email: str = os.getenv("CORETUNER_SMTP_FROM_EMAIL", "").strip()
    smtp_from_name: str = os.getenv("CORETUNER_SMTP_FROM_NAME", "CoreTuner").strip() or "CoreTuner"
    smtp_starttls: bool = _bool_env("CORETUNER_SMTP_STARTTLS", True)
    password_reset_minutes: int = _int_env("CORETUNER_PASSWORD_RESET_MINUTES", 20)
    password_reset_max_attempts: int = _int_env("CORETUNER_PASSWORD_RESET_MAX_ATTEMPTS", 5)
    password_reset_window_seconds: int = _int_env("CORETUNER_PASSWORD_RESET_WINDOW_SECONDS", 900)

    @property
    def secure_cookies(self) -> bool:
        return self.environment.lower() == "production"

    @property
    def is_production(self) -> bool:
        return self.environment.lower() == "production"

    @property
    def smtp_sender(self) -> str:
        return self.smtp_from_email or self.smtp_user

    @property
    def smtp_configured(self) -> bool:
        return bool(self.smtp_host and self.smtp_port and self.smtp_user and self.smtp_password and self.smtp_sender)


settings = Settings()
