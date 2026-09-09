from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(rel):
    return (ROOT / rel).read_text(encoding='utf-8')


def test_scripts_page_uses_new_shell():
    html = read('app/static/pages/scripts.html')
    assert 'scripts-commandbar' in html
    assert 'scripts-tabs' in html
    assert 'scripts-view' in html
    assert 'data-script-tab="library"' in html


def test_scripts_front_has_compact_sections_and_no_legacy_module_tabs():
    js = read('app/static/js/pages/scripts.js')
    assert 'scripts-kpi-grid' in js
    assert 'scripts-category-grid' in js
    assert 'scripts-table-card' in js
    assert 'scripts-schedule-intro' in js
    assert 'data-module-tab' not in js


def test_scripts_tab_is_restored_from_url():
    js = read('app/static/js/pages/scripts.js')
    router = read('app/static/js/router.js')
    assert "searchParams.get('tab')" in js
    assert "searchParams.set('tab', tab)" in js
    assert "'scripts'" in router and "'reports'" in router and "url.searchParams.delete('tab')" in router


def test_cache_busting_updated():
    index = read('app/static/index.html')
    assert 'scripts-ui-v10-47' in index


def test_scripts_styles_include_dark_and_mobile_states():
    css = read('app/static/styles.css')
    assert '.scripts-category-grid' in css
    assert 'html[data-theme="dark"] .scripts-tabs' in css
    assert '@media (max-width:720px)' in css
