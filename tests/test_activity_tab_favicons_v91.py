from pathlib import Path

def test_tab_favicons_have_uia_title_fallbacks_and_bridge_field():
    js = Path('app/static/js/pages/devices.js').read_text(encoding='utf-8')
    assert 'tab?.fav_icon_url' in js
    assert 'mail.google.com' in js
    assert 'maps.google.com' in js
    assert 'youtube.com' in js
    assert 'instagram.com' in js
    assert 'segware.com.br' in js
    assert 'activityTabKnownIcon' in js
    assert 'activity-tab-browser-fallback' in js

def test_devices_js_cache_version_bumped():
    html = Path('app/static/index.html').read_text(encoding='utf-8')
    assert 'devices.js?v=20260831-tab-favicons-v9-1' in html
