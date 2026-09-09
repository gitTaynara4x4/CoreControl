from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
HTML = (ROOT / 'app/static/pages/network.html').read_text(encoding='utf-8')
JS = (ROOT / 'app/static/js/pages/network.js').read_text(encoding='utf-8')
ROUTER = (ROOT / 'app/static/js/router.js').read_text(encoding='utf-8')
INDEX = (ROOT / 'app/static/index.html').read_text(encoding='utf-8')
CSS = (ROOT / 'app/static/styles.css').read_text(encoding='utf-8')


def test_network_uses_finished_tabs_and_toolbar():
    assert 'network-tabs' in HTML
    assert 'data-network-tab="overview"' in HTML
    assert 'data-network-tab="tests"' in HTML
    assert 'data-network-tab="devices"' in HTML
    assert 'Novo teste de rede' in HTML
    assert 'module-tabs' not in HTML


def test_network_front_has_compact_overview_and_filters():
    assert 'network-status-card' in JS
    assert 'network-kpi-grid' in JS
    assert 'networkDeviceSearch' in JS
    assert 'networkStatusFilter' in JS
    assert 'Visão geral da rede' in JS


def test_network_tabs_survive_refresh():
    assert "NETWORK_TABS = new Set(['overview', 'tests', 'devices'])" in JS
    assert "url.searchParams.set('tab', tab)" in JS
    assert "['updates', 'scripts', 'network', 'reports', 'settings']" in ROUTER


def test_network_has_test_and_device_views():
    assert 'DIAGNÓSTICO DE REDE' in JS
    assert 'network-test-types' in JS
    assert 'INVENTÁRIO DE REDE' in JS
    assert 'Descoberta de rede' in JS


def test_network_css_and_cache_busting_present():
    assert 'CoreControl v10.48 · Rede' in CSS
    assert '.network-commandbar' in CSS
    assert '.network-test-layout' in CSS
    assert '.network-table-card' in CSS
    assert 'network-ui-v10-48' in INDEX
