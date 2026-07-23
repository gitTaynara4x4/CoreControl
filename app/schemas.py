from __future__ import annotations

from typing import Any

from pydantic import BaseModel, EmailStr, Field, field_validator


class LoginRequest(BaseModel):
    email: EmailStr
    password: str = Field(min_length=8, max_length=200)


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
