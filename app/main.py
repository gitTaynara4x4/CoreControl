from __future__ import annotations

from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI, HTTPException, Query
from fastapi.responses import FileResponse, RedirectResponse
from fastapi.staticfiles import StaticFiles
from sqlalchemy import select

from .api import router as api_router
from .config import settings
from .db import Base, SessionLocal, apply_runtime_migrations, engine
from .models import User
from .public_api import DOWNLOAD_FILENAME, router as public_router
from .password_reset import router as password_reset_router
from .security import decode_download_token, hash_password

BASE_DIR = Path(__file__).resolve().parent
DASHBOARD_DIR = BASE_DIR / "static"
PUBLIC_DIR = BASE_DIR / "public"
DOWNLOAD_DIR = BASE_DIR / "downloads"
REMOTE_ASSET_DIR = BASE_DIR / "remote_assets"


def validate_runtime_settings() -> None:
    if not settings.is_production:
        return
    problems: list[str] = []
    if settings.secret_key == "change-this-in-production" or settings.secret_key.startswith("COLOQUE_") or len(settings.secret_key) < 32:
        problems.append("CORETUNER_SECRET_KEY precisa ter pelo menos 32 caracteres e não pode usar o padrão")
    if not settings.download_password:
        problems.append("CORETUNER_DOWNLOAD_PASSWORD não foi configurada")
    if settings.admin_password == "TroqueAgora123!" or settings.admin_password.startswith("COLOQUE_") or len(settings.admin_password) < 12:
        problems.append("CORETUNER_ADMIN_PASSWORD precisa ser alterada e ter pelo menos 12 caracteres")
    if problems:
        raise RuntimeError("Configuração de produção inválida: " + "; ".join(problems))


def initialize_database() -> None:
    Base.metadata.create_all(bind=engine)
    apply_runtime_migrations()
    with SessionLocal() as db:
        admin = db.scalar(select(User).where(User.email == settings.admin_email.lower()))
        if not admin:
            db.add(
                User(
                    name="Administrador CoreTuner",
                    email=settings.admin_email.lower(),
                    password_hash=hash_password(settings.admin_password),
                    role="platform_admin",
                    company_id=None,
                )
            )
            db.commit()


@asynccontextmanager
async def lifespan(_: FastAPI):
    validate_runtime_settings()
    initialize_database()
    yield


app = FastAPI(
    title=settings.app_name,
    version="0.4.11",
    docs_url="/api/docs" if not settings.is_production else None,
    redoc_url=None,
    lifespan=lifespan,
)
app.include_router(public_router)
app.include_router(password_reset_router)
app.include_router(api_router)
app.mount("/static", StaticFiles(directory=DASHBOARD_DIR), name="static")
app.mount("/site", StaticFiles(directory=PUBLIC_DIR), name="site")


@app.get("/remote-assets/meshcentral-custom.js")
def meshcentral_custom_script():
    file_path = REMOTE_ASSET_DIR / "meshcentral-custom.js"
    if not file_path.exists() or not file_path.is_file():
        raise HTTPException(status_code=404, detail="Script remoto indisponível")
    return FileResponse(
        file_path,
        media_type="application/javascript; charset=utf-8",
        headers={
            "Cache-Control": "no-store, max-age=0",
            "X-Content-Type-Options": "nosniff",
        },
    )


@app.get("/health")
def health():
    return {"status": "ok", "app": settings.app_name, "version": "0.4.11"}


@app.get("/downloads/{filename}")
def protected_download(filename: str, token: str = Query(..., min_length=20)):
    if filename != DOWNLOAD_FILENAME:
        raise HTTPException(status_code=404, detail="Arquivo não encontrado")
    decode_download_token(token, filename)
    file_path = DOWNLOAD_DIR / filename
    if not file_path.exists() or not file_path.is_file():
        raise HTTPException(status_code=404, detail="Instalador do CoreTuner indisponível")
    return FileResponse(
        file_path,
        media_type="application/vnd.microsoft.portable-executable",
        filename=filename,
        headers={
            "Cache-Control": "no-store, private",
            "X-Content-Type-Options": "nosniff",
        },
    )


@app.get("/")
def landing_page():
    return FileResponse(PUBLIC_DIR / "index.html")


@app.get("/planos")
def plans_page():
    return RedirectResponse(url="/#planos", status_code=307)


@app.get("/entrar")
def enter_central():
    return RedirectResponse(url="/central", status_code=307)


@app.get("/central")
def central_index():
    return FileResponse(DASHBOARD_DIR / "index.html")


@app.get("/central/{path:path}")
def central_spa(path: str):
    # O painel atual navega em uma única página. Rotas futuras continuam caindo no shell da aplicação.
    return FileResponse(DASHBOARD_DIR / "index.html")
