from __future__ import annotations

from typing import Any

from pydantic import BaseModel, EmailStr, Field, field_validator


class LoginRequest(BaseModel):
    email: EmailStr
    password: str = Field(min_length=8, max_length=200)


class PasswordResetRequest(BaseModel):
    email: EmailStr


class PasswordResetConfirmRequest(BaseModel):
    token: str = Field(min_length=20, max_length=300)
    password: str = Field(min_length=10, max_length=200)
    password_confirmation: str = Field(min_length=10, max_length=200)

    @field_validator("password_confirmation")
    @classmethod
    def reset_passwords_match(cls, value: str, info):
        if info.data.get("password") and value != info.data["password"]:
            raise ValueError("As senhas não conferem")
        return value


class CompanyRegistrationRequest(BaseModel):
    company_name: str = Field(min_length=2, max_length=160)
    responsible_name: str = Field(min_length=2, max_length=120)
    email: EmailStr
    password: str = Field(min_length=10, max_length=200)
    password_confirmation: str = Field(min_length=10, max_length=200)

    @field_validator("password_confirmation")
    @classmethod
    def passwords_match(cls, value: str, info):
        if info.data.get("password") and value != info.data["password"]:
            raise ValueError("As senhas não conferem")
        return value


class CompanyCreate(BaseModel):
    name: str = Field(min_length=2, max_length=160)


class UserCreate(BaseModel):
    name: str = Field(min_length=2, max_length=120)
    email: EmailStr
    password: str = Field(min_length=10, max_length=200)
    role: str = Field(default="viewer")
    company_id: int | None = None


class CompanyUpdate(BaseModel):
    name: str | None = Field(default=None, min_length=2, max_length=160)
    active: bool | None = None


class CompanyDestroyRequest(BaseModel):
    confirmation: str = Field(min_length=8, max_length=220)


class UserUpdate(BaseModel):
    name: str | None = Field(default=None, min_length=2, max_length=120)
    email: EmailStr | None = None
    password: str | None = Field(default=None, min_length=10, max_length=200)
    role: str | None = None
    company_id: int | None = None
    active: bool | None = None


class DeviceUpdate(BaseModel):
    company_id: int | None = None
    name: str | None = Field(default=None, min_length=1, max_length=160)
    hostname: str | None = Field(default=None, min_length=1, max_length=160)
    sector: str | None = Field(default=None, max_length=120)
    location: str | None = Field(default=None, max_length=160)
    manufacturer: str | None = Field(default=None, max_length=160)
    model: str | None = Field(default=None, max_length=160)
    serial_number: str | None = Field(default=None, max_length=160)
    profile: str | None = Field(default=None, max_length=80)
    active: bool | None = None


class EnrollmentRequest(BaseModel):
    enrollment_token: str
    device_uid: str = Field(min_length=3, max_length=190)
    name: str = Field(min_length=1, max_length=160)
    hostname: str = Field(min_length=1, max_length=160)
    sector: str | None = Field(default=None, max_length=120)
    location: str | None = Field(default=None, max_length=160)
    manufacturer: str | None = Field(default=None, max_length=160)
    model: str | None = Field(default=None, max_length=160)
    serial_number: str | None = Field(default=None, max_length=160)
    os_name: str | None = Field(default=None, max_length=160)
    os_version: str | None = Field(default=None, max_length=160)
    agent_version: str | None = Field(default=None, max_length=40)


class DeviceInstallRequest(BaseModel):
    company_id: int | None = None
    install_remote: bool = False
    device_uid: str = Field(min_length=3, max_length=190)
    name: str = Field(min_length=1, max_length=160)
    hostname: str = Field(min_length=1, max_length=160)
    sector: str | None = Field(default=None, max_length=120)
    location: str | None = Field(default=None, max_length=160)
    manufacturer: str | None = Field(default=None, max_length=160)
    model: str | None = Field(default=None, max_length=160)
    serial_number: str | None = Field(default=None, max_length=160)
    os_name: str | None = Field(default=None, max_length=160)
    os_version: str | None = Field(default=None, max_length=160)
    agent_version: str | None = Field(default=None, max_length=40)


class TelemetryRequest(BaseModel):
    device_uid: str
    cpu_percent: float | None = Field(default=None, ge=0, le=100)
    memory_percent: float | None = Field(default=None, ge=0, le=100)
    memory_used_gb: float | None = Field(default=None, ge=0)
    memory_total_gb: float | None = Field(default=None, ge=0)
    disk_percent: float | None = Field(default=None, ge=0, le=100)
    disk_free_gb: float | None = Field(default=None, ge=0)
    disk_total_gb: float | None = Field(default=None, ge=0)
    temperature_c: float | None = Field(default=None, ge=-20, le=150)
    uptime_seconds: int | None = Field(default=None, ge=0)
    ip_local: str | None = Field(default=None, max_length=80)
    network_name: str | None = Field(default=None, max_length=160)
    defender_active: bool | None = None
    firewall_active: bool | None = None
    profile: str | None = Field(default=None, max_length=80)
    extra: dict[str, Any] | None = None


class DownloadUnlockRequest(BaseModel):
    password: str = Field(min_length=1, max_length=128)


class UpdateCheckRequest(BaseModel):
    device_ids: list[int] | None = None

    @field_validator("device_ids")
    @classmethod
    def validate_update_device_ids(cls, value):
        if value is None:
            return value
        unique = list(dict.fromkeys(value))
        if len(unique) > 500:
            raise ValueError("Selecione no máximo 500 computadores por operação")
        if any(item <= 0 for item in unique):
            raise ValueError("Computador inválido")
        return unique


class UpdateInstallRequest(BaseModel):
    device_id: int = Field(gt=0)
    item_keys: list[str] = Field(min_length=1, max_length=500)


class AgentCommandResultRequest(BaseModel):
    device_uid: str = Field(min_length=3, max_length=190)
    ok: bool
    result: dict[str, Any] | None = None
    error: str | None = Field(default=None, max_length=4000)


class UpdatePolicyCreate(BaseModel):
    company_id: int | None = None
    name: str = Field(min_length=2, max_length=160)
    active: bool = True
    auto_scan: bool = True
    auto_install: bool = False
    include_windows: bool = True
    include_drivers: bool = False
    include_apps: bool = False
    scan_interval_hours: int = Field(default=24, ge=1, le=168)
    allowed_days: list[int] = Field(default_factory=lambda: [0, 1, 2, 3, 4], min_length=1, max_length=7)
    start_hour: int = Field(default=1, ge=0, le=23)
    end_hour: int = Field(default=5, ge=0, le=23)
    timezone: str = Field(default="America/Sao_Paulo", min_length=1, max_length=80)

    @field_validator("allowed_days")
    @classmethod
    def validate_days(cls, value):
        unique = sorted(set(value))
        if any(day < 0 or day > 6 for day in unique):
            raise ValueError("Dia da semana inválido")
        return unique


class UpdatePolicyUpdate(BaseModel):
    name: str | None = Field(default=None, min_length=2, max_length=160)
    active: bool | None = None
    auto_scan: bool | None = None
    auto_install: bool | None = None
    include_windows: bool | None = None
    include_drivers: bool | None = None
    include_apps: bool | None = None
    scan_interval_hours: int | None = Field(default=None, ge=1, le=168)
    allowed_days: list[int] | None = Field(default=None, min_length=1, max_length=7)
    start_hour: int | None = Field(default=None, ge=0, le=23)
    end_hour: int | None = Field(default=None, ge=0, le=23)
    timezone: str | None = Field(default=None, min_length=1, max_length=80)

    @field_validator("allowed_days")
    @classmethod
    def validate_optional_days(cls, value):
        if value is None:
            return value
        unique = sorted(set(value))
        if any(day < 0 or day > 6 for day in unique):
            raise ValueError("Dia da semana inválido")
        return unique
