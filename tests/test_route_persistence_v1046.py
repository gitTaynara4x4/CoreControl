from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path):
    return (ROOT / path).read_text(encoding='utf-8')


def test_boot_restores_current_route_instead_of_forcing_overview():
    app = read('app/static/app.js')
    assert "CT.restoreRoute('replace')" in app
    boot = app.split('CT.boot = async function boot()', 1)[1]
    assert "await CT.navigate('overview');" not in boot


def test_router_persists_page_and_detail_context_in_url():
    router = read('app/static/js/router.js')
    assert "url.searchParams.set(ROUTE_PAGE_PARAM, page)" in router
    assert "company: 'company'" in router
    assert "device: 'device'" in router
    assert "window.addEventListener('popstate'" in router


def test_updates_subtab_survives_reload():
    updates = read('app/static/js/pages/updates.js')
    assert "searchParams.get('tab')" in updates
    assert "url.searchParams.set('tab', tab)" in updates
    assert "activeTab = readActiveTab();" in updates


def test_auth_login_keeps_requested_route():
    auth = read('app/static/js/auth.js')
    assert "await CT.restoreRoute('replace');" in auth


def test_cache_busting_for_route_files():
    index = read('app/static/index.html')
    assert index.count('20260909-route-persistence-v10-46') >= 4
