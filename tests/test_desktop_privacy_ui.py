from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
APP_SOURCE = (ROOT / "desktop" / "app" / "src" / "main.go").read_text(encoding="utf-8")
SETUP_SOURCE = (ROOT / "desktop" / "setup" / "src" / "main.go").read_text(encoding="utf-8")


def test_setup_shows_only_linked_company_context():
    assert '"Empresa vinculada"' in SETUP_SOURCE
    assert "Este computador será adicionado a essa empresa." in SETUP_SOURCE
    assert "computadores, %d online" not in SETUP_SOURCE
    assert 'serverURL+"/api/devices"' not in SETUP_SOURCE


def test_local_app_does_not_expose_company_device_list():
    assert '"Computadores da empresa"' not in APP_SOURCE
    assert '"Administração"' not in APP_SOURCE
    assert 'server+"/api/devices"' not in APP_SOURCE
    assert 'server+"/api/auth/me"' in APP_SOURCE


def test_local_app_has_click_cursor_and_hover_feedback():
    assert "WM_SETCURSOR" in APP_SOURCE
    assert "WM_MOUSEMOVE" in APP_SOURCE
    assert "IDC_HAND" in APP_SOURCE
    assert "isHovered" in APP_SOURCE


def test_setup_buttons_use_click_cursor():
    assert "WM_SETCURSOR" in SETUP_SOURCE
    assert "IDC_HAND" in SETUP_SOURCE
    assert "isClickableControl" in SETUP_SOURCE
