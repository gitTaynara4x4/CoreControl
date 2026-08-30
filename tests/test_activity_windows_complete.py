from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def test_snapshot_collects_windows_11_explorer_tabs():
    activity = read("agent/src/activity_windows.go")
    explorer = read("agent/src/explorer_tabs_windows.go")
    assert "collectExplorerFolderTabs()" in activity
    assert "CabinetWClass" in explorer
    assert "ExploreWClass" in explorer
    assert "UIAutomationClient" in explorer
    assert "ControlType]::TabItem" in explorer
    assert 'ProcessName: "explorer"' in explorer


def test_snapshot_no_longer_stops_at_24_windows():
    activity = read("agent/src/activity_windows.go")
    assert "if len(apps) > 24" not in activity
    assert "if len(apps) > 200" in activity


def test_panel_renders_every_returned_window_and_scrolls():
    script = read("app/static/js/pages/devices.js")
    styles = read("app/static/styles.css")
    assert "apps.slice(0, 14)" not in script
    assert "${apps.map((app) =>" in script
    assert "activity-table-scroll" in script
    assert ".activity-table-scroll{max-height:520px;overflow:auto" in styles
    assert "position:sticky" in styles
