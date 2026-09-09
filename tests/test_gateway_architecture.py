from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def test_gateway_is_first_class_enrollment_kind():
    models = (ROOT / "app/models.py").read_text(encoding="utf-8")
    schemas = (ROOT / "app/schemas.py").read_text(encoding="utf-8")
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    db = (ROOT / "app/db.py").read_text(encoding="utf-8")

    assert 'purpose: Mapped[str]' in models
    assert 'device_kind: Mapped[str]' in models
    assert 'pattern=r"^(computer|gateway)$"' in schemas
    assert '@router.post("/companies/{company_id}/gateway-enrollment-token")' in api
    assert '@router.get("/gateway/{credential}/download")' in api
    assert '@router.get("/gateways")' in api
    assert 'purpose="gateway"' in api
    assert 'Device.device_kind == "computer"' in api
    assert 'Device.device_kind == "gateway"' in api
    assert '"device_kind": "VARCHAR(24) NOT NULL DEFAULT \'computer\'"' in db


def test_gateway_uses_outbound_agent_queue_and_local_magic_packet():
    gateway = (ROOT / "gateway/src/main.go").read_text(encoding="utf-8")
    windows = (ROOT / "gateway/src/install_windows.go").read_text(encoding="utf-8")
    build = (ROOT / "desktop/Build_Windows.ps1").read_text(encoding="utf-8")

    assert 'const gatewayVersion = "1.1.0"' in gateway
    assert 'DeviceKind:      "gateway"' in gateway
    assert '"gateway_mode":      true' in gateway
    assert '"wol_relay_capable": true' in gateway
    assert '"network_cidrs":' in gateway
    assert '"/api/agent/commands/next?device_uid="' in gateway
    assert 'command.Type != "power.wake_peer"' in gateway
    assert 'sendWakeOnLAN' in gateway
    assert 'schtasks.exe' in windows
    assert '"ONSTART"' in windows
    assert '"SYSTEM"' in windows
    assert 'CoreControlGateway.exe' in build


def test_power_engine_prefers_gateway_and_does_not_depend_on_router():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    ui = (ROOT / "app/static/js/ui.js").read_text(encoding="utf-8")
    company = (ROOT / "app/static/pages/company.html").read_text(encoding="utf-8")
    modals = (ROOT / "app/static/js/modals.js").read_text(encoding="utf-8")

    assert 'def find_power_gateways' in api
    assert 'pc_wol_prepared' in api
    assert '"gateway_available": box_available' in api
    assert '"power_engine_version": "10.41"' in api
    assert '"route_preflight_available": False' in api
    assert 'corecontrol_box_wol' in api
    assert 'CoreControl Box necessária' in ui
    assert 'id="gatewayBtn"' in company
    assert 'Adicionar CoreControl Box' in modals


def test_agents_report_multiple_lan_networks_for_relay_selection():
    agent = (ROOT / "agent/src/main.go").read_text(encoding="utf-8")
    wol = (ROOT / "agent/src/wol.go").read_text(encoding="utf-8")
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")

    assert '"network_cidrs":          localNetworkCIDRs()' in agent
    assert 'func localNetworkCIDRs() []string' in wol
    assert 'def _network_objects(info: dict)' in api
    assert 'if target_network == peer_network:' in api
    assert 'Device.device_kind == "gateway"' in api
    assert '"wol_relay_capable":      false' in agent


def test_gateway_does_not_pollute_computer_dashboards_or_update_lists():
    api = (ROOT / "app/api.py").read_text(encoding="utf-8")
    updates = (ROOT / "app/update_api.py").read_text(encoding="utf-8")

    # Gateways are infrastructure relays, not user PCs. Normal company/device,
    # dashboard and Windows Update views must keep counting computers only.
    assert 'select(Device).where(Device.active.is_(True), Device.device_kind == "computer", *company_filter)' in api
    assert 'select(Device).where(Device.company_id == company.id, Device.device_kind == "computer")' in api
    assert 'select(Device).where(Device.active.is_(True), Device.device_kind == "computer").order_by(Device.name)' in updates


def test_gateway_frontend_assets_are_cache_busted_for_release():
    index = (ROOT / "app/static/index.html").read_text(encoding="utf-8")
    for asset in ["ui.js", "modals.js", "pages/overview.js", "pages/companies.js", "pages/devices.js"]:
        assert f"{asset}?v=20260909-box-only-v10-41" in index
