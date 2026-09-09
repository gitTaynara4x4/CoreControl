from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SETTINGS_HTML = (ROOT / 'app/static/pages/settings.html').read_text(encoding='utf-8')
SETTINGS_JS = (ROOT / 'app/static/js/pages/settings.js').read_text(encoding='utf-8')
ROUTER_JS = (ROOT / 'app/static/js/router.js').read_text(encoding='utf-8')
INDEX_HTML = (ROOT / 'app/static/index.html').read_text(encoding='utf-8')
STYLES = (ROOT / 'app/static/styles.css').read_text(encoding='utf-8')


def test_settings_uses_custom_tabs_and_view():
    assert 'class="settings-tabs"' in SETTINGS_HTML
    for tab in ('general', 'permissions', 'alerts', 'security', 'integrations', 'appearance', 'agent', 'audit'):
        assert f'data-settings-tab="{tab}"' in SETTINGS_HTML
    assert 'id="settingsView"' in SETTINGS_HTML


def test_settings_keeps_tab_on_refresh():
    assert "searchParams.get('tab')" in SETTINGS_JS
    assert "searchParams.set('tab', tab)" in SETTINGS_JS
    assert "tabFromUrl()" in SETTINGS_JS
    assert "['updates', 'scripts', 'network', 'reports', 'settings']" in ROUTER_JS


def test_settings_general_layout_is_structured():
    assert 'settings-status-card' in SETTINGS_JS
    assert 'settings-value-list' in SETTINGS_JS
    assert 'settings-prepared-state' in SETTINGS_JS
    assert 'Nome exibido' in SETTINGS_JS
    assert 'Português (Brasil)' in SETTINGS_JS


def test_settings_does_not_fake_unimplemented_backend_actions():
    assert 'ainda precisa ser conectada ao backend' in SETTINGS_JS
    assert 'Estrutura preparada' in SETTINGS_JS


def test_settings_styles_and_cache_busting_are_present():
    assert '/* ===== Configurações v10.50 ===== */' in STYLES
    assert '.settings-tabs' in STYLES
    assert '.settings-theme-grid' in STYLES
    assert '.settings-feature-grid' in STYLES
    assert '20260909-settings-ui-v10-50' in INDEX_HTML
