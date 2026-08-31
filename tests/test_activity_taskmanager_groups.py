from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEVICES_JS = ROOT / "app" / "static" / "js" / "pages" / "devices.js"
STYLES = ROOT / "app" / "static" / "styles.css"
INDEX = ROOT / "app" / "static" / "index.html"


def test_activity_uses_expandable_task_manager_groups():
    source = DEVICES_JS.read_text(encoding="utf-8")
    assert "activityGroupedApplications" in source
    assert "activityRenderGroupRows" in source
    assert "data-activity-toggle" in source
    assert "data-activity-child" in source
    assert "activityExpandedGroups" in source
    assert "Abas abertas do navegador" not in source
    assert "Clique na seta para ver todas as abas e janelas agrupadas." in source


def test_group_styles_and_cache_busting_are_present():
    css = STYLES.read_text(encoding="utf-8")
    html = INDEX.read_text(encoding="utf-8")
    assert ".activity-group-toggle" in css
    assert ".activity-tree-child" in css
    assert ".activity-tree-branch" in css
    assert "styles.css?v=20260830-task-groups-v8" in html
    assert "devices.js?v=20260830-task-groups-v8" in html
