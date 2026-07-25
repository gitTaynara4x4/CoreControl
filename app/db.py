from __future__ import annotations

from pathlib import Path

from sqlalchemy import create_engine, inspect, text
from sqlalchemy.orm import DeclarativeBase, sessionmaker

from .config import settings


if settings.database_url.startswith("sqlite:///"):
    db_path = settings.database_url.replace("sqlite:///", "", 1)
    Path(db_path).parent.mkdir(parents=True, exist_ok=True)

connect_args = {"check_same_thread": False} if settings.database_url.startswith("sqlite") else {}
engine = create_engine(settings.database_url, connect_args=connect_args, pool_pre_ping=True)
SessionLocal = sessionmaker(bind=engine, autoflush=False, autocommit=False, expire_on_commit=False)


class Base(DeclarativeBase):
    pass


_RUNTIME_COLUMNS: dict[str, dict[str, str]] = {
    "companies": {
        "mesh_group_id": "VARCHAR(190) NULL",
        "mesh_group_name": "VARCHAR(190) NULL",
        "mesh_group_synced_at": "TIMESTAMP NULL",
    },
    "devices": {
        "mesh_node_id": "VARCHAR(190) NULL",
        "remote_online": "BOOLEAN NOT NULL DEFAULT FALSE",
        "remote_checked_at": "TIMESTAMP NULL",
        "remote_last_seen": "TIMESTAMP NULL",
    },
}


def apply_runtime_migrations() -> None:
    """Apply small, additive upgrades for existing SQLite/PostgreSQL installs.

    SQLAlchemy create_all creates missing tables but does not add columns to an
    existing table. These ALTER statements are intentionally additive and
    idempotent so a normal deploy upgrades the current CoreTuner database.
    """
    inspector = inspect(engine)
    existing_tables = set(inspector.get_table_names())
    with engine.begin() as connection:
        for table_name, columns in _RUNTIME_COLUMNS.items():
            if table_name not in existing_tables:
                continue
            present = {column["name"].lower() for column in inspector.get_columns(table_name)}
            for column_name, definition in columns.items():
                if column_name.lower() in present:
                    continue
                connection.execute(text(f'ALTER TABLE "{table_name}" ADD COLUMN "{column_name}" {definition}'))


def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
