from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
APP_SRC = ROOT / "desktop" / "app" / "src"
APP_SOURCE = "\n".join(
    p.read_text(encoding="utf-8")
    for p in sorted(APP_SRC.glob("*.go"))
    if not p.name.endswith("_test.go")
)
SETUP_SOURCE = (ROOT / "desktop" / "setup" / "src" / "main.go").read_text(encoding="utf-8")


def test_setup_shows_only_linked_company_context():
    assert '"✓ Empresa confirmada"' in SETUP_SOURCE
    assert "Nenhuma senha administrativa é salva neste computador." in SETUP_SOURCE
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


def test_setup_repaints_between_wizard_steps_without_ghosting():
    theme_source = (ROOT / "desktop" / "setup" / "src" / "theme_windows.go").read_text(encoding="utf-8")

    # The former combination (transparent STATIC controls + no real background erase)
    # left login/register pixels visible over the next wizard step on Windows.
    assert "eraseThemeBackground(syscall.Handle(hwnd), wParam)" in SETUP_SOURCE
    assert "forceRedraw(a.hwnd)" in SETUP_SOURCE
    assert "windowStyle := uintptr(WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX)" in SETUP_SOURCE
    assert "procSetBkMode.Call(hdc, OPAQUE)" in theme_source
    assert "return uintptr(themeWhiteBrush)" in theme_source
    assert "RDW_ALLCHILDREN" in theme_source
    assert 'createControl("STATIC", "Aguardando login"' not in SETUP_SOURCE
    assert 'createControl("STATIC", "Entre com sua conta para continuar."' not in SETUP_SOURCE
