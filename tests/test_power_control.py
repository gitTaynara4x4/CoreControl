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
    assert 'corecontrol_box_wol' in api
    assert 'CoreControl Box offline ou não instalada nesta rede' in api
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
    assert 'readiness?.safe_to_power_off' in ui
    assert 'CoreControl Box online' in ui
    assert 'powerState.safe_to_power_off' in overview
    assert 'powerState.safe_to_power_off' in devices


def test_agent_097_audits_and_prepares_wol_without_claiming_full_shutdown_guarantee():
    main = (ROOT / "agent/src/main.go").read_text(encoding="utf-8")
    windows = (ROOT / "agent/src/wol_capability_windows.go").read_text(encoding="utf-8")
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")

    assert 'const agentVersion = "0.9.13"' in main
    assert '"wol_capability"' in main
    assert "Get-NetAdapterPowerManagement" in windows
    assert "Set-NetAdapterPowerManagement" in windows
    assert "powercfg.exe /deviceenablewake" in windows
    assert "wake_armed" in windows
    assert "intel_amt_detected" in windows
    assert "pc_wol_prepared" in api
    assert "CoreControl Box online" in api


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


def test_legacy_wan_route_probe_is_not_used_by_v1041_power_path():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    devices = (ROOT / "app/static/js/pages/devices.js").read_text(encoding="utf-8")

    # Legacy diagnostics may remain in the backend, but the actual power endpoint
    # must never use router mutation/Wake-on-WAN.
    assert '@router.post("/devices/{device_id}/wake-route-test")' in api
    endpoint = api[api.index('@router.post("/devices/{device_id}/power")'):api.index('@router.get("/alerts")')]
    assert 'corecontrol_wan_upnp' not in endpoint
    assert 'latest_wan_wake_route' not in endpoint
    assert 'meshcentral_wake' not in endpoint
    assert "wakeRouteButton.classList.add('hidden')" in devices


def test_wan_route_only_accepts_public_ipv4_and_high_udp_port():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    update_api = (ROOT / "app/update_api.py").read_text(encoding="utf-8")
    assert 'parsed_ip.is_global' in api
    assert '40000 <= external_port <= 59999' in api
    assert 'parsed.is_global' in update_api
    assert 'CGNAT/NAT privado' in update_api


def test_route_probe_does_not_depend_on_stale_setup_version_metadata():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    devices = (ROOT / "app/static/js/pages/devices.js").read_text(encoding="utf-8")
    assert 'Atualize o CoreControl Agent para 0.9.7 antes de testar a rota.' not in api
    assert "!routeAgentSupported" not in devices
    assert "wakeRouteButton.classList.add('hidden')" in devices
    assert "wakeRouteButton.onclick = null" in devices


def test_reinstall_does_not_overwrite_runtime_agent_version_with_setup_version():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    assert "O instalador possui uma versão própria" in api
    assert "Preserve a versão real do Agent" in api

def test_native_agent_heartbeat_has_priority_over_mesh_power_state():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    ui = (ROOT / "app/static/js/ui.js").read_text(encoding="utf-8")

    fn = api[api.index("def device_power_currently_on"):api.index("def device_power_pending_state")]
    assert "agent_power_fresh" in fn
    assert "<= 30" in fn
    assert fn.index("if agent_power_fresh:") < fn.index("if mesh_ready and mesh_recent:")
    assert "if (device?.actual_online || device?.online) return true;" in ui


def test_real_shutdown_is_dispatched_by_native_agent():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    agent = (ROOT / "agent/src/update_windows.go").read_text(encoding="utf-8")
    assert '"power.shutdown"' in api
    assert 'methods.extend(["corecontrol_agent_shutdown", "corecontrol_box_ready"])' in api
    assert 'case "power.shutdown":' in agent
    assert "CORECONTROL_REAL_SHUTDOWN_CONFIRMED" in agent
    assert "shutdown.exe" in agent


def test_new_clients_use_local_gateway_instead_of_router_mutation():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    ui = (ROOT / "app/static/js/ui.js").read_text(encoding="utf-8")
    assert '"route_preflight_available": False' in api
    assert 'box-only architecture' in api
    assert 'CoreControl Box offline ou não instalada nesta rede' in api
    assert "CoreControl Box necessária" in ui
    assert "Preparar e desligar" not in ui
    assert "canPrepareRoute" not in ui


def test_setup_reports_the_bundled_agent_version_instead_of_setup_version():
    setup = (ROOT / "desktop/setup/src/main.go").read_text(encoding="utf-8")
    assert 'const appVersion = "0.4.21"' in setup
    assert 'const bundledAgentVersion = "0.9.13"' in setup
    assert '"agent_version": bundledAgentVersion' in setup
    assert '"agent_version": appVersion' not in setup


def test_release_installer_defaults_to_production_server():
    setup = (ROOT / "desktop/setup/src/main.go").read_text(encoding="utf-8")
    desktop = (ROOT / "desktop/app/src/main.go").read_text(encoding="utf-8")
    build = (ROOT / "desktop/Build_Windows.ps1").read_text(encoding="utf-8")
    production = "https://apps-corecontrol.9ywrah.easypanel.host"
    assert f'var defaultServerURL = "{production}"' in setup
    assert f'var defaultServerURL = "{production}"' in desktop
    assert f"else {{ '{production}' }}" in build
    assert 'var defaultServerURL = "http://127.0.0.1:8002"' not in setup


def test_v1041_power_path_is_box_only():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    agent = (ROOT / "agent/src/main.go").read_text(encoding="utf-8")
    assert 'Device.device_kind == "gateway"' in api
    endpoint = api[api.index('@router.post("/devices/{device_id}/power")'):api.index('@router.get("/alerts")')]
    assert 'find_power_gateways' in endpoint
    assert 'corecontrol_box_wol' in endpoint
    assert 'corecontrol_wan_upnp' not in endpoint
    assert 'meshcentral_lan_relay' not in endpoint
    assert 'meshcentral_wake' not in endpoint
    assert 'find_mesh_wake_relays' not in endpoint
    assert '"wol_relay_capable":      false' in agent

def test_real_shutdown_command_is_implemented_not_only_referenced():
    windows = (ROOT / "agent/src/update_windows.go").read_text(encoding="utf-8")
    assert 'case "power.shutdown":' in windows
    assert 'func executeRealShutdownCommand()' in windows
    assert 'shutdown.exe' in windows
    assert '"shutdown_scheduled": true' in windows
    assert 'CORECONTROL_REAL_SHUTDOWN_CONFIRMED' in windows
