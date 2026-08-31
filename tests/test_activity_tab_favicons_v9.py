from pathlib import Path

def test_devices_js_has_tab_favicon_helper():
    js = Path('app/static/js/pages/devices.js').read_text(encoding='utf-8')
    assert 'function activityTabFaviconSource(tab)' in js
    assert 'function activityTabIcon(tab, browserProcess' in js
    assert 'google.com/s2/favicons' in js
    assert "activityTabIcon(tab, group.process_name, Boolean(tab?.active), 'activity-child-icon')" in js
