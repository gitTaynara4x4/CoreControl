from pathlib import Path

def test_devices_has_automatic_activity_refresh():
    js = Path('app/static/js/pages/devices.js').read_text(encoding='utf-8')
    assert 'startActivityAutoRefresh(device)' in js
    assert 'activityRunAutomaticRefresh(device)' in js
    assert 'window.setTimeout(tick, 5000)' in js
    assert "requestActivitySnapshot(device, { silent: true })" in js
    assert 'cached_command' in js

def test_latest_features_are_preserved():
    js = Path('app/static/js/pages/devices.js').read_text(encoding='utf-8')
    assert 'activityGroupedApplications' in js
    assert 'activityTabIcon' in js
    assert 'fav_icon_url' in js
    assert 'activityExpandedGroups' in js

def test_cache_bust_changed():
    html = Path('app/static/index.html').read_text(encoding='utf-8')
    assert 'devices.js?v=20260901-activity-auto-v10-2' in html
