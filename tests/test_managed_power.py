from pathlib import Path

from app.meshcentral import MeshCentralClient

ROOT = Path(__file__).resolve().parents[1]


def test_managed_off_uses_mesh_runcommand_and_keeps_system_awake(monkeypatch):
    client = MeshCentralClient()
    calls = []

    def fake_command(action, args=None, **kwargs):
        calls.append((action, list(args or []), kwargs))
        return "CORECONTROL_MANAGED_OFF_CONFIRMED"

    monkeypatch.setattr(client, "_meshctrl_command", fake_command)
    out = client.device_enter_managed_off("node//LUZIA")
    assert "CONFIRMED" in out
    action, args, kwargs = calls[0]
    assert action == "RunCommand"
    assert args[0:2] == ["--id", "node//LUZIA"]
    script = args[args.index("--run") + 1]
    assert "SetThreadExecutionState" in script
    assert "managed-off-keeper.ps1" in script
    assert "WTSDisconnectSession" in script
    assert "--powershell" in args
    assert "--reply" in args


def test_managed_on_stops_keeper_without_wol(monkeypatch):
    client = MeshCentralClient()
    calls = []

    def fake_command(action, args=None, **kwargs):
        calls.append((action, list(args or []), kwargs))
        return "CORECONTROL_MANAGED_ON_CONFIRMED"

    monkeypatch.setattr(client, "_meshctrl_command", fake_command)
    client.device_exit_managed_off("node//LUZIA")
    _, args, _ = calls[0]
    script = args[args.index("--run") + 1]
    assert "managed-off-keeper.ps1" in script
    assert "Stop-Process" in script
    assert "Wake-on-LAN" not in script


def test_api_prioritizes_native_agent_managed_power_without_mesh_runcommand():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    agent = (ROOT / "agent/src/update_windows.go").read_text(encoding="utf-8")
    main = (ROOT / "agent/src/main.go").read_text(encoding="utf-8")
    assert 'action="power.managed_off.entered"' in api
    assert 'action="power.managed_off.exited"' in api
    assert '_run_agent_command_sync(' in api
    assert '"power.managed_off"' in api
    assert '"power.managed_on"' in api
    assert 'managed_mode_available = bool(device_online(device) and _version_at_least(device.agent_version, (0, 9, 10)))' in api
    assert '"power_engine_version": "10.37"' in api
    assert 'case "power.managed_off":' in agent
    assert 'case "power.managed_on":' in agent
    assert 'CORECONTROL_MANAGED_OFF_CONFIRMED' in agent
    assert 'CORECONTROL_MANAGED_ON_CONFIRMED' in agent
    assert 'const agentVersion = "0.9.10"' in main


def test_frontend_treats_managed_off_as_desligado_and_hides_router_setup():
    ui = (ROOT / "app/static/js/ui.js").read_text(encoding="utf-8")
    devices = (ROOT / "app/static/js/pages/devices.js").read_text(encoding="utf-8")
    overview = (ROOT / "app/static/js/pages/overview.js").read_text(encoding="utf-8")
    assert "managed_off_active" in ui
    assert "managed_mode_available" in devices
    assert "Não necessária no modo CoreControl Off" in devices
    assert "wakeRouteButton.classList.add('hidden')" in devices
    assert "managed_mode_available" in overview


def test_managed_off_does_not_require_elevated_include_username():
    agent = (ROOT / "agent" / "src" / "update_windows.go").read_text(encoding="utf-8")
    assert "-IncludeUserName" not in agent
    assert "WTSDisconnectSession" not in agent[agent.index("func executeManagedOffCommand()"):agent.index("func mapFromStruct")]
    assert "SC_MONITORPOWER" in agent



def test_managed_off_turns_display_off_without_disconnect_or_lock():
    agent = (ROOT / "agent" / "src" / "update_windows.go").read_text(encoding="utf-8")
    native = agent[agent.index("func executeManagedOffCommand()"):agent.index("func mapFromStruct")]
    assert "SC_MONITORPOWER" in native
    assert "SendMessage" in native
    assert "managed-off.active" in native
    assert "WTSDisconnectSession" not in native
    assert "LockWorkStation" not in native
    assert "rundll32.exe user32.dll,LockWorkStation" not in native


def test_managed_on_reenables_display():
    agent = (ROOT / "agent" / "src" / "update_windows.go").read_text(encoding="utf-8")
    native_on = agent[agent.index("func executeManagedOnCommand()"):agent.index("func mapFromStruct")]
    assert "managed-off.active" in native_on
    assert "SC_MONITORPOWER" in native_on
    assert "[IntPtr](-1)" in native_on
