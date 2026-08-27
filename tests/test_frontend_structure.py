from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
STATIC = ROOT / "app" / "static"


EXPECTED_PAGES = {
    "login.html": ["loginView", "loginForm", "registerCompanyForm"],
    "overview.html": ["overviewStats", "overviewAttention", "overviewCompanies"],
    "companies.html": ["companiesGrid", "newCompanyBtn"],
    "company.html": ["companyStats", "companyDevicesArea", "enrollBtn", "deleteCompanyBtn"],
    "devices.html": ["devicesCount", "deviceSearch", "deviceTableArea"],
    "device.html": ["deviceMetrics", "telemetryChart", "remoteAccessBtn"],
    "alerts.html": ["alertsArea"],
    "remote.html": ["remoteStats", "remoteDevicesArea"],
    "users.html": ["usersArea", "newUserBtn"],
    "updates.html": ["updatesView", "updatesCheckBtn"],
    "scripts.html": ["scriptsView", "scriptNewBtn"],
    "network.html": ["networkView", "networkTestBtn"],
    "reports.html": ["reportsView"],
    "settings.html": ["settingsView"],
}


EXPECTED_SCRIPTS = [
    "js/core.js",
    "js/ui.js",
    "js/auth.js",
    "js/router.js",
    "js/modals.js",
    "js/pages/overview.js",
    "js/pages/companies.js",
    "js/pages/devices.js",
    "js/pages/alerts.js",
    "js/pages/remote.js",
    "js/pages/users.js",
    "js/pages/updates.js",
    "js/pages/scripts.js",
    "js/pages/network.js",
    "js/pages/reports.js",
    "js/pages/settings.js",
    "app.js",
]


def test_each_central_screen_has_an_editable_html_file():
    for filename, required_ids in EXPECTED_PAGES.items():
        page = STATIC / "pages" / filename
        assert page.is_file(), filename
        html = page.read_text(encoding="utf-8")
        for element_id in required_ids:
            assert f'id="{element_id}"' in html, (filename, element_id)


def test_index_keeps_only_the_shared_shell_and_loads_split_scripts():
    index = (STATIC / "index.html").read_text(encoding="utf-8")
    assert 'id="loginMount"' in index
    assert 'id="appView"' in index
    assert 'id="content"' in index
    assert 'id="modalBackdrop"' in index
    assert 'id="remoteViewerMount"' in index

    for relative_path in EXPECTED_SCRIPTS:
        assert f'/static/{relative_path}' in index, relative_path

    assert '<form id="loginForm"' not in index
    assert 'Computadores que exigem atenção' not in index


def test_page_javascript_is_split_and_registered_in_router():
    registrations = {
        "overview": "js/pages/overview.js",
        "companies": "js/pages/companies.js",
        "company": "js/pages/companies.js",
        "devices": "js/pages/devices.js",
        "device": "js/pages/devices.js",
        "alerts": "js/pages/alerts.js",
        "remote": "js/pages/remote.js",
        "users": "js/pages/users.js",
        "updates": "js/pages/updates.js",
        "scripts": "js/pages/scripts.js",
        "network": "js/pages/network.js",
        "reports": "js/pages/reports.js",
        "settings": "js/pages/settings.js",
    }
    for page_name, relative_path in registrations.items():
        script = (STATIC / relative_path).read_text(encoding="utf-8")
        assert f"registerPage('{page_name}'" in script


def test_remote_v7_and_windows_icon_assets_were_preserved():
    remote_script = (ROOT / "app" / "remote_assets" / "meshcentral-custom.js").read_text(
        encoding="utf-8"
    )
    assert "CoreControl Remote v7" in remote_script
    assert "obterParametro('ctnode')" in remote_script

    assert (ROOT / "desktop" / "assets" / "coretuner.ico").is_file()
    assert (ROOT / "desktop" / "setup" / "src" / "coretuner.ico").is_file()
    assert (ROOT / "desktop" / "app" / "src" / "coretuner.ico").is_file()
    assert (ROOT / "app" / "downloads" / "CoreControlSetup.exe").stat().st_size > 5_000_000


def test_login_mount_is_removed_from_layout_after_authentication():
    auth = (ROOT / "app/static/js/auth.js").read_text(encoding="utf-8")
    assert "CT.$('#loginMount').classList.add('hidden')" in auth
    assert "CT.$('#loginMount').classList.remove('hidden')" in auth


def test_frontend_cache_version_install_code_v1_is_consistent():
    index = (ROOT / "app/static/index.html").read_text(encoding="utf-8")
    core = (ROOT / "app/static/js/core.js").read_text(encoding="utf-8")
    assert "20260827-install-code-v1" in index
    assert "CT.VERSION = '20260827-install-code-v1'" in core


def test_company_destroy_modal_is_split_and_requires_confirmation():
    modal = (STATIC / "components/modals/company-delete.html").read_text(encoding="utf-8")
    companies_js = (STATIC / "js/pages/companies.js").read_text(encoding="utf-8")
    modals_js = (STATIC / "js/modals.js").read_text(encoding="utf-8")
    assert 'id="companyDeleteForm"' in modal
    assert 'id="deleteCompanyConfirmation"' in modal
    assert "CT.canDestroyCompanies()" in companies_js
    assert "method: 'DELETE'" in modals_js
