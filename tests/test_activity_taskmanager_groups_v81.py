from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
JS = (ROOT / 'app/static/js/pages/devices.js').read_text(encoding='utf-8')
HTML = (ROOT / 'app/static/index.html').read_text(encoding='utf-8')
CSS = (ROOT / 'app/static/styles.css').read_text(encoding='utf-8')

def test_expandable_groups_are_in_javascript():
    assert 'activityGroupedApplications' in JS
    assert 'activityRenderGroupRows' in JS
    assert 'activityBindGroupToggles' in JS
    assert 'data-activity-toggle' in JS
    assert 'data-activity-child' in JS
    assert 'activityExpandedGroups' in JS

def test_group_ui_and_cache_bust():
    assert '.activity-group-toggle' in CSS
    assert '.activity-tree-child' in CSS
    assert 'devices.js?v=20260831-task-groups-v8-1' in HTML
    assert 'styles.css?v=20260831-task-groups-v8-1' in HTML
