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

    @property
    def secure_cookies(self) -> bool:
        return self.environment.lower() == "production"

    @property
    def is_production(self) -> bool:
        return self.environment.lower() == "production"


settings = Settings()
