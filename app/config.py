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
    dev_web: bool = _bool_env("CORETUNER_DEV_WEB", False)
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
    remote_login_token_key: str = os.getenv("CORETUNER_REMOTE_LOGIN_TOKEN_KEY", "").strip()
    remote_login_user: str = os.getenv("CORETUNER_REMOTE_LOGIN_USER", "").strip()
    remote_login_domain: str = os.getenv("CORETUNER_REMOTE_LOGIN_DOMAIN", "").strip()
    remote_login_token_minutes: int = _int_env("CORETUNER_REMOTE_LOGIN_TOKEN_MINUTES", 2)
    remote_admin_user: str = os.getenv("CORETUNER_REMOTE_ADMIN_USER", "user-mesh-adm").strip()
    remote_node_path: str = os.getenv("CORETUNER_REMOTE_NODE_PATH", "node").strip() or "node"
    remote_meshctrl_path: str = os.getenv(
        "CORETUNER_REMOTE_MESHCTRL_PATH",
        "/opt/meshcentral-client/node_modules/meshcentral/meshctrl.js",
    ).strip()
    remote_agent_type: int = _int_env("CORETUNER_REMOTE_AGENT_TYPE", 4)
    remote_agent_install_flags: int = _int_env("CORETUNER_REMOTE_AGENT_INSTALL_FLAGS", 2)
    remote_group_features: int = _int_env("CORETUNER_REMOTE_GROUP_FEATURES", 2)
    remote_group_consent: int = _int_env("CORETUNER_REMOTE_GROUP_CONSENT", 65)
    remote_command_timeout_seconds: int = _int_env("CORETUNER_REMOTE_COMMAND_TIMEOUT_SECONDS", 45)
    remote_agent_download_timeout_seconds: int = _int_env("CORETUNER_REMOTE_AGENT_DOWNLOAD_TIMEOUT_SECONDS", 90)
    remote_agent_cache_seconds: int = _int_env("CORETUNER_REMOTE_AGENT_CACHE_SECONDS", 86400)
    remote_status_cache_seconds: int = _int_env("CORETUNER_REMOTE_STATUS_CACHE_SECONDS", 15)
    remote_status_stale_seconds: int = _int_env("CORETUNER_REMOTE_STATUS_STALE_SECONDS", 120)
    remote_agent_cache_dir: str = os.getenv(
        "CORETUNER_REMOTE_AGENT_CACHE_DIR",
        str(Path(os.getenv("CORETUNER_DATA_DIR", "./data")).resolve() / "remote-agents"),
    ).strip()
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
    def remote_token_configured(self) -> bool:
        return bool(
            self.remote_enabled
            and self.remote_url
            and self.remote_login_token_key
            and self.remote_login_user
        )

    @property
    def remote_provisioning_configured(self) -> bool:
        return bool(
            self.remote_token_configured
            and self.remote_admin_user
            and self.remote_meshctrl_path
            and self.remote_agent_type > 0
        )

    @property
    def smtp_sender(self) -> str:
        return self.smtp_from_email or self.smtp_user

    @property
    def smtp_configured(self) -> bool:
        return bool(self.smtp_host and self.smtp_port and self.smtp_user and self.smtp_password and self.smtp_sender)


settings = Settings()
