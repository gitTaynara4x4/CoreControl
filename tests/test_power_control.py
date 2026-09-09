from pathlib import Path

import pytest

from app.meshcentral import MeshCentralClient, MeshCentralCommandError

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def test_meshcentral_legacy_power_commands_still_build_correctly(monkeypatch):
    client = MeshCentralClient()
    calls = []

    def fake_command(action, args=None, **kwargs):
        calls.append((action, list(args or []), kwargs))
        return "ok"

    monkeypatch.setattr(client, "_meshctrl_command", fake_command)
    assert client.device_power("node//ABC", "wake") == "ok"
    assert client.device_power("node//ABC", "off") == "ok"
    assert calls[0][0] == "DevicePower"
    assert calls[0][1] == ["--wake", "--id", "node//ABC"]
    assert calls[1][1] == ["--off", "--id", "node//ABC"]


def test_meshcentral_legacy_power_rejects_invalid_action(monkeypatch):
    client = MeshCentralClient()
    monkeypatch.setattr(client, "_meshctrl_command", lambda *args, **kwargs: "ok")
    with pytest.raises(MeshCentralCommandError):
        client.device_power("node//ABC", "reboot-now")


def test_default_power_controls_are_economy_and_activate():
    api = read("app/api.py")
    ui = read("app/static/js/ui.js")
    overview = read("app/static/js/pages/overview.js")
    devices = read("app/static/js/pages/devices.js")
    device_html = read("app/static/pages/device.html")

    assert '@router.post("/devices/{device_id}/power")' in api
    assert 'aliases = {"off": "economy", "wake": "activate"}' in api
    assert "CT.requestDevicePower" in ui
    assert "Modo econômico" in overview
    assert "Ativar computador" in overview
    assert "Modo econômico" in devices
    assert 'id="devicePowerBtn"' in device_html
    assert 'id="deviceShutdownBtn"' in device_html


def test_default_power_path_does_not_require_gateway_router_or_wol():
    api = read("app/api.py")
    endpoint = api[api.index('@router.post("/devices/{device_id}/power")'):api.index('@router.get("/alerts")')]
    assert 'find_power_gateways' not in endpoint
    assert 'corecontrol_box_wol' not in endpoint
    assert 'corecontrol_wan_upnp' not in endpoint
    assert 'meshcentral_wake' not in endpoint
    assert '"power.managed_off"' in endpoint
    assert '"power.managed_on"' in endpoint
    assert '"requires_verified_wake": False' in api
    assert '"software_only_power": True' in api


def test_native_agent_heartbeat_has_priority_over_mesh_power_state():
    api = read("app/api.py")
    ui = read("app/static/js/ui.js")
    fn = api[api.index("def device_power_currently_on"):api.index("def device_power_pending_state")]
    assert "agent_power_fresh" in fn
    assert "<= 30" in fn
    assert fn.index("if agent_power_fresh:") < fn.index("if mesh_ready and mesh_recent:")
    assert "if (device?.actual_online || device?.online) return true;" in ui


def test_economy_is_not_presented_as_powered_off():
    api = read("app/api.py")
    ui = read("app/static/js/ui.js")
    assert 'online = bool(actual_online)' in api
    assert 'return bool(device_online(device))' in api
    assert "CT.deviceEconomyModeActive" in ui
    assert "managed_off_active || device?.managed_off) return false" not in ui


def test_agent_uses_software_economy_and_keeps_machine_reachable():
    windows = read("agent/src/update_windows.go")
    main = read("agent/src/main.go")
    native = windows[windows.index("func executeManagedOffCommand()"):windows.index("func mapFromStruct")]
    assert 'const agentVersion = "0.9.14"' in main
    assert 'case "power.managed_off":' in windows
    assert 'case "power.managed_on":' in windows
    assert "SetThreadExecutionState" in native
    assert "SC_MONITORPOWER" in native
    assert "powercfg.exe /getactivescheme" in native
    assert "economy-previous-scheme.txt" in native
    assert "CORECONTROL_ECONOMY_CONFIRMED" in native


def test_full_shutdown_remains_explicit_secondary_action():
    api = read("app/api.py")
    ui = read("app/static/js/ui.js")
    windows = read("agent/src/update_windows.go")
    assert 'requested not in {"economy", "activate", "shutdown"}' in api
    assert 'action="power.shutdown.sent"' in api
    assert 'remote_power_on_guaranteed": False' in api
    assert "CT.confirmDeviceShutdown" in ui
    assert "talvez seja necessário ligar este computador presencialmente" in ui
    assert 'case "power.shutdown":' in windows
    assert "shutdown.exe" in windows


def test_setup_reports_bundled_agent_and_production_server():
    setup = read("desktop/setup/src/main.go")
    desktop = read("desktop/app/src/main.go")
    build = read("desktop/Build_Windows.ps1")
    production = "https://apps-corecontrol.9ywrah.easypanel.host"
    assert 'const appVersion = "0.4.22"' in setup
    assert 'const bundledAgentVersion = "0.9.14"' in setup
    assert '"agent_version": bundledAgentVersion' in setup
    assert f'var defaultServerURL = "{production}"' in setup
    assert f'var defaultServerURL = "{production}"' in desktop
    assert f"else {{ '{production}' }}" in build


def test_wol_diagnostics_can_remain_optional_without_entering_default_path():
    api = read("app/api.py")
    devices = read("app/static/js/pages/devices.js")
    assert '@router.post("/devices/{device_id}/wake-route-test")' in api
    assert "wakeRouteButton.classList.add('hidden')" in devices
    assert "Wake-on-LAN opcional" in devices
