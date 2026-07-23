import os
import tempfile

_tmp = tempfile.TemporaryDirectory()
os.environ["CORETUNER_DATA_DIR"] = _tmp.name
os.environ["CORETUNER_SECRET_KEY"] = "test-secret-key-with-enough-length"
os.environ["CORETUNER_ADMIN_EMAIL"] = "admin@test.example.com"
os.environ["CORETUNER_ADMIN_PASSWORD"] = "TestPassword123!"
os.environ["CORETUNER_ENV"] = "development"
os.environ["CORETUNER_DOWNLOAD_PASSWORD"] = "test-download-password"
os.environ["CORETUNER_DOWNLOAD_MAX_ATTEMPTS"] = "20"
os.environ.pop("CORETUNER_DATABASE_URL", None)

from fastapi.testclient import TestClient  # noqa: E402
from app.main import app  # noqa: E402


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
                "agent_version": "0.4.3",
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
            },
        )
        assert telemetry.status_code == 200, telemetry.text

        devices = client.get("/api/devices")
        assert devices.status_code == 200
        assert len(devices.json()) == 1
        assert devices.json()[0]["telemetry"]["disk_percent"] == 93.0

        alerts = client.get("/api/alerts?status_filter=active")
        assert any(item["type"] == "disk_low" for item in alerts.json())


def test_download_requires_company_login_and_password():
    with TestClient(app) as client:
        unauthenticated = client.post(
            "/api/public/download-ticket", json={"password": "test-download-password"}
        )
        assert unauthenticated.status_code == 401

        register_company(client, "download")

        wrong = client.post("/api/public/download-ticket", json={"password": "0000"})
        assert wrong.status_code == 401

        unlocked = client.post(
            "/api/public/download-ticket", json={"password": "test-download-password"}
        )
        assert unlocked.status_code == 200, unlocked.text
        payload = unlocked.json()
        assert payload["filename"] == "CoreTunerSetup.exe"
        assert payload["download_url"].startswith("/downloads/CoreTunerSetup.exe?token=")

        download = client.get(payload["download_url"])
        assert download.status_code == 200
        assert download.content[:2] == b"MZ"
        assert len(download.content) > 5_000_000


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


def test_site_central_and_health_are_served():
    with TestClient(app) as client:
        landing = client.get("/")
        assert landing.status_code == 200
        assert "Criar empresa" in landing.text
        assert "CoreTuner Setup" in landing.text

        central = client.get("/central")
        assert central.status_code == 200
        assert "registerCompanyForm" in central.text

        site_js = client.get("/site/site.js")
        assert site_js.status_code == 200
        assert "register-company" in site_js.text
        assert "CoreTunerSetup.exe" in site_js.text

        health = client.get("/health")
        assert health.status_code == 200
        assert health.json()["version"] == "0.4.3"
