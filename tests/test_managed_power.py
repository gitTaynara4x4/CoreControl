from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def test_managed_commands_are_the_compatible_transport_for_economy_mode():
    api = read("app/api.py")
    agent = read("agent/src/update_windows.go")
    assert '"power.managed_off"' in api
    assert '"power.managed_on"' in api
    assert 'case "power.managed_off":' in agent
    assert 'case "power.managed_on":' in agent
    assert '"power_engine_version": "10.42"' in api


def test_economy_keeps_system_awake_turns_display_off_and_uses_power_saver():
    agent = read("agent/src/update_windows.go")
    native = agent[agent.index("func executeManagedOffCommand()"):agent.index("func executeManagedOnCommand()")]
    assert "SetThreadExecutionState" in native
    assert "ES_SYSTEM_REQUIRED" in native
    assert "SC_MONITORPOWER" in native
    assert "managed-off.active" in native
    assert "powercfg.exe /getactivescheme" in native
    assert "a1841308-3541-4fab-bc81-f71556f20b4a" in native
    assert "WTSDisconnectSession" not in native
    assert "LockWorkStation" not in native


def test_activate_restores_previous_plan_and_display():
    agent = read("agent/src/update_windows.go")
    native = agent[agent.index("func executeManagedOnCommand()"):agent.index("func mapFromStruct")]
    assert "economy-previous-scheme.txt" in native
    assert "powercfg.exe /setactive $scheme" in native
    assert "SC_MONITORPOWER" in native
    assert "[IntPtr](-1)" in native
    assert '"power_plan_restored": true' in native


def test_frontend_calls_state_economy_instead_of_fake_desligado():
    ui = read("app/static/js/ui.js")
    devices = read("app/static/js/pages/devices.js")
    overview = read("app/static/js/pages/overview.js")
    assert "CT.deviceEconomyModeActive" in ui
    assert "Modo econômico" in devices
    assert "Modo econômico" in overview
    assert "Ativar computador" in devices
    assert "Ativar computador" in overview
