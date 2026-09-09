from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def test_updates_page_has_polished_navigation_and_actions():
    html = read("app/static/pages/updates.html")
    assert 'class="updates-commandbar"' in html
    assert 'class="updates-tabs"' in html
    assert 'data-module-tab="overview"' in html
    assert 'data-module-tab="computers"' in html
    assert 'data-module-tab="policies"' in html
    assert 'id="updatesCheckBtn"' in html


def test_updates_js_explains_module_hierarchy_and_sources():
    js = read("app/static/js/pages/updates.js")
    assert '“Atualizações” é o módulo geral.' in js
    assert "'Windows', 'Windows Update'" in js
    assert "'Drivers', 'Windows Update'" in js
    assert "'Aplicativos', 'Windows Package Manager'" in js
    assert 'data-updates-device-search' in js
    assert 'data-updates-status-filter' in js


def test_updates_styles_are_scoped_and_responsive():
    css = read("app/static/styles.css")
    assert '.page-updates' in css
    assert '.updates-kpi-grid' in css
    assert '.updates-source-grid' in css
    assert '.updates-table-card' in css
    assert '@media (max-width:720px)' in css
    assert 'html[data-theme="dark"] .updates-tabs' in css


def test_cache_busting_points_to_v1045():
    html = read("app/static/index.html")
    assert '/static/styles.css?v=20260909-settings-ui-v10-50' in html
    assert '/static/js/pages/updates.js?v=20260909-route-persistence-v10-46' in html
    assert '/static/js/router.js?v=20260909-settings-ui-v10-50' in html


def test_router_copy_clarifies_update_scope():
    router = read("app/static/js/router.js")
    assert "Gerencie Windows Update, drivers e aplicativos dos computadores." in router
