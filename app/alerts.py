from __future__ import annotations

from datetime import datetime, timezone

from sqlalchemy import desc, select
from sqlalchemy.orm import Session

from .models import Alert, Device, Telemetry


def _upsert_alert(
    db: Session,
    *,
    device: Device,
    alert_type: str,
    severity: str,
    title: str,
    message: str,
    active: bool,
) -> None:
    now = datetime.now(timezone.utc)
    current = db.scalar(
        select(Alert).where(
            Alert.device_id == device.id,
            Alert.alert_type == alert_type,
            Alert.status.in_(["open", "acknowledged"]),
        )
    )
    if active:
        if current:
            current.last_seen_at = now
            current.message = message
            current.severity = severity
        else:
            db.add(
                Alert(
                    company_id=device.company_id,
                    device_id=device.id,
                    alert_type=alert_type,
                    severity=severity,
                    title=title,
                    message=message,
                    status="open",
                    opened_at=now,
                    last_seen_at=now,
                )
            )
    elif current:
        current.status = "resolved"
        current.resolved_at = now
        current.last_seen_at = now


def _sustained(db: Session, device_id: int, field: str, threshold: float, required: int = 3) -> bool:
    samples = list(
        db.scalars(
            select(Telemetry)
            .where(Telemetry.device_id == device_id)
            .order_by(desc(Telemetry.recorded_at))
            .limit(required)
        ).all()
    )
    if len(samples) < required:
        return False
    values = [getattr(sample, field) for sample in samples]
    return all(value is not None and value >= threshold for value in values)


def evaluate_telemetry_alerts(db: Session, device: Device, sample: Telemetry) -> None:
    cpu_active = _sustained(db, device.id, "cpu_percent", 92, required=3)
    memory_active = _sustained(db, device.id, "memory_percent", 90, required=3)
    temperature_active = _sustained(db, device.id, "temperature_c", 85, required=2)

    _upsert_alert(
        db,
        device=device,
        alert_type="cpu_high",
        severity="warning",
        title="Processador em uso elevado",
        message=f"CPU permaneceu em uso elevado; leitura atual: {sample.cpu_percent:.0f}%."
        if sample.cpu_percent is not None
        else "",
        active=cpu_active,
    )
    _upsert_alert(
        db,
        device=device,
        alert_type="memory_high",
        severity="warning",
        title="Memória RAM em uso elevado",
        message=f"Memória permaneceu elevada; leitura atual: {sample.memory_percent:.0f}%."
        if sample.memory_percent is not None
        else "",
        active=memory_active,
    )
    _upsert_alert(
        db,
        device=device,
        alert_type="disk_low",
        severity="critical",
        title="Pouco espaço livre no disco",
        message=f"Restam {sample.disk_free_gb:.1f} GB no disco principal." if sample.disk_free_gb is not None else "",
        active=(
            sample.disk_percent is not None
            and sample.disk_percent >= 92
            or sample.disk_free_gb is not None
            and sample.disk_free_gb <= 10
        ),
    )
    _upsert_alert(
        db,
        device=device,
        alert_type="temperature_high",
        severity="critical",
        title="Temperatura elevada",
        message=f"Temperatura permaneceu elevada; leitura atual: {sample.temperature_c:.0f} °C."
        if sample.temperature_c is not None
        else "",
        active=temperature_active,
    )
    _upsert_alert(
        db,
        device=device,
        alert_type="defender_disabled",
        severity="critical",
        title="Proteção antivírus desativada",
        message="O Microsoft Defender foi informado como desativado.",
        active=sample.defender_active is False,
    )
    _upsert_alert(
        db,
        device=device,
        alert_type="firewall_disabled",
        severity="critical",
        title="Firewall desativado",
        message="Um ou mais perfis do Firewall do Windows foram informados como desativados.",
        active=sample.firewall_active is False,
    )
