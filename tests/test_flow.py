import os
import tempfile
from datetime import datetime, timezone

_tmp = tempfile.TemporaryDirectory()
os.environ["CORETUNER_DATA_DIR"] = _tmp.name
os.environ["CORETUNER_SECRET_KEY"] = "test-secret-key-with-enough-length"
os.environ["CORECONTROL_GLOBAL_ADMIN_EMAIL"] = "global@test.example.com"
os.environ["CORECONTROL_GLOBAL_ADMIN_PASSWORD_HASH"] = "pbkdf2_sha256$310000$YCmI7xMMracfxRlNkhBrCQ==$IQNDwsLqyqNIQSZBFwJmDbS4eKOKTL5suO23x6cW7PA="
os.environ["CORETUNER_ENV"] = "development"
os.environ["CORETUNER_DOWNLOAD_PASSWORD"] = "test-download-password"
os.environ["CORETUNER_DOWNLOAD_MAX_ATTEMPTS"] = "20"
os.environ["CORETUNER_SMTP_HOST"] = "smtp.test.example.com"
os.environ["CORETUNER_SMTP_PORT"] = "587"
os.environ["CORETUNER_SMTP_USER"] = "sender@test.example.com"
os.environ["CORETUNER_SMTP_PASSWORD"] = "test-app-password"
os.environ["CORETUNER_SMTP_FROM_EMAIL"] = "sender@test.example.com"
os.environ["CORETUNER_REMOTE_ENABLED"] = "true"
os.environ["CORETUNER_REMOTE_URL"] = "https://remote.test.example.com"
os.environ["CORETUNER_REMOTE_LOGIN_TOKEN_KEY"] = "11" * 80
os.environ["CORETUNER_REMOTE_LOGIN_USER"] = "coretuner-integracao"
os.environ["CORETUNER_REMOTE_LOGIN_DOMAIN"] = ""
os.environ["CORETUNER_REMOTE_LOGIN_TOKEN_MINUTES"] = "2"
os.environ["CORETUNER_REMOTE_ADMIN_USER"] = ""
os.environ["CORETUNER_DATABASE_URL"] = f"sqlite:///{_tmp.name}/coretuner-test.db"

from fastapi.testclient import TestClient  # noqa: E402
from app.main import app  # noqa: E402
from app.db import SessionLocal  # noqa: E402
from app.models import Device, User  # noqa: E402
from app.security import hash_password  # noqa: E402


def seed_test_platform_accounts():
    with SessionLocal() as db:
        if not db.query(User).filter(User.email == "global@test.example.com").first():
            db.add(
                User(
                    name="Administrador Global",
                    email="global@test.example.com",
                    password_hash=hash_password("SuperAdminTest123!"),
                    role="global_admin",
                    company_id=None,
                    active=True,
                )
            )
        if not db.query(User).filter(User.email == "admin@test.example.com").first():
            db.add(
                User(
                    name="Administrador CoreControl",
                    email="admin@test.example.com",
                    password_hash=hash_password("TestPassword123!"),
                    role="platform_admin",
                    company_id=None,
                    active=True,
                )
            )
        db.commit()


def register_company(client, suffix="1"):
    response = client.post(
        "/api/auth/register-company",
        json={
            "company_name": f"Empresa Cliente {suffix}",
            "responsible_name": f"Responsável {suffix}",
            "email": f"cliente-{suffix}@example.com",
            "password": "CompanyPassword123!",
            "password_confirmation": "CompanyPassword123!",
        },
    )
    assert response.status_code == 201, response.text
    return response.json()


def test_company_self_registration_login_and_isolation():
    with TestClient(app) as client_a:
        with SessionLocal() as db:
            assert db.query(User).filter(User.email.in_(["admin@test.example.com", "global@test.example.com"])).count() == 0
        auth_a = register_company(client_a, "a")
        assert auth_a["user"]["role"] == "company_admin"
        assert auth_a["company"]["name"] == "Empresa Cliente a"
        assert auth_a["access_token"]

        me = client_a.get("/api/auth/me")
        assert me.status_code == 200
        company_a_id = me.json()["company_id"]

    with TestClient(app) as client_b:
        auth_b = register_company(client_b, "b")
        company_b_id = auth_b["company"]["id"]
        assert company_b_id != company_a_id

        forbidden = client_b.get(f"/api/companies/{company_a_id}")
        assert forbidden.status_code == 403

    with TestClient(app) as login_client:
        login = login_client.post(
            "/api/auth/login",
            json={"email": "cliente-a@example.com", "password": "CompanyPassword123!"},
        )
        assert login.status_code == 200, login.text
        assert login.json()["company"]["id"] == company_a_id



def test_global_admin_sees_edits_and_can_destroy_companies():
    with TestClient(app) as client:
        seed_test_platform_accounts()
        login = client.post(
            "/api/auth/login",
            json={"email": "global@test.example.com", "password": "SuperAdminTest123!"},
        )
        assert login.status_code == 200, login.text
        assert login.json()["user"]["role"] == "global_admin"
        assert login.json()["user"]["company_id"] is None

        company_a = client.post("/api/companies", json={"name": "Global Empresa A"})
        company_b = client.post("/api/companies", json={"name": "Global Empresa B"})
        assert company_a.status_code == 201, company_a.text
        assert company_b.status_code == 201, company_b.text
        company_a_id = company_a.json()["id"]
        company_b_id = company_b.json()["id"]

        renamed = client.patch(
            f"/api/companies/{company_a_id}",
            json={"name": "Global Empresa A Editada", "active": True},
        )
        assert renamed.status_code == 200, renamed.text
        assert renamed.json()["name"] == "Global Empresa A Editada"

        installed = client.post(
            "/api/devices/install",
            json={
                "company_id": company_a_id,
                "device_uid": "global-device-001",
                "name": "PC ADM 01",
                "hostname": "PC-ADM-01",
                "sector": "Administrativo",
                "install_remote": False,
            },
        )
        assert installed.status_code == 201, installed.text
        device_id = installed.json()["device_id"]

        moved = client.patch(
            f"/api/devices/{device_id}",
            json={
                "company_id": company_b_id,
                "name": "PC ADM 01 Editado",
                "sector": "Financeiro",
                "active": True,
            },
        )
        assert moved.status_code == 200, moved.text
        assert moved.json()["company_id"] == company_b_id
        assert moved.json()["company_name"] == "Global Empresa B"
        assert moved.json()["sector"] == "Financeiro"

        created_user = client.post(
            "/api/users",
            json={
                "name": "Gestor Global",
                "email": "gestor-global@example.com",
                "password": "CompanyPassword123!",
                "role": "company_admin",
                "company_id": company_b_id,
            },
        )
        assert created_user.status_code == 201, created_user.text

        updated_user = client.patch(
            f"/api/users/{created_user.json()['id']}",
            json={"name": "Gestor Editado", "active": False},
        )
        assert updated_user.status_code == 200, updated_user.text
        assert updated_user.json()["name"] == "Gestor Editado"
        assert updated_user.json()["active"] is False

        companies = client.get("/api/companies")
        assert companies.status_code == 200
        company_ids = {item["id"] for item in companies.json()}
        assert company_a_id in company_ids
        assert company_b_id in company_ids

        company_detail = client.get(f"/api/companies/{company_b_id}")
        assert company_detail.status_code == 200
        assert any(item["id"] == device_id for item in company_detail.json()["devices"])

        devices = client.get("/api/devices")
        assert devices.status_code == 200
        selected = next(item for item in devices.json() if item["id"] == device_id)
        assert selected["company_name"] == "Global Empresa B"

        wrong_confirmation = client.request(
            "DELETE",
            f"/api/companies/{company_b_id}",
            json={"confirmation": "EXCLUIR empresa errada"},
        )
        assert wrong_confirmation.status_code == 400

        destroyed = client.request(
            "DELETE",
            f"/api/companies/{company_b_id}",
            json={"confirmation": "EXCLUIR Global Empresa B"},
        )
        assert destroyed.status_code == 200, destroyed.text
        assert destroyed.json()["deleted"]["devices"] == 1
        assert destroyed.json()["deleted"]["users"] == 1

        missing_company = client.get(f"/api/companies/{company_b_id}")
        assert missing_company.status_code == 404
        remaining_devices = client.get("/api/devices")
        assert all(item["id"] != device_id for item in remaining_devices.json())

        platform_login = client.post(
            "/api/auth/logout"
        )
        assert platform_login.status_code == 200
        admin_login = client.post(
            "/api/auth/login",
            json={"email": "admin@test.example.com", "password": "TestPassword123!"},
        )
        assert admin_login.status_code == 200, admin_login.text
        forbidden_destroy = client.request(
            "DELETE",
            f"/api/companies/{company_a_id}",
            json={"confirmation": "EXCLUIR Global Empresa A Editada"},
        )
        assert forbidden_destroy.status_code == 403

def test_setup_directly_registers_current_device_and_agent_sends_telemetry():
    with TestClient(app) as client:
        auth = register_company(client, "device")
        token = auth["access_token"]

        install = client.post(
            "/api/devices/install",
            headers={"Authorization": f"Bearer {token}"},
            json={
                "device_uid": "machine-guid-setup-test",
                "name": "PC ATENDIMENTO 01",
                "hostname": "DESKTOP-TEST",
                "sector": "Telemarketing",
                "location": "Unidade Taubaté",
                "manufacturer": "Dell",
                "model": "OptiPlex",
                "serial_number": "SERIAL001",
                "os_name": "Windows 11 Pro",
                "os_version": "10.0",
                "agent_version": "0.4.11",
                "install_remote": False,
            },
        )
        assert install.status_code == 201, install.text
        secret = install.json()["agent_secret"]
        assert secret.startswith("ctagt_")

        telemetry = client.post(
            "/api/agent/telemetry",
            headers={"Authorization": f"Bearer {secret}"},
            json={
                "device_uid": "machine-guid-setup-test",
                "cpu_percent": 35.2,
                "memory_percent": 64.1,
                "memory_used_gb": 5.1,
                "memory_total_gb": 8.0,
                "disk_percent": 93.0,
                "disk_free_gb": 8.0,
                "disk_total_gb": 120.0,
                "defender_active": True,
                "firewall_active": True,
                "profile": "Nenhum",
                "extra": {
                    "remote_agent_installed": True,
                    "remote_agent_running": True,
                    "remote_service_name": "Mesh Agent",
                },
            },
        )
        assert telemetry.status_code == 200, telemetry.text

        devices = client.get("/api/devices")
        assert devices.status_code == 200
        assert len(devices.json()) == 1
        assert devices.json()[0]["telemetry"]["disk_percent"] == 93.0
        assert devices.json()[0]["remote"]["available"] is False

        with SessionLocal() as db:
            device = db.get(Device, devices.json()[0]["id"])
            device.mesh_node_id = "node//NODETEST123"
            device.remote_online = True
            device.remote_checked_at = datetime.now(timezone.utc)
            device.remote_last_seen = datetime.now(timezone.utc)
            db.commit()

        devices = client.get("/api/devices")
        assert devices.json()[0]["remote"]["available"] is True

        remote = client.post(f"/api/devices/{devices.json()[0]['id']}/remote-session")
        assert remote.status_code == 200, remote.text
        assert "gotonode=NODETEST123" in remote.json()["url"]
        assert "ctnode=NODETEST123" in remote.json()["url"]
        assert "viewmode=11" in remote.json()["url"]
        assert "hide=63" in remote.json()["url"]
        assert "login=" in remote.json()["url"]
        assert remote.json()["embedded"] is True

        alerts = client.get("/api/alerts?status_filter=active")
        assert any(item["type"] == "disk_low" for item in alerts.json())


def test_download_requires_company_login_and_password():
    with TestClient(app) as client:
        unauthenticated = client.post(
            "/api/public/download-ticket", json={"password": "test-download-password"}
        )
        assert unauthenticated.status_code == 401

        register_company(client, "download")

        manifest = client.get("/api/desktop/manifest")
        assert manifest.status_code == 200, manifest.text
        assert "CoreControl.exe" in manifest.json()["files"]
        assert "CoreControlAgent.exe" in manifest.json()["files"]
        assert "CoreTunerRemoteAgent.exe" not in manifest.json()["files"]

        wrong = client.post("/api/public/download-ticket", json={"password": "0000"})
        assert wrong.status_code == 401

        unlocked = client.post(
            "/api/public/download-ticket", json={"password": "test-download-password"}
        )
        assert unlocked.status_code == 200, unlocked.text
        payload = unlocked.json()
        assert payload["filename"] == "CoreControlSetup.exe"
        assert payload["download_url"].startswith("/downloads/CoreControlSetup.exe?token=")

        download = client.get(payload["download_url"])
        assert download.status_code == 200
        assert download.content[:2] == b"MZ"
        assert len(download.content) > 5_000_000


def test_remote_install_returns_clear_warning_when_provisioning_is_not_configured():
    with TestClient(app) as client:
        auth = register_company(client, "remote-warning")
        token = auth["access_token"]
        response = client.post(
            "/api/devices/install",
            headers={"Authorization": f"Bearer {token}"},
            json={
                "device_uid": "machine-guid-remote-warning",
                "name": "PC REMOTO",
                "hostname": "DESKTOP-REMOTE",
                "agent_version": "0.4.11",
                "install_remote": True,
            },
        )
        assert response.status_code == 201, response.text
        payload = response.json()
        assert payload["remote_agent"] is None
        assert "automação remota" in payload["remote_warning"].lower()


def test_duplicate_email_and_password_confirmation_are_rejected():
    with TestClient(app) as client:
        register_company(client, "duplicate")
        duplicate = client.post(
            "/api/auth/register-company",
            json={
                "company_name": "Outra Empresa",
                "responsible_name": "Outra Pessoa",
                "email": "cliente-duplicate@example.com",
                "password": "CompanyPassword123!",
                "password_confirmation": "CompanyPassword123!",
            },
        )
        assert duplicate.status_code == 409

        mismatch = client.post(
            "/api/auth/register-company",
            json={
                "company_name": "Empresa Senha",
                "responsible_name": "Pessoa Senha",
                "email": "senha@example.com",
                "password": "CompanyPassword123!",
                "password_confirmation": "OutraPassword123!",
            },
        )
        assert mismatch.status_code == 422


def test_password_reset_email_token_and_single_use(monkeypatch):
    sent = {}

    def fake_send_password_reset_email(*, recipient_name, recipient_email, reset_url):
        sent["name"] = recipient_name
        sent["email"] = recipient_email
        sent["url"] = reset_url

    monkeypatch.setattr(
        "app.password_reset.send_password_reset_email",
        fake_send_password_reset_email,
    )

    with TestClient(app) as client:
        register_company(client, "reset")
        client.post("/api/auth/logout")

        requested = client.post(
            "/api/auth/password-reset/request",
            json={"email": "cliente-reset@example.com"},
        )
        assert requested.status_code == 200, requested.text
        assert "Se o e-mail" in requested.json()["message"]
        assert sent["email"] == "cliente-reset@example.com"
        token = sent["url"].split("reset_token=", 1)[1]

        changed = client.post(
            "/api/auth/password-reset/confirm",
            json={
                "token": token,
                "password": "NewCompanyPassword123!",
                "password_confirmation": "NewCompanyPassword123!",
            },
        )
        assert changed.status_code == 200, changed.text

        old_login = client.post(
            "/api/auth/login",
            json={"email": "cliente-reset@example.com", "password": "CompanyPassword123!"},
        )
        assert old_login.status_code == 401

        new_login = client.post(
            "/api/auth/login",
            json={"email": "cliente-reset@example.com", "password": "NewCompanyPassword123!"},
        )
        assert new_login.status_code == 200, new_login.text

        reused = client.post(
            "/api/auth/password-reset/confirm",
            json={
                "token": token,
                "password": "AnotherPassword123!",
                "password_confirmation": "AnotherPassword123!",
            },
        )
        assert reused.status_code == 400


def test_site_central_and_health_are_served():
    with TestClient(app) as client:
        landing = client.get("/")
        assert landing.status_code == 200
        assert "Criar empresa" in landing.text
        assert "CoreControl Setup" in landing.text

        central = client.get("/central")
        assert central.status_code == 200
        assert 'id="loginMount"' in central.text

        login_page = client.get("/static/pages/login.html")
        assert login_page.status_code == 200
        assert "registerCompanyForm" in login_page.text

        site_js = client.get("/site/site.js")
        assert site_js.status_code == 200
        assert "register-company" in site_js.text
        assert "CoreControlSetup.exe" in site_js.text

        health = client.get("/health")
        assert health.status_code == 200
        assert health.json()["version"] == "0.4.11"


def test_updates_queue_agent_scan_inventory_install_and_policy():
    with TestClient(app) as client:
        auth = register_company(client, "updates")
        token = auth["access_token"]
        headers = {"Authorization": f"Bearer {token}"}

        install = client.post(
            "/api/devices/install",
            headers=headers,
            json={
                "device_uid": "machine-updates-001",
                "name": "PC UPDATE 01",
                "hostname": "PC-UPDATE-01",
                "agent_version": "0.5.0",
                "install_remote": False,
            },
        )
        assert install.status_code == 201, install.text
        device_id = install.json()["device_id"]
        agent_secret = install.json()["agent_secret"]
        agent_headers = {"Authorization": f"Bearer {agent_secret}"}

        queued = client.post(
            "/api/updates/check",
            headers=headers,
            json={"device_ids": [device_id]},
        )
        assert queued.status_code == 200, queued.text
        assert queued.json()["queued"] == 1

        command = client.get(
            "/api/agent/commands/next",
            headers=agent_headers,
            params={"device_uid": "machine-updates-001"},
        )
        assert command.status_code == 200, command.text
        scan_command = command.json()["command"]
        assert scan_command["type"] == "updates.scan"

        scan_result = client.post(
            f"/api/agent/commands/{scan_command['id']}/result",
            headers=agent_headers,
            json={
                "device_uid": "machine-updates-001",
                "ok": True,
                "result": {
                    "windows": [
                        {"id": "win-guid-1", "title": "Atualização cumulativa", "kb": "5030001", "severity": "Critical"}
                    ],
                    "drivers": [
                        {"id": "drv-guid-1", "title": "Driver de vídeo"}
                    ],
                    "apps": [
                        {"id": "Vendor.App", "title": "Aplicativo", "current_version": "1.0", "available_version": "1.1"}
                    ],
                    "reboot_required": False,
                    "warnings": [],
                },
            },
        )
        assert scan_result.status_code == 200, scan_result.text

        detail = client.get(f"/api/updates/devices/{device_id}", headers=headers)
        assert detail.status_code == 200, detail.text
        data = detail.json()
        assert data["windows_pending"] == 1
        assert data["driver_pending"] == 1
        assert data["app_pending"] == 1
        assert data["critical_pending"] == 1
        keys = {item["key"] for item in data["items"]}
        assert keys == {"windows:win-guid-1", "driver:drv-guid-1", "app:Vendor.App"}

        install_queue = client.post(
            "/api/updates/install",
            headers=headers,
            json={"device_id": device_id, "item_keys": ["windows:win-guid-1", "app:Vendor.App"]},
        )
        assert install_queue.status_code == 200, install_queue.text

        install_command_response = client.get(
            "/api/agent/commands/next",
            headers=agent_headers,
            params={"device_uid": "machine-updates-001"},
        )
        install_command = install_command_response.json()["command"]
        assert install_command["type"] == "updates.install"
        assert install_command["payload"]["windows_ids"] == ["win-guid-1"]
        assert install_command["payload"]["app_ids"] == ["Vendor.App"]
        assert install_command["payload"]["driver_ids"] == []

        completed = client.post(
            f"/api/agent/commands/{install_command['id']}/result",
            headers=agent_headers,
            json={
                "device_uid": "machine-updates-001",
                "ok": True,
                "result": {"installed": [], "failed": [], "reboot_required": True, "warnings": []},
            },
        )
        assert completed.status_code == 200, completed.text

        rescan = client.get(
            "/api/agent/commands/next",
            headers=agent_headers,
            params={"device_uid": "machine-updates-001"},
        )
        assert rescan.status_code == 200
        assert rescan.json()["command"]["type"] == "updates.scan"

        company_id = auth["company"]["id"]
        policy = client.post(
            "/api/updates/policies",
            headers=headers,
            json={
                "name": "Janela noturna",
                "auto_scan": True,
                "auto_install": False,
                "include_windows": True,
                "include_drivers": False,
                "include_apps": False,
                "scan_interval_hours": 24,
                "allowed_days": [0, 1, 2, 3, 4],
                "start_hour": 1,
                "end_hour": 5,
                "timezone": "America/Sao_Paulo",
            },
        )
        assert policy.status_code == 201, policy.text
        assert policy.json()["company_id"] == company_id
        policies = client.get("/api/updates/policies", headers=headers)
        assert policies.status_code == 200
        assert any(item["name"] == "Janela noturna" for item in policies.json())
