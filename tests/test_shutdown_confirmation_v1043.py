from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def test_backend_has_fast_post_shutdown_confirmation_state_machine():
    api = read("app/api.py")
    update_api = read("app/update_api.py")
    fn = api[api.index("def device_shutdown_lifecycle"):api.index("def device_managed_off_state")]
    assert "FULL_SHUTDOWN_TRANSITION_SECONDS = 18" in api
    assert "FULL_SHUTDOWN_HEARTBEAT_STALE_SECONDS = 12" in api
    assert "FULL_SHUTDOWN_FAILURE_SECONDS = 35" in api
    assert 'AgentCommand.command_type == "power.shutdown"' in fn
    assert '"state": "shutting_down"' in fn
    assert '"confirmation": "confirmed"' in fn
    assert '"state": "offline"' in fn
    assert 'minimum_seconds=4 if recent_shutdown else 20' in update_api
    assert 'Desligamento em andamento. Aguarde a confirmação' in api


def test_confirmed_shutdown_overrides_generic_three_minute_online_grace():
    api = read("app/api.py")
    effective = api[api.index("def device_effectively_online"):api.index("def device_power_currently_on")]
    serialize = api[api.index("def serialize_device"):api.index("def sync_company_remote_devices")]
    remote_status = api[api.index('@router.get("/devices/{device_id}/remote-status")'):api.index('@router.post("/devices/{device_id}/remote-session")')]
    assert 'shutdown.get("state") == "offline"' in effective
    assert 'shutdown.get("confirmation") == "confirmed"' in effective
    assert '"power_state": shutdown.get("state")' in serialize
    assert '"shutdown_confirmation": shutdown.get("confirmation")' in serialize
    assert 'actual_on = False' in remote_status
    assert 'shutdown.get("state") not in {"shutting_down", "offline"}' in remote_status


def test_frontend_shows_shutting_down_and_waits_for_server_confirmation():
    ui = read("app/static/js/ui.js")
    devices = read("app/static/js/pages/devices.js")
    overview = read("app/static/js/pages/overview.js")
    index = read("app/static/index.html")
    assert "CT.deviceShutdownState" in ui
    assert "CT.deviceShutdownConfirmed" in ui
    assert "status.shutdown_confirmation === 'confirmed'" in ui
    assert "Desligando..." in devices
    assert "Desligamento confirmado: o computador está offline." in devices
    assert "Desligando..." in overview
    assert "20260909-shutdown-confirm-v10-43" in index


def test_power_engine_bumped_without_requiring_agent_reinstall():
    api = read("app/api.py")
    agent = read("agent/src/main.go")
    assert '"power_engine_version": "10.43"' in api
    assert 'const agentVersion = "0.9.14"' in agent
