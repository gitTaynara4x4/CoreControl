from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def test_v1042_uses_software_economy_as_default_power_path():
    api = read("app/api.py")
    endpoint = api[api.index('@router.post("/devices/{device_id}/power")'):api.index('@router.get("/alerts")')]
    assert 'aliases = {"off": "economy", "wake": "activate"}' in endpoint
    assert 'requested not in {"economy", "activate", "shutdown"}' in endpoint
    assert '"power.managed_off"' in endpoint
    assert '"power.managed_on"' in endpoint
    assert 'find_power_gateways' not in endpoint
    assert 'CoreControl Box offline ou não instalada' not in endpoint
    assert '"power_engine_version": "10.43"' in api
    assert '"software_only_power": True' in api
    assert '"requires_verified_wake": False' in api


def test_economy_mode_is_not_reported_as_physically_offline():
    api = read("app/api.py")
    ui = read("app/static/js/ui.js")
    serialize = api[api.index("def serialize_device"):api.index("def sync_company_remote_devices")]
    effective = api[api.index("def device_effectively_online"):api.index("def device_power_currently_on")]
    assert 'online = bool(actual_online)' in serialize
    assert 'return bool(device_online(device))' in effective
    assert 'if (device?.power?.managed_off_active || device?.managed_off) return false;' not in ui
    assert 'CT.deviceEconomyModeActive' in ui


def test_agent_economy_keeps_agent_reachable_and_restores_power_plan():
    windows = read("agent/src/update_windows.go")
    managed = windows[windows.index("func executeManagedOffCommand()"):windows.index("func mapFromStruct")]
    assert "SetThreadExecutionState" in managed
    assert "SC_MONITORPOWER" in managed
    assert "powercfg.exe /getactivescheme" in managed
    assert "a1841308-3541-4fab-bc81-f71556f20b4a" in managed
    assert "economy-previous-scheme.txt" in managed
    assert "power_plan_restored" in managed
    assert "CORECONTROL_ECONOMY_CONFIRMED" in managed
    assert "shutdown.exe" not in managed


def test_full_shutdown_is_secondary_and_explicit():
    api = read("app/api.py")
    ui = read("app/static/js/ui.js")
    device_html = read("app/static/pages/device.html")
    assert 'requested not in {"economy", "activate", "shutdown"}' in api
    assert 'action="power.shutdown.sent"' in api
    assert 'remote_power_on_guaranteed": False' in api
    assert 'CT.confirmDeviceShutdown' in ui
    assert 'talvez seja necessário ligar este computador presencialmente' in ui
    assert 'id="deviceShutdownBtn"' in device_html
    assert 'Desligar completamente' in device_html


def test_customer_ui_says_economy_not_fake_off_or_required_box():
    overview = read("app/static/js/pages/overview.js")
    devices = read("app/static/js/pages/devices.js")
    companies = read("app/static/js/pages/companies.js")
    company_html = read("app/static/pages/company.html")
    assert "Modo econômico" in overview
    assert "Ativar computador" in overview
    assert "Modo econômico" in devices
    assert "Ativar computador" in devices
    assert "sem depender de roteador, Wake-on-LAN ou equipamento adicional" in company_html
    assert "Ela não é necessária para o Modo econômico" in companies


def test_release_versions_are_bumped():
    agent = read("agent/src/main.go")
    setup = read("desktop/setup/src/main.go")
    index = read("app/static/index.html")
    assert 'const agentVersion = "0.9.14"' in agent
    assert 'const appVersion = "0.4.22"' in setup
    assert 'const bundledAgentVersion = "0.9.14"' in setup
    assert "20260909-shutdown-confirm-v10-43" in index
