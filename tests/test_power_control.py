from pathlib import Path

import pytest

from app.meshcentral import MeshCentralClient, MeshCentralCommandError


ROOT = Path(__file__).resolve().parents[1]


def test_meshcentral_device_power_builds_wake_and_off_commands(monkeypatch):
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


def test_meshcentral_device_power_rejects_invalid_action(monkeypatch):
    client = MeshCentralClient()
    monkeypatch.setattr(client, "_meshctrl_command", lambda *args, **kwargs: "ok")
    with pytest.raises(MeshCentralCommandError):
        client.device_power("node//ABC", "reboot-now")


def test_power_controls_exist_in_overview_and_device_detail():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    ui = (ROOT / "app/static/js/ui.js").read_text(encoding="utf-8")
    overview = (ROOT / "app/static/js/pages/overview.js").read_text(encoding="utf-8")
    devices = (ROOT / "app/static/js/pages/devices.js").read_text(encoding="utf-8")
    device_html = (ROOT / "app/static/pages/device.html").read_text(encoding="utf-8")

    assert '@router.post("/devices/{device_id}/power")' in api
    assert "CT.requestDevicePower" in ui
    assert "CT.waitForDevicePower" in ui
    assert 'data-ops="power"' in overview
    assert "Ligar computador" in overview
    assert "Desligar computador" in overview
    assert 'id="devicePowerBtn"' in device_html
    assert "devicePowerBtn" in devices


def test_power_control_has_verified_lan_relay_fallback():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    agent = (ROOT / "agent/src/main.go").read_text(encoding="utf-8")
    wol = (ROOT / "agent/src/wol.go").read_text(encoding="utf-8")
    windows_commands = (ROOT / "agent/src/update_windows.go").read_text(encoding="utf-8")

    assert '@router.get("/devices/{device_id}/power-readiness")' in api
    assert '"power.wake_peer"' in api
    assert 'corecontrol_lan_relay' in api
    assert 'Desligamento bloqueado por segurança' in api
    assert '"primary_mac"' in agent
    assert '"network_cidr"' in agent
    assert '"wol_relay_capable"' in agent
    assert 'func sendWakeOnLAN' in wol
    assert 'case "power.wake_peer"' in windows_commands


def test_frontend_requires_safe_wake_route_before_shutdown():
    ui = (ROOT / "app/static/js/ui.js").read_text(encoding="utf-8")
    overview = (ROOT / "app/static/js/pages/overview.js").read_text(encoding="utf-8")
    devices = (ROOT / "app/static/js/pages/devices.js").read_text(encoding="utf-8")

    assert '/power-readiness' in ui
    assert 'readiness.safe_to_power_off' in ui
    assert 'Wake Relay verificado' in ui
    assert 'powerState.safe_to_power_off' in overview
    assert 'powerState.safe_to_power_off' in devices


def test_agent_097_audits_and_prepares_wol_without_claiming_full_shutdown_guarantee():
    main = (ROOT / "agent/src/main.go").read_text(encoding="utf-8")
    windows = (ROOT / "agent/src/wol_capability_windows.go").read_text(encoding="utf-8")
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")

    assert 'const agentVersion = "0.9.7"' in main
    assert '"wol_capability"' in main
    assert "Get-NetAdapterPowerManagement" in windows
    assert "Set-NetAdapterPowerManagement" in windows
    assert "powercfg.exe /deviceenablewake" in windows
    assert "wake_armed" in windows
    assert "intel_amt_detected" in windows
    assert "pc_wol_prepared" in api
    assert "rota externa confirmada" in api


def test_device_detail_exposes_wol_preflight_status():
    devices = (ROOT / "app/static/js/pages/devices.js").read_text(encoding="utf-8")
    assert "Wake-on-LAN" in devices
    assert "Magic Packet" in devices
    assert "Placa armada para wake" in devices
    assert "Intel AMT / vPro" in devices
    assert "Rota para ligar após desligar" in devices


def test_agent_097_ignores_virtual_vpn_adapters_for_primary_wol_nic():
    windows = (ROOT / "agent/src/process_windows.go").read_text(encoding="utf-8")
    assert "defaultRouteLocalIPv4" in windows
    assert "isVirtualNetworkInterface" in windows
    assert '"radmin"' in windows
    assert '"famatech"' in windows
    assert '"tailscale"' in windows
    assert '"zerotier"' in windows
    assert '"hyper-v"' in windows
    assert '"virtualbox"' in windows
    assert '"wireguard"' in windows
    assert "!candidate.Virtual" in windows
    preflight = (ROOT / "agent/src/wol_capability_windows.go").read_text(encoding="utf-8")
    assert "Is-PhysicalAdapter" in preflight
    assert "$virtualPattern" in preflight
    assert "Get-NetRoute" in preflight
    assert "Nenhuma placa de rede física ativa" in preflight


def test_server_prefers_mac_selected_by_wol_preflight():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    assert 'normalize_mac(capability.get("mac_address")) or normalize_mac(extra.get("primary_mac"))' in api


def test_wan_wol_route_probe_is_verified_from_vps_before_shutdown_is_unlocked():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    update_api = (ROOT / "app/update_api.py").read_text(encoding="utf-8")
    windows_commands = (ROOT / "agent/src/update_windows.go").read_text(encoding="utf-8")
    route = (ROOT / "agent/src/wol_route_windows.go").read_text(encoding="utf-8")
    devices = (ROOT / "app/static/js/pages/devices.js").read_text(encoding="utf-8")
    device_html = (ROOT / "app/static/pages/device.html").read_text(encoding="utf-8")

    assert '@router.post("/devices/{device_id}/wake-route-test")' in api
    assert 'latest_wan_wake_route' in api
    assert 'corecontrol_wan_upnp' in api
    assert '_send_wan_magic_packet' in api
    assert 'power.route_probe_confirm' in update_api
    assert '_send_wake_route_probe' in update_api
    assert 'case "power.route_probe"' in windows_commands
    assert 'case "power.route_probe_confirm"' in windows_commands
    assert 'AddPortMapping' in route
    assert '239.255.255.250' in route
    assert 'CoreControl Wake Route' in route
    assert 'id="wakeRouteTestBtn"' in device_html
    assert 'Testar rota de ligamento' in devices
    assert 'Rota para ligar confirmada' in devices


def test_wan_route_only_accepts_public_ipv4_and_high_udp_port():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    update_api = (ROOT / "app/update_api.py").read_text(encoding="utf-8")
    assert 'parsed_ip.is_global' in api
    assert '40000 <= external_port <= 59999' in api
    assert 'parsed.is_global' in update_api
    assert 'CGNAT/NAT privado' in update_api
