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


def test_api_prioritizes_software_only_managed_power_without_os_name_dependency():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    assert 'action="power.managed_off.entered"' in api
    assert 'action="power.managed_off.exited"' in api
    assert 'meshcentral_client.device_enter_managed_off' in api
    assert 'meshcentral_client.device_exit_managed_off' in api
    assert '"managed_mode_available": managed_mode_available' in api
    assert '"power_off_mode": "managed" if managed_mode_available' in api
    assert 'managed_mode_available = bool(mesh_fallback)' in api
    assert 'if readiness.get("managed_mode_available"):' in api
    assert 'and not readiness.get("managed_mode_available")' in api
    assert 'engine": "10.34"' in api


def test_frontend_treats_managed_off_as_desligado_and_hides_router_setup():
    ui = (ROOT / "app/static/js/ui.js").read_text(encoding="utf-8")
    devices = (ROOT / "app/static/js/pages/devices.js").read_text(encoding="utf-8")
    overview = (ROOT / "app/static/js/pages/overview.js").read_text(encoding="utf-8")
    assert "managed_off_active" in ui
    assert "managed_mode_available" in devices
    assert "Não necessária no modo CoreControl Off" in devices
    assert "wakeRouteButton.classList.add('hidden')" in devices
    assert "managed_mode_available" in overview
