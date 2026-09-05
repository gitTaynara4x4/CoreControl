from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def test_remote_session_reconciles_mesh_rights_before_opening():
    api = (ROOT / "app" / "api.py").read_text(encoding="utf-8")
    section = api.split('@router.post("/devices/{device_id}/remote-session")', 1)[1]
    section = section.split('@router.get("/devices/{device_id}/power-readiness")', 1)[0]
    assert "meshcentral_client.ensure_company_group(company)" in section


def test_integration_user_is_always_synced_with_full_remote_control():
    source = (ROOT / "app" / "meshcentral.py").read_text(encoding="utf-8")
    section = source.split("integration_user_id = self.ensure_integration_user()", 1)[1]
    section = section.split("return mesh_id, mesh_hex.lower(), group_name", 1)[0]
    assert '"AddUserToDeviceGroup"' in section
    assert '"--remotecontrol"' in section
    assert '"--desktopviewonly"' not in section
    assert '"--limiteddesktop"' not in section
    assert "already_linked" not in section


def test_coretuner_remote_session_forces_meshcentral_input_on():
    script = (ROOT / "app" / "remote_assets" / "meshcentral-custom.js").read_text(encoding="utf-8")
    assert "CoreControl Remote v10.6-control" in script
    assert "function habilitarControle()" in script
    assert "input.checked = true" in script
    assert "window.putstore('DeskControl', 1)" in script
    assert "controle de mouse e teclado ativo" in script
